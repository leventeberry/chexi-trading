DROP TABLE IF EXISTS user_mfa_recovery_codes;
DROP TABLE IF EXISTS mfa_challenges;
DROP TABLE IF EXISTS user_totp_factors;

ALTER TABLE users DROP COLUMN IF EXISTS totp_enabled;
