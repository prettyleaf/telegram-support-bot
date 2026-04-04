ALTER TABLE ticket_messages
    DROP COLUMN IF EXISTS media_mime_type,
    DROP COLUMN IF EXISTS media_file_name,
    DROP COLUMN IF EXISTS media_file_id;
