-- Add total_reviews column to events table
ALTER TABLE events ADD COLUMN total_reviews INT DEFAULT 0 NOT NULL;

-- Populate total_reviews by counting existing reviews for each event
UPDATE events SET total_reviews = (
    SELECT COUNT(*) FROM reviews WHERE reviews.event_id = events.id
);
