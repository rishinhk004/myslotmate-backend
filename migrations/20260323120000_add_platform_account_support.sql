-- Add platform account owner type for tracking platform fees
-- Run via: docker compose run --rm migrate

-- Add 'platform' to account_owner_type enum
ALTER TYPE account_owner_type ADD VALUE IF NOT EXISTS 'platform';

-- Ensure platform account exists (will be created in code on startup)
-- No automatic creation here — app will handle it
