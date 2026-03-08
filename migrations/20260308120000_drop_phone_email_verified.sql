-- +migrate Up
-- Remove phone and email verification columns from hosts table
ALTER TABLE hosts DROP COLUMN IF EXISTS is_email_verified;
ALTER TABLE hosts DROP COLUMN IF EXISTS is_phone_verified;

-- +migrate Down
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS is_email_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS is_phone_verified BOOLEAN NOT NULL DEFAULT FALSE;
