-- Migration: 000003_add_hwid_delete_tracking (rollback)

ALTER TABLE user_settings
    DROP COLUMN IF EXISTS last_hwid_delete_at;
