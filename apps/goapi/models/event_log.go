package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventLog maps to append-only event_log (audit / domain / telemetry).
type EventLog struct {
	ID          int64           `gorm:"primaryKey;autoIncrement"`
	OccurredAt  time.Time       `gorm:"not null;index;column:occurred_at"`
	EventType   string          `gorm:"not null;index;size:255"`
	ActorUserID *uuid.UUID      `gorm:"type:uuid;index"`
	Subject     string          `gorm:"size:512"`
	Metadata    json.RawMessage `gorm:"type:jsonb;not null"`
	RequestID   string          `gorm:"size:64;index;column:request_id"`
}

// TableName overrides default pluralization.
func (EventLog) TableName() string {
	return "event_log"
}
