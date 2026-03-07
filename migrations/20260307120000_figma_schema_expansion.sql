-- Migration: Figma schema expansion
-- Adds all new columns, enums, and tables required by frontend designs

-- ── New enum types ──────────────────────────────────────────────────────────

DO $$ BEGIN
    CREATE TYPE host_application_status AS ENUM ('draft', 'pending', 'under_review', 'approved', 'rejected');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE event_status AS ENUM ('draft', 'live', 'paused');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE event_mood AS ENUM ('adventure', 'social', 'wellness', 'chill', 'romantic', 'intellectual', 'foodie', 'nightlife');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE cancellation_policy AS ENUM ('flexible', 'moderate', 'strict');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE support_category AS ENUM ('report_participant', 'technical_support', 'policy_help');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE message_sender_type AS ENUM ('system', 'host', 'guest');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ── users table ─────────────────────────────────────────────────────────────

ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS city TEXT;

-- ── hosts table ─────────────────────────────────────────────────────────────

-- Rename name → first_name (if still exists)
DO $$ BEGIN
    ALTER TABLE hosts RENAME COLUMN name TO first_name;
EXCEPTION WHEN undefined_column THEN NULL;
END $$;

ALTER TABLE hosts ADD COLUMN IF NOT EXISTS last_name TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS city TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS avatar_url TEXT;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS tagline TEXT;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS bio TEXT;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS application_status host_application_status NOT NULL DEFAULT 'draft';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS experience_desc TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS moods TEXT[] DEFAULT '{}';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS preferred_days TEXT[] DEFAULT '{}';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS group_size INTEGER;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS government_id_url TEXT;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS rejected_at TIMESTAMPTZ;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS is_identity_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS is_email_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS is_phone_verified BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS is_super_host BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS is_community_champ BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS expertise_tags TEXT[] DEFAULT '{}';
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS social_instagram TEXT;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS social_linkedin TEXT;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS social_website TEXT;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS avg_rating DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS total_reviews INTEGER NOT NULL DEFAULT 0;

-- ── events table ────────────────────────────────────────────────────────────

-- Rename name → title (if still exists)
DO $$ BEGIN
    ALTER TABLE events RENAME COLUMN name TO title;
EXCEPTION WHEN undefined_column THEN NULL;
END $$;

ALTER TABLE events ADD COLUMN IF NOT EXISTS hook_line TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS mood event_mood;
ALTER TABLE events ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS cover_image_url TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS gallery_urls TEXT[] DEFAULT '{}';
ALTER TABLE events ADD COLUMN IF NOT EXISTS is_online BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE events ADD COLUMN IF NOT EXISTS location TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS location_lat DOUBLE PRECISION;
ALTER TABLE events ADD COLUMN IF NOT EXISTS location_lng DOUBLE PRECISION;
ALTER TABLE events ADD COLUMN IF NOT EXISTS duration_minutes INTEGER;
ALTER TABLE events ADD COLUMN IF NOT EXISTS min_group_size INTEGER;
ALTER TABLE events ADD COLUMN IF NOT EXISTS max_group_size INTEGER;
ALTER TABLE events ADD COLUMN IF NOT EXISTS price_cents BIGINT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS is_free BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE events ADD COLUMN IF NOT EXISTS is_recurring BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE events ADD COLUMN IF NOT EXISTS recurrence_rule TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS cancellation_policy cancellation_policy;
ALTER TABLE events ADD COLUMN IF NOT EXISTS status event_status NOT NULL DEFAULT 'draft';
ALTER TABLE events ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;
ALTER TABLE events ADD COLUMN IF NOT EXISTS paused_at TIMESTAMPTZ;
ALTER TABLE events ADD COLUMN IF NOT EXISTS avg_rating DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE events ADD COLUMN IF NOT EXISTS total_bookings INTEGER NOT NULL DEFAULT 0;

-- ── inbox_messages table ────────────────────────────────────────────────────

-- Drop old host_id column if exists, add new sender columns
ALTER TABLE inbox_messages ADD COLUMN IF NOT EXISTS sender_type message_sender_type NOT NULL DEFAULT 'host';
ALTER TABLE inbox_messages ADD COLUMN IF NOT EXISTS sender_id UUID;
ALTER TABLE inbox_messages ADD COLUMN IF NOT EXISTS attachment_url TEXT;
ALTER TABLE inbox_messages ADD COLUMN IF NOT EXISTS is_read BOOLEAN NOT NULL DEFAULT FALSE;

-- Remove old host_id if it exists (data should be migrated to sender_id first in production)
-- ALTER TABLE inbox_messages DROP COLUMN IF EXISTS host_id;

-- ── reviews table ───────────────────────────────────────────────────────────

ALTER TABLE reviews ADD COLUMN IF NOT EXISTS rating INTEGER NOT NULL DEFAULT 5;

-- Add constraint for rating range
DO $$ BEGIN
    ALTER TABLE reviews ADD CONSTRAINT reviews_rating_check CHECK (rating >= 1 AND rating <= 5);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ── support_tickets table ───────────────────────────────────────────────────

ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS category support_category;
ALTER TABLE support_tickets ADD COLUMN IF NOT EXISTS reported_user_id UUID REFERENCES users(id);

-- ── saved_experiences table (new) ───────────────────────────────────────────

CREATE TABLE IF NOT EXISTS saved_experiences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, event_id)
);

CREATE INDEX IF NOT EXISTS idx_saved_experiences_user_id ON saved_experiences(user_id);
CREATE INDEX IF NOT EXISTS idx_saved_experiences_event_id ON saved_experiences(event_id);

-- ── Useful indexes ──────────────────────────────────────────────────────────

CREATE INDEX IF NOT EXISTS idx_hosts_application_status ON hosts(application_status);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
CREATE INDEX IF NOT EXISTS idx_events_host_id_status ON events(host_id, status);
CREATE INDEX IF NOT EXISTS idx_events_time ON events(time);
CREATE INDEX IF NOT EXISTS idx_inbox_messages_sender ON inbox_messages(sender_type, sender_id);
CREATE INDEX IF NOT EXISTS idx_inbox_messages_is_read ON inbox_messages(is_read);
