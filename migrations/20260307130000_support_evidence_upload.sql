-- Migration: Add report_reason enum and evidence columns to support_tickets

-- ── New report_reason enum ──────────────────────────────────────────────────
DO $$ BEGIN
    CREATE TYPE report_reason AS ENUM (
        'verbal_harassment',
        'safety_concern',
        'inappropriate_behavior',
        'spam_or_scam'
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ── Add report-specific columns to support_tickets ──────────────────────────
ALTER TABLE support_tickets
    ADD COLUMN IF NOT EXISTS event_id       UUID REFERENCES events(id),
    ADD COLUMN IF NOT EXISTS session_date   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS report_reason  TEXT,
    ADD COLUMN IF NOT EXISTS evidence_urls  TEXT[] DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS is_urgent      BOOLEAN NOT NULL DEFAULT FALSE;
