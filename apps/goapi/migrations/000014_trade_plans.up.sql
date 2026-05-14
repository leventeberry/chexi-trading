CREATE TABLE IF NOT EXISTS trade_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    symbol VARCHAR(32) NOT NULL,
    strategy_name VARCHAR(128) NOT NULL,
    direction VARCHAR(8) NOT NULL CHECK (direction IN ('LONG', 'SHORT')),
    thesis TEXT NOT NULL,
    planned_entry DOUBLE PRECISION NOT NULL,
    stop_loss DOUBLE PRECISION NOT NULL,
    target_price DOUBLE PRECISION NOT NULL,
    position_size DOUBLE PRECISION NOT NULL,
    max_risk_amount DOUBLE PRECISION NOT NULL,
    risk_reward_ratio DOUBLE PRECISION NOT NULL,
    source_score DOUBLE PRECISION,
    source_label VARCHAR(32),
    notes TEXT,
    status VARCHAR(16) NOT NULL CHECK (status IN ('PLANNED', 'ACTIVE', 'CLOSED', 'CANCELED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_trade_plans_user_created ON trade_plans(user_id, created_at DESC);
