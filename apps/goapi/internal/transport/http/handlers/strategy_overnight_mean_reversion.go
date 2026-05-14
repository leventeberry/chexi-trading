package handlers

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"goapi/internal/strategy/overnightmeanreversion"
)

// OvernightMeanReversionScoreRequest is the JSON body for POST .../overnight-mean-reversion/score.
type OvernightMeanReversionScoreRequest struct {
	Symbol                  string  `json:"symbol" binding:"required,min=1,max=64"`
	CurrentPrice            float64 `json:"current_price" binding:"required"`
	DailyVolume             float64 `json:"daily_volume"`
	PercentChange24h        float64 `json:"percent_change_24h"`
	IntradayDrawdownPercent float64 `json:"intraday_drawdown_percent"`
	RSI                     float64 `json:"rsi"`
	SupportDistancePercent  float64 `json:"support_distance_percent"`
	VolumeTrend             string  `json:"volume_trend" binding:"required,oneof=rising flat falling"`
	CandleSignal            string  `json:"candle_signal" binding:"required,oneof=bullish_reversal neutral bearish_continuation"`
	HigherLowDetected       bool    `json:"higher_low_detected"`
	PlannedEntry            float64 `json:"planned_entry" binding:"required"`
	StopLoss                float64 `json:"stop_loss" binding:"required"`
	TargetPrice             float64 `json:"target_price" binding:"required"`
}

func (r *OvernightMeanReversionScoreRequest) toInput() overnightmeanreversion.Input {
	return overnightmeanreversion.Input{
		Symbol:                  strings.TrimSpace(r.Symbol),
		CurrentPrice:            r.CurrentPrice,
		DailyVolume:             r.DailyVolume,
		PercentChange24h:        r.PercentChange24h,
		IntradayDrawdownPercent: r.IntradayDrawdownPercent,
		RSI:                     r.RSI,
		SupportDistancePercent:  r.SupportDistancePercent,
		VolumeTrend:             overnightmeanreversion.VolumeTrend(r.VolumeTrend),
		CandleSignal:            overnightmeanreversion.CandleSignal(r.CandleSignal),
		HigherLowDetected:       r.HigherLowDetected,
		PlannedEntry:            r.PlannedEntry,
		StopLoss:                r.StopLoss,
		TargetPrice:             r.TargetPrice,
	}
}

func validateOvernightMeanReversionScoreRequest(r *OvernightMeanReversionScoreRequest) error {
	if err := validateFinite("current_price", r.CurrentPrice); err != nil {
		return err
	}
	if err := validateFinite("daily_volume", r.DailyVolume); err != nil {
		return err
	}
	if err := validateFinite("percent_change_24h", r.PercentChange24h); err != nil {
		return err
	}
	if err := validateFinite("intraday_drawdown_percent", r.IntradayDrawdownPercent); err != nil {
		return err
	}
	if err := validateFinite("rsi", r.RSI); err != nil {
		return err
	}
	if err := validateFinite("support_distance_percent", r.SupportDistancePercent); err != nil {
		return err
	}
	if err := validateFinite("planned_entry", r.PlannedEntry); err != nil {
		return err
	}
	if err := validateFinite("stop_loss", r.StopLoss); err != nil {
		return err
	}
	if err := validateFinite("target_price", r.TargetPrice); err != nil {
		return err
	}
	if r.CurrentPrice <= 0 {
		return errors.New("current_price must be greater than zero")
	}
	if r.DailyVolume < 0 {
		return errors.New("daily_volume must be non-negative")
	}
	if r.IntradayDrawdownPercent < 0 {
		return errors.New("intraday_drawdown_percent must be non-negative")
	}
	if r.SupportDistancePercent < 0 {
		return errors.New("support_distance_percent must be non-negative")
	}
	if r.RSI < 0 || r.RSI > 100 {
		return errors.New("rsi must be between 0 and 100")
	}
	if r.PlannedEntry <= 0 || r.StopLoss <= 0 || r.TargetPrice <= 0 {
		return errors.New("planned_entry, stop_loss, and target_price must be greater than zero")
	}
	return nil
}

func validateFinite(field string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("%s must be a finite number", field)
	}
	return nil
}

// ScoreOvernightMeanReversion runs deterministic OMR scoring for manual validation and dashboards.
// It does not execute trades or call external exchanges.
func ScoreOvernightMeanReversion() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req OvernightMeanReversionScoreRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		if err := validateOvernightMeanReversionScoreRequest(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		out := overnightmeanreversion.Score(req.toInput())
		c.JSON(http.StatusOK, out)
	}
}
