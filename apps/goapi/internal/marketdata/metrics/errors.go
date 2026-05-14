package metrics

import "errors"

// Sentinel errors for invalid ticker inputs or undefined derived metrics.
var (
	ErrEmptyProductID          = errors.New("market metrics: product_id is required")
	ErrNonPositiveCurrentPrice = errors.New("market metrics: current_price must be greater than zero")
	ErrInvalidSpread           = errors.New("market metrics: best_ask must be greater than or equal to best_bid")
	ErrOpen24hZero             = errors.New("market metrics: open_24h is zero; percent_change_24h is undefined")
	ErrInvalidOpen24h          = errors.New("market metrics: open_24h must be finite and non-zero")
	ErrHigh24hNonPositive      = errors.New("market metrics: high_24h must be greater than zero for intraday_drawdown_percent")
	ErrDegenerateHighLowRange  = errors.New("market metrics: high_24h equals low_24h; range_position_percent is undefined")
	ErrInvalidVolume24h        = errors.New("market metrics: volume_24h must be finite and non-negative")
)
