-- Workflow step execution logs
CREATE TABLE IF NOT EXISTS rivo_workflow_step_logs (
    id BIGSERIAL PRIMARY KEY,
    workflow_run_id BIGINT NOT NULL REFERENCES rivo_workflow_runs(id) ON DELETE CASCADE,
    step_id TEXT NOT NULL,
    attempt INTEGER NOT NULL DEFAULT 1,
    worker_id TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,
    input JSONB NOT NULL DEFAULT '{}',
    output JSONB,
    status TEXT NOT NULL DEFAULT 'running', -- running, completed, failed
    error TEXT,
    error_stack TEXT,
    logs JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS rivo_workflow_step_logs_run_idx ON rivo_workflow_step_logs (workflow_run_id);
CREATE INDEX IF NOT EXISTS rivo_workflow_step_logs_step_idx ON rivo_workflow_step_logs (workflow_run_id, step_id);
