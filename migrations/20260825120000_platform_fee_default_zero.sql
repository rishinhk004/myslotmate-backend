-- +migrate Up
-- Platform takes 0% by default; host keeps 100%. Per-host overrides
-- (hosts.platform_fee_percentage) are unaffected.
UPDATE platform_settings
SET value = '{"host_percentage": 100, "platform_percentage": 0}', updated_at = NOW()
WHERE key = 'platform_fee';

-- +migrate Down
UPDATE platform_settings
SET value = '{"host_percentage": 70, "platform_percentage": 30}', updated_at = NOW()
WHERE key = 'platform_fee';
