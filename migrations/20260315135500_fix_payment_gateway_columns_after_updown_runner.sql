-- Ensure top-up gateway tracking columns exist in payments.
-- This migration repairs databases where an older migration runner
-- executed both Up and Down sections from mixed-format migration files.

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS gateway_order_id   TEXT,
    ADD COLUMN IF NOT EXISTS gateway_payment_id TEXT;

CREATE INDEX IF NOT EXISTS idx_payments_gateway_order_id
    ON payments (gateway_order_id)
    WHERE gateway_order_id IS NOT NULL;
