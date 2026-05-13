-- Organization webhooks (outbound HTTP notifications; secret encrypted at rest).
CREATE TABLE IF NOT EXISTS organization_webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret_ciphertext BYTEA NOT NULL,
    secret_key_version INT NOT NULL DEFAULT 1,
    events TEXT[] NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS organization_webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES organization_webhooks(id) ON DELETE CASCADE,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(32) NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    response_status INT NULL,
    response_body_truncated TEXT NULL,
    last_error TEXT NULL,
    next_attempt_at TIMESTAMPTZ NULL,
    delivered_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_organization_webhooks_org_id ON organization_webhooks(organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_webhooks_enabled ON organization_webhooks(enabled);
CREATE INDEX IF NOT EXISTS idx_organization_webhook_deliveries_webhook_created ON organization_webhook_deliveries(webhook_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_organization_webhook_deliveries_retry ON organization_webhook_deliveries(status, next_attempt_at);
