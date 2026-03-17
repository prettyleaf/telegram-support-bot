-- Support Ticket Bot Database Schema
-- ==================================

-- Tickets table - stores all support tickets
CREATE TABLE IF NOT EXISTS tickets (
    id SERIAL PRIMARY KEY,
    ticket_number VARCHAR(20) UNIQUE NOT NULL,
    user_id BIGINT NOT NULL,
    user_name TEXT,
    user_username TEXT,
    support_chat_id BIGINT NOT NULL,
    thread_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    priority VARCHAR(10) DEFAULT 'normal',
    subject TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE,
    closed_by BIGINT,
    closed_by_name TEXT
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_thread_id ON tickets(thread_id);
CREATE INDEX IF NOT EXISTS idx_tickets_ticket_number ON tickets(ticket_number);
CREATE INDEX IF NOT EXISTS idx_tickets_created_at ON tickets(created_at DESC);

-- Ticket messages - stores message history for tickets
CREATE TABLE IF NOT EXISTS ticket_messages (
    id SERIAL PRIMARY KEY,
    ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    sender_id BIGINT NOT NULL,
    sender_name TEXT,
    sender_username TEXT,
    is_admin BOOLEAN DEFAULT FALSE,
    message_text TEXT,
    message_id BIGINT,
    original_message_id BIGINT,
    has_media BOOLEAN DEFAULT FALSE,
    media_type VARCHAR(50),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ticket_messages_ticket_id ON ticket_messages(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_created_at ON ticket_messages(created_at);

-- Ticket notes - internal admin notes (not visible to users)
CREATE TABLE IF NOT EXISTS ticket_notes (
    id SERIAL PRIMARY KEY,
    ticket_id INT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    admin_id BIGINT NOT NULL,
    admin_name TEXT,
    note_text TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ticket_notes_ticket_id ON ticket_notes(ticket_id);

-- Banned users - users banned from creating tickets
CREATE TABLE IF NOT EXISTS banned_users (
    id SERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL,
    user_name TEXT,
    user_username TEXT,
    banned_by BIGINT NOT NULL,
    banned_by_name TEXT,
    reason TEXT,
    banned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_banned_users_user_id ON banned_users(user_id);

-- Canned responses - predefined responses for common questions
CREATE TABLE IF NOT EXISTS canned_responses (
    id SERIAL PRIMARY KEY,
    shortcut VARCHAR(50) UNIQUE NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_canned_responses_shortcut ON canned_responses(shortcut);

-- Ticket statistics - for analytics
CREATE TABLE IF NOT EXISTS ticket_stats (
    id SERIAL PRIMARY KEY,
    date DATE UNIQUE NOT NULL,
    tickets_opened INT DEFAULT 0,
    tickets_closed INT DEFAULT 0,
    avg_response_time_minutes INT,
    avg_resolution_time_minutes INT
);

CREATE INDEX IF NOT EXISTS idx_ticket_stats_date ON ticket_stats(date DESC);

-- User onboarding - stores captcha verification state
CREATE TABLE IF NOT EXISTS user_onboarding (
    user_id BIGINT PRIMARY KEY,
    captcha_passed BOOLEAN NOT NULL DEFAULT FALSE,
    captcha_passed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- User settings - stores user preferences like language
CREATE TABLE IF NOT EXISTS user_settings (
    id SERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL,
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    last_hwid_delete_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_onboarding_captcha_passed ON user_onboarding(captcha_passed);
CREATE INDEX IF NOT EXISTS idx_user_settings_user_id ON user_settings(user_id);
