package models

import (
	"time"

	"github.com/google/uuid"
)

// Trade plan lifecycle (manual-only; no execution hooks in V1).
const (
	TradePlanStatusPlanned  = "PLANNED"
	TradePlanStatusActive   = "ACTIVE"
	TradePlanStatusClosed   = "CLOSED"
	TradePlanStatusCanceled = "CANCELED"
)

// TradePlan is an advisory manual trade plan persisted for the owning user.
type TradePlan struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Symbol          string    `gorm:"size:32;not null;index" json:"symbol"`
	StrategyName    string    `gorm:"size:128;not null" json:"strategy_name"`
	Direction       string    `gorm:"size:8;not null" json:"direction"`
	Thesis          string    `gorm:"type:text;not null" json:"thesis"`
	PlannedEntry    float64   `json:"planned_entry"`
	StopLoss        float64   `json:"stop_loss"`
	TargetPrice     float64   `json:"target_price"`
	PositionSize    float64   `json:"position_size"`
	MaxRiskAmount   float64   `json:"max_risk_amount"`
	RiskRewardRatio float64   `json:"risk_reward_ratio"`
	SourceScore     *float64  `json:"source_score,omitempty"`
	SourceLabel     string    `gorm:"size:32" json:"source_label,omitempty"`
	Notes           string    `gorm:"type:text" json:"notes,omitempty"`
	Status          string    `gorm:"size:16;not null;index" json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (TradePlan) TableName() string {
	return "trade_plans"
}
