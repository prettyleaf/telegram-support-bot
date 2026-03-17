-- Migration: 000003_add_hwid_delete_tracking
-- Description: Track the last time a user removed an HWID device via the bot

ALTER TABLE user_settings
    ADD COLUMN IF NOT EXISTS last_hwid_delete_at TIMESTAMP WITH TIME ZONE;
