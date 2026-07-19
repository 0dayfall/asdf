-- Add status and location visibility to users
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status_visibility TEXT NOT NULL DEFAULT 'public'
        CHECK (status_visibility IN ('public','registered','friends','private')),
    ADD COLUMN IF NOT EXISTS location_visibility TEXT NOT NULL DEFAULT 'public'
        CHECK (location_visibility IN ('public','registered','friends','private'));

-- Per-link storage with visibility control
CREATE TABLE IF NOT EXISTS user_links (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform    TEXT NOT NULL DEFAULT '',
    rel         TEXT NOT NULL,
    href        TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    visibility  TEXT NOT NULL DEFAULT 'public'
                    CHECK (visibility IN ('public','registered','friends','private')),
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_links_user_id   ON user_links(user_id);
CREATE INDEX IF NOT EXISTS idx_user_links_visibility ON user_links(visibility);

-- Friend connections
CREATE TABLE IF NOT EXISTS connections (
    id           SERIAL PRIMARY KEY,
    requester_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','accepted','blocked')),
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(requester_id, addressee_id)
);

CREATE INDEX IF NOT EXISTS idx_connections_requester ON connections(requester_id);
CREATE INDEX IF NOT EXISTS idx_connections_addressee ON connections(addressee_id);
CREATE INDEX IF NOT EXISTS idx_connections_status    ON connections(status);

-- OAuth linked accounts
CREATE TABLE IF NOT EXISTS oauth_accounts (
    id                    SERIAL PRIMARY KEY,
    user_id               INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider              TEXT NOT NULL,
    provider_user_id      TEXT NOT NULL,
    provider_username     TEXT NOT NULL DEFAULT '',
    provider_display_name TEXT NOT NULL DEFAULT '',
    access_token          TEXT NOT NULL DEFAULT '',
    refresh_token         TEXT NOT NULL DEFAULT '',
    token_expires_at      TIMESTAMP WITH TIME ZONE,
    created_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at            TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user_id  ON oauth_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_oauth_accounts_provider ON oauth_accounts(provider, provider_user_id);
