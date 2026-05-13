-- Profile columns on users + 1:1 user_settings table.

ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(120);
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(2048);
ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone VARCHAR(64);
ALTER TABLE users ADD COLUMN IF NOT EXISTS locale VARCHAR(16);

CREATE TABLE IF NOT EXISTS user_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    theme VARCHAR(32) NOT NULL DEFAULT 'system',
    notification_preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
    marketing_email_opt_in BOOLEAN NOT NULL DEFAULT FALSE,
    security_notification_opt_in BOOLEAN NOT NULL DEFAULT TRUE,
    extra_settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
