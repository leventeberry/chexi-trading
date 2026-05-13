package models

import (
	"time"

	"github.com/google/uuid"
)

// Delivery statuses for organization_webhook_deliveries.
const (
	OrganizationWebhookDeliveryStatusPending   = "pending"
	OrganizationWebhookDeliveryStatusDelivered = "delivered"
	OrganizationWebhookDeliveryStatusFailed    = "failed"
)

// OrganizationWebhookDelivery tracks one outbound webhook attempt chain.
type OrganizationWebhookDelivery struct {
	ID                    uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WebhookID             uuid.UUID  `gorm:"type:uuid;not null;index" json:"webhook_id"`
	EventType             string     `gorm:"size:128;not null" json:"event_type"`
	Payload               []byte     `gorm:"type:jsonb;not null" json:"payload"`
	Status                string     `gorm:"size:32;not null" json:"status"`
	Attempts              int        `gorm:"not null;default:0" json:"attempts"`
	ResponseStatus        *int       `json:"response_status,omitempty"`
	ResponseBodyTruncated *string    `gorm:"type:text" json:"response_body_truncated,omitempty"`
	LastError             *string    `gorm:"type:text" json:"last_error,omitempty"`
	NextAttemptAt         *time.Time `json:"next_attempt_at,omitempty"`
	DeliveredAt           *time.Time `json:"delivered_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

func (OrganizationWebhookDelivery) TableName() string {
	return "organization_webhook_deliveries"
}
