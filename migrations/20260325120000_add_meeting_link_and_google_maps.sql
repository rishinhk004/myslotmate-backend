-- Migration: Add meeting link for online events and Google Maps integration
-- Purpose:
-- 1. Add meeting_link column for online events (zoom, teams, google meet, etc.)
-- 2. Add google_maps_url column for location-based events (direct Google Maps link)

ALTER TABLE events ADD COLUMN IF NOT EXISTS meeting_link TEXT;
ALTER TABLE events ADD COLUMN IF NOT EXISTS google_maps_url TEXT;

-- Add index for online events with meeting links
CREATE INDEX IF NOT EXISTS idx_events_online_meeting ON events (is_online, meeting_link) WHERE is_online = true AND meeting_link IS NOT NULL;

-- Add index for location-based events with Google Maps links
CREATE INDEX IF NOT EXISTS idx_events_location_maps ON events (location, google_maps_url) WHERE location IS NOT NULL AND google_maps_url IS NOT NULL;
