-- Append-only audit / domain events. Extend via event_type + metadata JSONB without schema churn.
CREATE TABLE IF NOT EXISTS event_log (
    id BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    event_type TEXT NOT NULL,
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    subject TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_event_log_occurred_at ON event_log (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_event_log_event_type ON event_log (event_type);
CREATE INDEX IF NOT EXISTS idx_event_log_actor ON event_log (actor_user_id);
CREATE INDEX IF NOT EXISTS idx_event_log_request_id ON event_log (request_id);
CREATE INDEX IF NOT EXISTS idx_event_log_metadata ON event_log USING GIN (metadata);
