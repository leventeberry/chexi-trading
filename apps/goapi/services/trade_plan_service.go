package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"goapi/internal/tradeplan/risk"
	"goapi/models"
	"goapi/repositories"
)

var tradePlanSymbolPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,24}-[A-Za-z0-9]{1,24}$`)

// CreateTradePlanInput is the service payload for creating a trade plan (status is always PLANNED).
type CreateTradePlanInput struct {
	Symbol        string
	StrategyName  string
	Direction     string
	Thesis        string
	PlannedEntry  float64
	StopLoss      float64
	TargetPrice   float64
	PositionSize  float64
	MaxRiskAmount float64
	SourceScore   *float64
	SourceLabel   string
	Notes         string
}

// TradePlanDTO is returned by trade plan APIs (includes server-computed risk fields).
type TradePlanDTO struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	Symbol          string    `json:"symbol"`
	StrategyName    string    `json:"strategy_name"`
	Direction       string    `json:"direction"`
	Thesis          string    `json:"thesis"`
	PlannedEntry    float64   `json:"planned_entry"`
	StopLoss        float64   `json:"stop_loss"`
	TargetPrice     float64   `json:"target_price"`
	PositionSize    float64   `json:"position_size"`
	MaxRiskAmount   float64   `json:"max_risk_amount"`
	RiskRewardRatio float64   `json:"risk_reward_ratio"`
	RiskPerUnit     float64   `json:"risk_per_unit"`
	RewardPerUnit   float64   `json:"reward_per_unit"`
	MaxLoss         float64   `json:"max_loss"`
	SourceScore     *float64  `json:"source_score,omitempty"`
	SourceLabel     string    `json:"source_label,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	Status          string    `json:"status"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

type tradePlanService struct {
	repo repositories.TradePlanRepository
}

// NewTradePlanService constructs TradePlanService.
func NewTradePlanService(repo repositories.TradePlanRepository) TradePlanService {
	return &tradePlanService{repo: repo}
}

func validateTradePlanSymbol(symbol string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" || !tradePlanSymbolPattern.MatchString(s) {
		return "", ErrInvalidTradePlanSymbol
	}
	return s, nil
}

func (s *tradePlanService) CreateTradePlan(_ context.Context, userID uuid.UUID, in *CreateTradePlanInput) (*TradePlanDTO, error) {
	if in == nil {
		return nil, ErrInvalidTradePlanInput
	}
	if in.PositionSize <= 0 || in.MaxRiskAmount <= 0 {
		return nil, ErrInvalidTradePlanSizing
	}
	sym, err := validateTradePlanSymbol(in.Symbol)
	if err != nil {
		return nil, err
	}
	dir, err := risk.ParseDirection(in.Direction)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTradePlanDirection, err)
	}
	riskPer, rewardPer, rr, err := risk.RiskRewardPerUnit(dir, in.PlannedEntry, in.StopLoss, in.TargetPrice)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTradePlanGeometry, err)
	}
	thesis := strings.TrimSpace(in.Thesis)
	if thesis == "" {
		return nil, ErrInvalidTradePlanInput
	}
	strategy := strings.TrimSpace(in.StrategyName)
	if strategy == "" {
		return nil, ErrInvalidTradePlanInput
	}

	row := &models.TradePlan{
		UserID:          userID,
		Symbol:          sym,
		StrategyName:    strategy,
		Direction:       string(dir),
		Thesis:          thesis,
		PlannedEntry:    in.PlannedEntry,
		StopLoss:        in.StopLoss,
		TargetPrice:     in.TargetPrice,
		PositionSize:    in.PositionSize,
		MaxRiskAmount:   in.MaxRiskAmount,
		RiskRewardRatio: rr,
		SourceScore:     in.SourceScore,
		SourceLabel:     strings.TrimSpace(in.SourceLabel),
		Notes:           strings.TrimSpace(in.Notes),
		Status:          models.TradePlanStatusPlanned,
	}
	if err := s.repo.Create(row); err != nil {
		return nil, err
	}
	return tradePlanToDTO(row, riskPer, rewardPer), nil
}

func (s *tradePlanService) ListTradePlans(_ context.Context, userID uuid.UUID) ([]TradePlanDTO, error) {
	rows, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]TradePlanDTO, 0, len(rows))
	for i := range rows {
		dto, derr := dtoFromStoredRow(&rows[i])
		if derr != nil {
			continue
		}
		out = append(out, *dto)
	}
	return out, nil
}

func (s *tradePlanService) GetTradePlan(_ context.Context, userID, id uuid.UUID) (*TradePlanDTO, error) {
	row, err := s.repo.FindByUserAndID(userID, id)
	if err != nil {
		if errors.Is(err, repositories.ErrTradePlanNotFound) {
			return nil, ErrTradePlanNotFound
		}
		return nil, err
	}
	return dtoFromStoredRow(row)
}

func dtoFromStoredRow(row *models.TradePlan) (*TradePlanDTO, error) {
	dir, err := risk.ParseDirection(row.Direction)
	if err != nil {
		return nil, err
	}
	riskPer, rewardPer, _, err := risk.RiskRewardPerUnit(dir, row.PlannedEntry, row.StopLoss, row.TargetPrice)
	if err != nil {
		return nil, err
	}
	return tradePlanToDTO(row, riskPer, rewardPer), nil
}

func tradePlanToDTO(row *models.TradePlan, riskPerUnit, rewardPerUnit float64) *TradePlanDTO {
	maxLoss := risk.MaxLoss(riskPerUnit, row.PositionSize)
	return &TradePlanDTO{
		ID:              row.ID,
		UserID:          row.UserID,
		Symbol:          row.Symbol,
		StrategyName:    row.StrategyName,
		Direction:       row.Direction,
		Thesis:          row.Thesis,
		PlannedEntry:    row.PlannedEntry,
		StopLoss:        row.StopLoss,
		TargetPrice:     row.TargetPrice,
		PositionSize:    row.PositionSize,
		MaxRiskAmount:   row.MaxRiskAmount,
		RiskRewardRatio: row.RiskRewardRatio,
		RiskPerUnit:     riskPerUnit,
		RewardPerUnit:   rewardPerUnit,
		MaxLoss:         maxLoss,
		SourceScore:     row.SourceScore,
		SourceLabel:     row.SourceLabel,
		Notes:           row.Notes,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
