-- Add telegram_update_id column to images table for deduplication
ALTER TABLE images ADD COLUMN telegram_update_id INTEGER DEFAULT 0;
