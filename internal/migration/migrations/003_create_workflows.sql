-- +migrate Up
CREATE TABLE rivo_workflow_definitions (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    version         INTEGER NOT NULL DEFAULT 1,
    namespace       TEXT NOT NULL DEFAULT 'default',

    -- DAG definition stored as JSON
    definition      JSONB NOT NULL,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (namespace, name, version)
);

CREATE TABLE rivo_workflow_runs (
    id              BIGSERIAL PRIMARY KEY,
    definition_id   BIGINT NOT NULL REFERENCES rivo_workflow_definitions(id),
    namespace       TEXT NOT NULL DEFAULT 'default',

    state           TEXT NOT NULL DEFAULT 'pending',
    input           JSONB NOT NULL DEFAULT '{}',
    output          JSONB,

    -- Step execution state for replay
    step_states     JSONB NOT NULL DEFAULT '{}',

    -- Error info
    error           TEXT,

    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rivo_workflow_runs_state_check CHECK (
        state IN ('pending', 'running', 'completed', 'failed', 'cancelled')
    )
);

CREATE INDEX rivo_workflow_runs_state_idx ON rivo_workflow_runs (state, created_at);
CREATE INDEX rivo_workflow_runs_definition_idx ON rivo_workflow_runs (definition_id);

-- +migrate Down
DROP TABLE IF EXISTS rivo_workflow_runs;
DROP TABLE IF EXISTS rivo_workflow_definitions;
