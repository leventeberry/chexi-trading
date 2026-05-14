// Package risk provides pure helpers for manual trade plan geometry and sizing.
// It performs no I/O and does not execute trades.
package risk

import (
	"fmt"
	"strings"
)

// Direction is the intended trade direction.
type Direction string

const (
	DirectionLong  Direction = "LONG"
	DirectionShort Direction = "SHORT"
)

// ParseDirection returns a Direction or an error if s is not LONG or SHORT.
func ParseDirection(s string) (Direction, error) {
	switch Direction(strings.TrimSpace(strings.ToUpper(s))) {
	case DirectionLong:
		return DirectionLong, nil
	case DirectionShort:
		return DirectionShort, nil
	default:
		return "", fmt.Errorf("invalid direction %q (expected LONG or SHORT)", s)
	}
}

// ValidateGeometry checks stop / entry / target ordering for the direction.
func ValidateGeometry(d Direction, plannedEntry, stopLoss, targetPrice float64) error {
	switch d {
	case DirectionLong:
		if !(stopLoss < plannedEntry && plannedEntry < targetPrice) {
			return fmt.Errorf("LONG requires stop_loss < planned_entry < target_price")
		}
	case DirectionShort:
		if !(targetPrice < plannedEntry && plannedEntry < stopLoss) {
			return fmt.Errorf("SHORT requires target_price < planned_entry < stop_loss")
		}
	default:
		return fmt.Errorf("unknown direction")
	}
	return nil
}

// RiskRewardPerUnit returns risk per unit, reward per unit, and reward/risk ratio.
// riskPerUnit must be strictly positive.
func RiskRewardPerUnit(d Direction, plannedEntry, stopLoss, targetPrice float64) (riskPerUnit, rewardPerUnit, rr float64, err error) {
	if err := ValidateGeometry(d, plannedEntry, stopLoss, targetPrice); err != nil {
		return 0, 0, 0, err
	}
	switch d {
	case DirectionLong:
		riskPerUnit = plannedEntry - stopLoss
		rewardPerUnit = targetPrice - plannedEntry
	case DirectionShort:
		riskPerUnit = stopLoss - plannedEntry
		rewardPerUnit = plannedEntry - targetPrice
	default:
		return 0, 0, 0, fmt.Errorf("unknown direction")
	}
	if riskPerUnit <= 0 {
		return 0, 0, 0, fmt.Errorf("risk per unit must be positive")
	}
	rr = rewardPerUnit / riskPerUnit
	return riskPerUnit, rewardPerUnit, rr, nil
}

// MaxLoss is risk per unit times position size (same units as the instrument).
func MaxLoss(riskPerUnit, positionSize float64) float64 {
	return riskPerUnit * positionSize
}
