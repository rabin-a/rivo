-- +migrate Up
CREATE TABLE rivo_periodic_jobs (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    namespace       TEXT NOT NULL DEFAULT 'default',
    kind            TEXT NOT NULL,
    queue           TEXT NOT NULL DEFAULT 'default',
    cron_expr       TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'UTC',
    payload         JSONB NOT NULL DEFAULT '{}',

    enabled         BOOLEAN NOT NULL DEFAULT true,
    last_run_at     TIMESTAMPTZ,
    next_run_at     TIMESTAMPTZ NOT NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (namespace, name)
);

CREATE INDEX rivo_periodic_jobs_next_run_idx ON rivo_periodic_jobs (next_run_at)
    WHERE enabled = true;

-- +migrate Down
DROP TABLE IF EXISTS rivo_periodic_jobs;
