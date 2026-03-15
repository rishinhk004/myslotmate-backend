-- Add frontend-aligned event mood values while keeping legacy values for backward compatibility.

ALTER TYPE event_mood ADD VALUE IF NOT EXISTS 'adventurous';
ALTER TYPE event_mood ADD VALUE IF NOT EXISTS 'relaxing';
ALTER TYPE event_mood ADD VALUE IF NOT EXISTS 'creative';
ALTER TYPE event_mood ADD VALUE IF NOT EXISTS 'educational';
ALTER TYPE event_mood ADD VALUE IF NOT EXISTS 'culinary';
ALTER TYPE event_mood ADD VALUE IF NOT EXISTS 'cultural';
