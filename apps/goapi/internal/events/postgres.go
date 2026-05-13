package events

import (
	"context"

	"goapi/models"
	"gorm.io/gorm"
)

// PostgresRecorder persists events to event_log via GORM.
type PostgresRecorder struct {
	db *gorm.DB
}

// NewPostgresRecorder builds a Postgres-backed Recorder.
func NewPostgresRecorder(db *gorm.DB) *PostgresRecorder {
	return &PostgresRecorder{db: db}
}

// Record implements Recorder.
func (p *PostgresRecorder) Record(ctx context.Context, e Event) error {
	if p == nil || p.db == nil {
		return nil
	}
	t := e.OccurredAt
	if t.IsZero() {
		t = NowUTC()
	}
	row := models.EventLog{
		OccurredAt:  t,
		EventType:   e.EventType,
		ActorUserID: e.ActorUserID,
		Subject:     e.Subject,
		Metadata:    NormalizeMetadata(e.Metadata),
		RequestID:   e.RequestID,
	}
	return p.db.WithContext(ctx).Create(&row).Error
}
