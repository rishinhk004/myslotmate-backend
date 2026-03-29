-- Migration: Add notification tracking columns to bookings
-- Purpose:
-- 1. Track whether WhatsApp confirmation notification has been sent
-- 2. Track whether email reminder notification has been sent (1-2hr before event)

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS notification_sent_whatsapp BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS reminder_notification_sent_email BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS reminder_notification_sent_at TIMESTAMPTZ;

-- Create index for finding bookings that need reminder notifications
CREATE INDEX IF NOT EXISTS idx_bookings_pending_reminders ON bookings (event_id, status, reminder_notification_sent_email) 
  WHERE status IN ('pending', 'confirmed') AND reminder_notification_sent_email = false;
