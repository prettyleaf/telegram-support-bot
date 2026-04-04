CREATE TABLE IF NOT EXISTS web_admin_users (
    id BIGSERIAL PRIMARY KEY,
    username TEXT UNIQUE,
    display_name TEXT NOT NULL,
    github_login TEXT UNIQUE,
    github_email TEXT UNIQUE,
    telegram_id BIGINT UNIQUE,
    webauthn_user_id BYTEA NOT NULL UNIQUE,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS web_admin_passkeys (
    id BIGSERIAL PRIMARY KEY,
    admin_user_id BIGINT NOT NULL REFERENCES web_admin_users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    credential_id TEXT NOT NULL UNIQUE,
    credential BYTEA NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_web_admin_users_username ON web_admin_users(username);
CREATE INDEX IF NOT EXISTS idx_web_admin_users_github_login ON web_admin_users(github_login);
CREATE INDEX IF NOT EXISTS idx_web_admin_users_github_email ON web_admin_users(github_email);
CREATE INDEX IF NOT EXISTS idx_web_admin_users_telegram_id ON web_admin_users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_web_admin_passkeys_admin_user_id ON web_admin_passkeys(admin_user_id);
