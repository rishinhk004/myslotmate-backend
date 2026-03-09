-- +migrate Up
-- Add gateway-level tracking columns to the payments table for Razorpay
-- Standard payment collection (wallet top-up).

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS gateway_order_id   TEXT,    -- Razorpay order_xxxxx
    ADD COLUMN IF NOT EXISTS gateway_payment_id TEXT;    -- Razorpay pay_xxxxx

-- Index for fast webhook reconciliation by order ID.
CREATE INDEX IF NOT EXISTS idx_payments_gateway_order_id
    ON payments (gateway_order_id)
    WHERE gateway_order_id IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_payments_gateway_order_id;
ALTER TABLE payments
    DROP COLUMN IF EXISTS gateway_payment_id,
    DROP COLUMN IF EXISTS gateway_order_id;
