-- Earnings & Payouts schema for MySlotMate
-- Supports: earnings summary, platform fee breakdown, payout methods, payout history

-- Payout method type
CREATE TYPE payout_method_type AS ENUM ('bank', 'upi');

-- ---------------------------------------------------------------------------
-- platform_settings (fee config for earnings breakdown)
-- ---------------------------------------------------------------------------
CREATE TABLE platform_settings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key TEXT NOT NULL UNIQUE,
  value JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed default platform fee (85% host, 15% platform)
INSERT INTO platform_settings (key, value) VALUES
  ('platform_fee', '{"host_percentage": 85, "platform_percentage": 15}')
ON CONFLICT (key) DO NOTHING;

-- ---------------------------------------------------------------------------
-- payout_methods (bank accounts, UPI – multiple per host)
-- ---------------------------------------------------------------------------
CREATE TABLE payout_methods (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id UUID NOT NULL REFERENCES hosts (id) ON DELETE CASCADE,
  type payout_method_type NOT NULL,
  -- Bank fields
  bank_name TEXT,
  account_type TEXT,           -- 'checking', 'savings'
  last_four_digits TEXT,       -- masked display: **** 4567
  account_number_encrypted TEXT, -- encrypted full number (optional)
  ifsc TEXT,
  beneficiary_name TEXT,
  -- UPI fields
  upi_id TEXT,
  -- Common
  is_verified BOOLEAN NOT NULL DEFAULT false,
  is_primary BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT payout_method_bank_check CHECK (
    (type = 'bank' AND bank_name IS NOT NULL AND last_four_digits IS NOT NULL) OR
    (type = 'upi' AND upi_id IS NOT NULL)
  )
);

CREATE INDEX idx_payout_methods_host ON payout_methods (host_id);
CREATE INDEX idx_payout_methods_primary ON payout_methods (host_id, is_primary) WHERE is_primary = true;

-- ---------------------------------------------------------------------------
-- host_earnings (aggregate earnings per host)
-- ---------------------------------------------------------------------------
CREATE TABLE host_earnings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id UUID NOT NULL UNIQUE REFERENCES hosts (id) ON DELETE CASCADE,
  total_earnings_cents BIGINT NOT NULL DEFAULT 0 CHECK (total_earnings_cents >= 0),
  pending_clearance_cents BIGINT NOT NULL DEFAULT 0 CHECK (pending_clearance_cents >= 0),
  estimated_clearance_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_host_earnings_host ON host_earnings (host_id);

-- Auto-create host_earnings when host is created
CREATE OR REPLACE FUNCTION create_host_earnings()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO host_earnings (host_id)
  VALUES (NEW.id)
  ON CONFLICT (host_id) DO NOTHING;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER create_earnings_on_host_insert
  AFTER INSERT ON hosts
  FOR EACH ROW
  EXECUTE PROCEDURE create_host_earnings();

-- ---------------------------------------------------------------------------
-- Extend payments for payouts (payout_method_id, display reference)
-- ---------------------------------------------------------------------------
ALTER TABLE payments ADD COLUMN IF NOT EXISTS payout_method_id UUID REFERENCES payout_methods (id) ON DELETE SET NULL;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS display_reference TEXT;  -- e.g. TXN-88234
CREATE INDEX idx_payments_payout_method ON payments (payout_method_id) WHERE payout_method_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Extend bookings for platform fee breakdown (amount, service_fee, net_earning)
-- ---------------------------------------------------------------------------
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS amount_cents BIGINT;           -- total booking value
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS service_fee_cents BIGINT;     -- platform fee (15%)
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS net_earning_cents BIGINT;     -- host net (85%)

-- ---------------------------------------------------------------------------
-- Triggers
-- ---------------------------------------------------------------------------
CREATE TRIGGER platform_settings_updated_at BEFORE UPDATE ON platform_settings FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER payout_methods_updated_at BEFORE UPDATE ON payout_methods FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER host_earnings_updated_at BEFORE UPDATE ON host_earnings FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
