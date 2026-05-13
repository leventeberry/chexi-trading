DROP INDEX IF EXISTS idx_organization_webhook_deliveries_retry;
DROP INDEX IF EXISTS idx_organization_webhook_deliveries_webhook_created;
DROP INDEX IF EXISTS idx_organization_webhooks_enabled;
DROP INDEX IF EXISTS idx_organization_webhooks_org_id;

DROP TABLE IF EXISTS organization_webhook_deliveries;
DROP TABLE IF EXISTS organization_webhooks;
