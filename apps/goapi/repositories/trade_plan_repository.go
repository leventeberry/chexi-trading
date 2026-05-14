package repositories

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"goapi/models"
	"gorm.io/gorm"
)

type tradePlanRepository struct {
	db *gorm.DB
}

// NewTradePlanRepository constructs TradePlanRepository.
func NewTradePlanRepository(db *gorm.DB) TradePlanRepository {
	return &tradePlanRepository{db: db}
}

func (r *tradePlanRepository) Create(row *models.TradePlan) error {
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	if err := r.db.Create(row).Error; err != nil {
		return fmt.Errorf("create trade plan: %w", err)
	}
	return nil
}

func (r *tradePlanRepository) ListByUser(userID uuid.UUID) ([]models.TradePlan, error) {
	var rows []models.TradePlan
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list trade plans: %w", err)
	}
	return rows, nil
}

func (r *tradePlanRepository) FindByUserAndID(userID, id uuid.UUID) (*models.TradePlan, error) {
	var row models.TradePlan
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTradePlanNotFound
		}
		return nil, fmt.Errorf("get trade plan: %w", err)
	}
	return &row, nil
}
