-- Migration: 000004_add_user_onboarding
-- Description: Add onboarding captcha completion tracking for private bot users

CREATE TABLE IF NOT EXISTS user_onboarding (
    user_id BIGINT PRIMARY KEY,
    captcha_passed BOOLEAN NOT NULL DEFAULT FALSE,
    captcha_passed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_onboarding_captcha_passed ON user_onboarding(captcha_passed);
