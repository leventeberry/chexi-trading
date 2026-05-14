package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"goapi/container"
	"goapi/internal/marketdata/metrics"
	"goapi/internal/marketdata/omrinput"
	"goapi/internal/strategy/overnightmeanreversion"
)

// MarketTickerOMRScoreRequest is the JSON body for POST .../market/tickers/:productID/overnight-mean-reversion/score.
type MarketTickerOMRScoreRequest struct {
	RSI                    float64 `json:"rsi"`
	SupportDistancePercent float64 `json:"support_distance_percent"`
	VolumeTrend            string  `json:"volume_trend" binding:"required,oneof=rising flat falling"`
	CandleSignal           string  `json:"candle_signal" binding:"required,oneof=bullish_reversal neutral bearish_continuation"`
	HigherLowDetected      bool    `json:"higher_low_detected"`
	PlannedEntry           float64 `json:"planned_entry" binding:"required"`
	StopLoss               float64 `json:"stop_loss" binding:"required"`
	TargetPrice            float64 `json:"target_price" binding:"required"`
}

// ScoreMarketTickerOMR scores the latest in-memory ticker for :productID using manual OMR fields (advisory only).
func ScoreMarketTickerOMR(c *container.Container) gin.HandlerFunc {
	return func(gc *gin.Context) {
		if c == nil || c.TickerStore == nil {
			gc.JSON(http.StatusNotFound, gin.H{"error": "Ticker not found"})
			return
		}
		pid := gc.Param("productID")
		ev, ok := c.TickerStore.GetTicker(pid)
		if !ok {
			gc.JSON(http.StatusNotFound, gin.H{"error": "Ticker not found"})
			return
		}

		met, err := metrics.FromTicker(ev)
		if err != nil {
			gc.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var req MarketTickerOMRScoreRequest
		if err := gc.ShouldBindJSON(&req); err != nil {
			gc.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		manual := omrinput.ManualFields{
			RSI:                    req.RSI,
			SupportDistancePercent: req.SupportDistancePercent,
			VolumeTrend:            overnightmeanreversion.VolumeTrend(req.VolumeTrend),
			CandleSignal:           overnightmeanreversion.CandleSignal(req.CandleSignal),
			HigherLowDetected:      req.HigherLowDetected,
			PlannedEntry:           req.PlannedEntry,
			StopLoss:               req.StopLoss,
			TargetPrice:            req.TargetPrice,
		}

		input, err := omrinput.InputFromMetrics(met, manual)
		if err != nil {
			gc.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		out := overnightmeanreversion.Score(input)
		gc.JSON(http.StatusOK, out)
	}
}
