DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS connections;
DROP TABLE IF EXISTS user_links;
ALTER TABLE users
    DROP COLUMN IF EXISTS location_visibility,
    DROP COLUMN IF EXISTS status_visibility,
    DROP COLUMN IF EXISTS status;
