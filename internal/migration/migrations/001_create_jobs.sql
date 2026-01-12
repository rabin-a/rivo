-- +migrate Up
CREATE TABLE rivo_jobs (
    id              BIGSERIAL PRIMARY KEY,
    kind            TEXT NOT NULL,
    queue           TEXT NOT NULL DEFAULT 'default',
    namespace       TEXT NOT NULL DEFAULT 'default',
    priority        SMALLINT NOT NULL DEFAULT 0,
    state           TEXT NOT NULL DEFAULT 'available',
    payload         JSONB NOT NULL DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',

    -- Scheduling
    scheduled_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Execution tracking
    attempt         SMALLINT NOT NULL DEFAULT 0,
    max_attempts    SMALLINT NOT NULL DEFAULT 25,
    attempted_at    TIMESTAMPTZ,
    attempted_by    TEXT,

    -- Completion
    completed_at    TIMESTAMPTZ,
    cancelled_at    TIMESTAMPTZ,
    discarded_at    TIMESTAMPTZ,

    -- Error tracking
    errors          JSONB NOT NULL DEFAULT '[]',

    -- Idempotency
    idempotency_key TEXT,

    -- Workflow association
    workflow_run_id BIGINT,
    step_id         TEXT,

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rivo_jobs_state_check CHECK (
        state IN ('available', 'scheduled', 'running', 'retryable',
                  'completed', 'cancelled', 'discarded')
    )
);

-- Primary index for job fetching (SKIP LOCKED pattern)
CREATE INDEX rivo_jobs_fetch_idx ON rivo_jobs (
    namespace, queue, priority DESC, scheduled_at, id
) WHERE state = 'available';

-- Index for scheduled jobs
CREATE INDEX rivo_jobs_scheduled_idx ON rivo_jobs (scheduled_at)
    WHERE state IN ('scheduled', 'retryable');

-- Unique index for idempotency
CREATE UNIQUE INDEX rivo_jobs_idempotency_idx ON rivo_jobs (
    namespace, kind, idempotency_key
) WHERE idempotency_key IS NOT NULL
  AND state NOT IN ('completed', 'cancelled', 'discarded');

-- Index for workflow jobs
CREATE INDEX rivo_jobs_workflow_idx ON rivo_jobs (workflow_run_id, step_id)
    WHERE workflow_run_id IS NOT NULL;

-- Index for job kind
CREATE INDEX rivo_jobs_kind_idx ON rivo_jobs (namespace, kind, state);

-- +migrate Down
DROP TABLE IF EXISTS rivo_jobs;
