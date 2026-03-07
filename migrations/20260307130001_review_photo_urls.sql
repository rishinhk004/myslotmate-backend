-- Migration: Add photo_urls column to reviews table

ALTER TABLE reviews
    ADD COLUMN IF NOT EXISTS photo_urls TEXT[] DEFAULT '{}';
