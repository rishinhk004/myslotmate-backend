-- Add 'processing' to payment_status (must run in autocommit)
ALTER TYPE payment_status ADD VALUE IF NOT EXISTS 'processing';
