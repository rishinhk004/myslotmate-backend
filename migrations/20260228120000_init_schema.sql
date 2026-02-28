-- MySlotMate initial schema (PostgreSQL)
-- Run via: docker compose run --rm migrate

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enums
CREATE TYPE account_owner_type AS ENUM ('user', 'host');
CREATE TYPE booking_status AS ENUM ('pending', 'confirmed', 'cancelled', 'refunded');
CREATE TYPE payment_type AS ENUM ('booking', 'withdrawal', 'refund', 'payout', 'topup');
CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'reversed');
CREATE TYPE support_ticket_status AS ENUM ('open', 'in_progress', 'resolved', 'closed');
CREATE TYPE fraud_flag_type AS ENUM (
  'abnormal_booking_spike',
  'payment_abuse',
  'suspicious_activity',
  'manual_block'
);

-- ---------------------------------------------------------------------------
-- accounts (one per user – wallet + bank details)
-- ---------------------------------------------------------------------------
CREATE TABLE accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_type account_owner_type NOT NULL,
  owner_id UUID NOT NULL,
  balance_cents BIGINT NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
  bank_details JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_account_per_owner UNIQUE (owner_type, owner_id)
);

CREATE INDEX idx_accounts_owner ON accounts (owner_type, owner_id);

-- ---------------------------------------------------------------------------
-- users (anyone can register; is_verified required to become host)
-- ---------------------------------------------------------------------------
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  auth_uid TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL DEFAULT '',
  phn_number TEXT NOT NULL DEFAULT '',
  email TEXT DEFAULT '',
  account_id UUID REFERENCES accounts (id) ON DELETE SET NULL,
  is_verified BOOLEAN NOT NULL DEFAULT false,
  verified_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_auth_uid ON users (auth_uid);
CREATE INDEX idx_users_account ON users (account_id);
CREATE INDEX idx_users_verified ON users (is_verified);

-- ---------------------------------------------------------------------------
-- hosts (verified users only; one host profile per user)
-- ---------------------------------------------------------------------------
CREATE TABLE hosts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
  name TEXT NOT NULL DEFAULT '',
  phn_number TEXT NOT NULL DEFAULT '',
  account_id UUID REFERENCES accounts (id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT host_user_must_be_verified CHECK (
    EXISTS (SELECT 1 FROM users u WHERE u.id = user_id AND u.is_verified = true)
  )
);

CREATE UNIQUE INDEX idx_hosts_user_id ON hosts (user_id);
CREATE INDEX idx_hosts_account ON hosts (account_id);

-- ---------------------------------------------------------------------------
-- events (created by hosts; capacity for overbooking prevention)
-- ---------------------------------------------------------------------------
CREATE TABLE events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id UUID NOT NULL REFERENCES hosts (id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  "time" TIMESTAMPTZ NOT NULL,
  end_time TIMESTAMPTZ,
  capacity INT NOT NULL DEFAULT 0 CHECK (capacity >= 0),
  ai_suggestion TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_events_host ON events (host_id);
CREATE INDEX idx_events_time ON events ("time");

-- ---------------------------------------------------------------------------
-- bookings (user books event; status + quantity; overbooking enforced in app)
-- ---------------------------------------------------------------------------
CREATE TABLE bookings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  quantity INT NOT NULL CHECK (quantity > 0),
  status booking_status NOT NULL DEFAULT 'pending',
  payment_id UUID,
  idempotency_key TEXT UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  cancelled_at TIMESTAMPTZ
);

