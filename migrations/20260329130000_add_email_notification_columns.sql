-- Migration: Add email notification tracking columns to bookings
-- Purpose:
-- 1. Add notification_sent_email column to track booking confirmation emails
-- 2. Add reminder_notification_sent_whatsapp column to track reminder WhatsApp messages
-- 3. Add reminder_whatsapp_sent_at to track when reminder WhatsApp was sent

ALTER TABLE bookings ADD COLUMN IF NOT EXISTS notification_sent_email BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS reminder_notification_sent_whatsapp BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS reminder_whatsapp_sent_at TIMESTAMPTZ;

-- Create indexes for faster queries
CREATE INDEX IF NOT EXISTS idx_bookings_pending_email_notifications ON bookings (status, notification_sent_email) 
  WHERE status IN ('pending', 'confirmed') AND notification_sent_email = false;
