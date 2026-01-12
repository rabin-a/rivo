-- +migrate Up

-- Queue configuration for dynamic concurrency
CREATE TABLE IF NOT EXISTS rivo_queues (
    name            TEXT NOT NULL,
    namespace       TEXT NOT NULL DEFAULT 'default',
    concurrency     INTEGER NOT NULL DEFAULT 10,
    paused          BOOLEAN NOT NULL DEFAULT false,
    priority        INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (namespace, name)
);

-- Track active workers
CREATE TABLE IF NOT EXISTS rivo_workers (
    id              TEXT PRIMARY KEY,
    namespace       TEXT NOT NULL DEFAULT 'default',
    queues          TEXT[] NOT NULL DEFAULT '{}',
    concurrency     INTEGER NOT NULL DEFAULT 10,
    status          TEXT NOT NULL DEFAULT 'active',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_heartbeat  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    jobs_processed  BIGINT NOT NULL DEFAULT 0,
    jobs_failed     BIGINT NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX rivo_workers_status_idx ON rivo_workers (status, last_heartbeat);
CREATE INDEX rivo_workers_namespace_idx ON rivo_workers (namespace);

-- Detailed job execution logs
CREATE TABLE IF NOT EXISTS rivo_job_logs (
    id              BIGSERIAL PRIMARY KEY,
    job_id          BIGINT NOT NULL REFERENCES rivo_jobs(id) ON DELETE CASCADE,
    attempt         SMALLINT NOT NULL,
    worker_id       TEXT,

    -- Timing
    started_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ,
    duration_ms     INTEGER,

    -- Input/Output
    input           JSONB,
    output          JSONB,

    -- Status
    status          TEXT NOT NULL DEFAULT 'running',
    error           TEXT,
    error_stack     TEXT,

    -- Logs (array of log entries)
    logs            JSONB NOT NULL DEFAULT '[]',

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX rivo_job_logs_job_idx ON rivo_job_logs (job_id, attempt);
CREATE INDEX rivo_job_logs_worker_idx ON rivo_job_logs (worker_id);
CREATE INDEX rivo_job_logs_created_idx ON rivo_job_logs (created_at);

-- Queue statistics view
CREATE OR REPLACE VIEW rivo_queue_stats AS
SELECT
    namespace,
    queue,
    COUNT(*) FILTER (WHERE state = 'available') as available,
    COUNT(*) FILTER (WHERE state = 'running') as running,
    COUNT(*) FILTER (WHERE state = 'completed') as completed,
    COUNT(*) FILTER (WHERE state = 'retryable') as retryable,
    COUNT(*) FILTER (WHERE state = 'discarded') as failed,
    COUNT(*) as total
FROM rivo_jobs
GROUP BY namespace, queue;

-- +migrate Down
DROP VIEW IF EXISTS rivo_queue_stats;
DROP TABLE IF EXISTS rivo_job_logs;
DROP TABLE IF EXISTS rivo_workers;
