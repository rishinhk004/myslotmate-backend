-- Remove DB-level gate requiring users.is_verified before host profile create/update.
-- Host verification is now granted on admin approval.

DROP TRIGGER IF EXISTS trg_host_user_must_be_verified ON hosts;
DROP FUNCTION IF EXISTS check_host_user_verified();
