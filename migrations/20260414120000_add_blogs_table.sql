-- Migration: Create blogs table
-- Purpose: Store blog posts that can be created by admins and read by everyone

CREATE TABLE IF NOT EXISTS blogs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title VARCHAR(255) NOT NULL,
  description VARCHAR(500),
  category VARCHAR(50) NOT NULL CHECK (category IN ('Hosting', 'Wellness', 'Adventure')),
  content TEXT NOT NULL,
  cover_image_url VARCHAR(500),
  author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  author_name VARCHAR(255) NOT NULL,
  read_time_minutes INT DEFAULT 5,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for faster queries
CREATE INDEX IF NOT EXISTS idx_blogs_published_at ON blogs(published_at DESC) WHERE published_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_blogs_category ON blogs(category);
CREATE INDEX IF NOT EXISTS idx_blogs_author_id ON blogs(author_id);
