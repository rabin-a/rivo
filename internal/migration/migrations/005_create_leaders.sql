-- +migrate Up
CREATE TABLE IF NOT EXISTS rivo_leaders (
    name            TEXT PRIMARY KEY,
    leader_id       TEXT NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +migrate Down
DROP TABLE IF EXISTS rivo_leaders;
