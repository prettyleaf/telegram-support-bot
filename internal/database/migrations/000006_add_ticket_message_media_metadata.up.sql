ALTER TABLE ticket_messages
    ADD COLUMN IF NOT EXISTS media_file_id TEXT,
    ADD COLUMN IF NOT EXISTS media_file_name TEXT,
    ADD COLUMN IF NOT EXISTS media_mime_type TEXT;