CREATE INDEX idx_bookings_event ON bookings (event_id);
CREATE INDEX idx_bookings_user ON bookings (user_id);
CREATE INDEX idx_bookings_status ON bookings (event_id, status);
CREATE UNIQUE INDEX idx_bookings_idempotency ON bookings (idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Add FK to payments after payments table exists (see below)
-- ALTER TABLE bookings ADD CONSTRAINT fk_bookings_payment FOREIGN KEY (payment_id) REFERENCES payments (id);

-- ---------------------------------------------------------------------------
-- reviews (per event; AI sentiment + host replies)
-- ---------------------------------------------------------------------------
CREATE TABLE reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  name TEXT,
  description TEXT NOT NULL,
  reply TEXT[] DEFAULT '{}',
  sentiment_score DOUBLE PRECISION,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reviews_event ON reviews (event_id);
CREATE INDEX idx_reviews_user ON reviews (user_id);

-- ---------------------------------------------------------------------------
-- payments (idempotency, retry, status)
-- ---------------------------------------------------------------------------
CREATE TABLE payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key TEXT NOT NULL UNIQUE,
  account_id UUID NOT NULL REFERENCES accounts (id) ON DELETE RESTRICT,
  type payment_type NOT NULL,
  reference_id UUID,
  amount_cents BIGINT NOT NULL,
  status payment_status NOT NULL DEFAULT 'pending',
  retry_count INT NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_account ON payments (account_id);
CREATE INDEX idx_payments_status ON payments (status);
CREATE UNIQUE INDEX idx_payments_idempotency ON payments (idempotency_key);

-- Link bookings.payment_id to payments
ALTER TABLE bookings
  ADD CONSTRAINT fk_bookings_payment FOREIGN KEY (payment_id) REFERENCES payments (id) ON DELETE SET NULL;

-- ---------------------------------------------------------------------------
-- inbox_messages (host broadcasts per event)
-- ---------------------------------------------------------------------------
CREATE TABLE inbox_messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id UUID NOT NULL REFERENCES events (id) ON DELETE CASCADE,
  host_id UUID NOT NULL REFERENCES hosts (id) ON DELETE CASCADE,
  message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_inbox_messages_host ON inbox_messages (host_id);
CREATE INDEX idx_inbox_messages_event ON inbox_messages (event_id);

-- ---------------------------------------------------------------------------
-- support_tickets (host/user ↔ support)
-- ---------------------------------------------------------------------------
CREATE TABLE support_tickets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  subject TEXT NOT NULL,
  messages JSONB NOT NULL DEFAULT '[]',
  status support_ticket_status NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_support_tickets_user ON support_tickets (user_id);
CREATE INDEX idx_support_tickets_status ON support_tickets (status);

-- ---------------------------------------------------------------------------
-- fraud_flags (block / flag suspicious users)
-- ---------------------------------------------------------------------------
CREATE TABLE fraud_flags (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  type fraud_flag_type NOT NULL,
  reason TEXT,
  blocked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  blocked_until TIMESTAMPTZ,
  is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX idx_fraud_flags_user ON fraud_flags (user_id);
CREATE INDEX idx_fraud_flags_active ON fraud_flags (user_id, is_active) WHERE is_active = true;

-- ---------------------------------------------------------------------------
-- event_reviews (optional: denormalized list of review ids on event for quick access)
-- ---------------------------------------------------------------------------
-- We derive event reviews via reviews.event_id; no extra table needed.
-- If you want an events.review_ids UUID[] column:
-- ALTER TABLE events ADD COLUMN review_ids UUID[] DEFAULT '{}';

-- ---------------------------------------------------------------------------
-- RLS (Row Level Security) – optional: enable and add policies per table
-- ---------------------------------------------------------------------------
-- ALTER TABLE users ENABLE ROW LEVEL SECURITY;
-- ALTER TABLE hosts ENABLE ROW LEVEL SECURITY;
-- ... (add policies based on auth.uid() matching user_id / host_id)

-- ---------------------------------------------------------------------------
-- updated_at triggers (optional)
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER accounts_updated_at BEFORE UPDATE ON accounts FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Auto-create account (wallet) for each new user
CREATE OR REPLACE FUNCTION create_user_account()
RETURNS TRIGGER AS $$
DECLARE
  new_account_id UUID;
BEGIN
  IF NEW.account_id IS NULL THEN
    INSERT INTO accounts (owner_type, owner_id, balance_cents)
    VALUES ('user', NEW.id, 0)
    RETURNING id INTO new_account_id;
    NEW.account_id := new_account_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER create_account_on_user_insert
  BEFORE INSERT ON users
  FOR EACH ROW
  EXECUTE PROCEDURE create_user_account();

CREATE OR REPLACE FUNCTION create_host_account()
RETURNS TRIGGER AS $$
DECLARE
  new_account_id UUID;
BEGIN
  IF NEW.account_id IS NULL THEN
    INSERT INTO accounts (owner_type, owner_id, balance_cents)
    VALUES ('host', NEW.id, 0)
    RETURNING id INTO new_account_id;
    NEW.account_id := new_account_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER create_account_on_host_insert
  BEFORE INSERT ON hosts
  FOR EACH ROW
  EXECUTE PROCEDURE create_host_account();
CREATE TRIGGER hosts_updated_at BEFORE UPDATE ON hosts FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER events_updated_at BEFORE UPDATE ON events FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER bookings_updated_at BEFORE UPDATE ON bookings FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER reviews_updated_at BEFORE UPDATE ON reviews FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER payments_updated_at BEFORE UPDATE ON payments FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
CREATE TRIGGER support_tickets_updated_at BEFORE UPDATE ON support_tickets FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
