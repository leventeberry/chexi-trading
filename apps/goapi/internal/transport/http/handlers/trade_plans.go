package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"goapi/services"
)

// CreateTradePlanRequest is JSON for POST /api/v1/trade-plans.
type CreateTradePlanRequest struct {
	Symbol        string   `json:"symbol" binding:"required"`
	StrategyName  string   `json:"strategy_name" binding:"required,min=1,max=128"`
	Direction     string   `json:"direction" binding:"required"`
	Thesis        string   `json:"thesis" binding:"required,min=1,max=8000"`
	PlannedEntry  float64  `json:"planned_entry" binding:"required"`
	StopLoss      float64  `json:"stop_loss" binding:"required"`
	TargetPrice   float64  `json:"target_price" binding:"required"`
	PositionSize  float64  `json:"position_size" binding:"required,gt=0"`
	MaxRiskAmount float64  `json:"max_risk_amount" binding:"required,gt=0"`
	SourceScore   *float64 `json:"source_score"`
	SourceLabel   string   `json:"source_label" binding:"max=32"`
	Notes         string   `json:"notes" binding:"max=8000"`
}

// CreateTradePlan handles POST /api/v1/trade-plans.
func CreateTradePlan(svc services.TradePlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, ok := actorFromGin(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		var req CreateTradePlanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		dto, err := svc.CreateTradePlan(c.Request.Context(), actorID, &services.CreateTradePlanInput{
			Symbol:        req.Symbol,
			StrategyName:  req.StrategyName,
			Direction:     req.Direction,
			Thesis:        req.Thesis,
			PlannedEntry:  req.PlannedEntry,
			StopLoss:      req.StopLoss,
			TargetPrice:   req.TargetPrice,
			PositionSize:  req.PositionSize,
			MaxRiskAmount: req.MaxRiskAmount,
			SourceScore:   req.SourceScore,
			SourceLabel:   req.SourceLabel,
			Notes:         req.Notes,
		})
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, dto)
	}
}

// ListTradePlans handles GET /api/v1/trade-plans.
func ListTradePlans(svc services.TradePlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, ok := actorFromGin(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		list, err := svc.ListTradePlans(c.Request.Context(), actorID)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// GetTradePlan handles GET /api/v1/trade-plans/:id.
func GetTradePlan(svc services.TradePlanService) gin.HandlerFunc {
	return func(c *gin.Context) {
		actorID, _, ok := actorFromGin(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trade plan id"})
			return
		}
		dto, err := svc.GetTradePlan(c.Request.Context(), actorID, id)
		if err != nil {
			handleServiceError(c, err)
			return
		}
		c.JSON(http.StatusOK, dto)
	}
}
