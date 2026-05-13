package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"goapi/config"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	"goapi/models"
	"goapi/repositories"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWebhookDeliver_TerminalFailureAfterMaxAttempts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationWebhook{}, &models.OrganizationWebhookDelivery{}); err != nil {
		t.Fatal(err)
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 7)
	}
	cfg := &config.Config{}
	cfg.Environment = config.EnvironmentTest
	cfg.Queue.MaxAttempts = 3
	cfg.Webhooks.EncryptionKey = key

	plainSecret := "whsec_test_plaintext_secret_val_!"
	cipher, err := authinfra.EncryptAESGCM([]byte(plainSecret), key)
	if err != nil {
		t.Fatal(err)
	}

	uid := uuid.New()
	now := time.Now().UTC()
	user := models.User{
		ID:        uid,
		Email:     "wh-job-" + uid.String()[:8] + "@example.com",
		PassHash:  "x",
		Role:      "user",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	orgID := uuid.New()
	org := models.Organization{
		ID:              orgID,
		Name:            "O",
		Slug:            "o-" + orgID.String()[:8],
		CreatedByUserID: uid,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&org).Error; err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	hook := models.OrganizationWebhook{
		ID:               uuid.New(),
		OrganizationID:   orgID,
		URL:              srv.URL + "/h",
		SecretCiphertext: cipher,
		SecretKeyVersion: 1,
		Events:           pq.StringArray{"organization.updated"},
		Enabled:          true,
		CreatedByUserID:  uid,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(&hook).Error; err != nil {
		t.Fatal(err)
	}

	envelope := map[string]interface{}{
		"event":           "organization.updated",
		"organization_id": orgID.String(),
		"payload":         map[string]string{"x": "y"},
	}
	raw, _ := json.Marshal(envelope)
	delivery := models.OrganizationWebhookDelivery{
		ID:        uuid.New(),
		WebhookID: hook.ID,
		EventType: "organization.updated",
		Payload:   raw,
		Status:    models.OrganizationWebhookDeliveryStatusPending,
		CreatedAt: now,
	}
	if err := db.Create(&delivery).Error; err != nil {
		t.Fatal(err)
	}

	repo := repositories.NewOrganizationWebhookRepository(db)
	reg := queue.NewRegistry()
	RegisterWebhookHandlers(reg, repo, cfg)

	jobPayload, _ := json.Marshal(webhookDeliverPayload{DeliveryID: delivery.ID.String()})

	for i := 0; i < 2; i++ {
		if err := reg.Dispatch(context.Background(), WebhookDeliver, jobPayload); err == nil {
			t.Fatalf("attempt %d: expected error", i+1)
		}
	}
	if err := reg.Dispatch(context.Background(), WebhookDeliver, jobPayload); err != nil {
		t.Fatalf("final attempt: %v", err)
	}

	dd, err := repo.FindDeliveryByID(delivery.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dd.Status != models.OrganizationWebhookDeliveryStatusFailed {
		t.Fatalf("status=%q attempts=%d", dd.Status, dd.Attempts)
	}
	if dd.Attempts != 3 {
		t.Fatalf("attempts=%d", dd.Attempts)
	}
}
