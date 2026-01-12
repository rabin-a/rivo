-- +migrate Up
CREATE TABLE rivo_audit_log (
    id              BIGSERIAL PRIMARY KEY,
    namespace       TEXT NOT NULL DEFAULT 'default',
    entity_type     TEXT NOT NULL,  -- 'job', 'workflow_run', 'periodic_job'
    entity_id       BIGINT NOT NULL,

    event_type      TEXT NOT NULL,
    event_data      JSONB NOT NULL DEFAULT '{}',

    actor           TEXT,  -- worker ID or API user
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX rivo_audit_log_entity_idx ON rivo_audit_log (entity_type, entity_id, created_at);
CREATE INDEX rivo_audit_log_created_idx ON rivo_audit_log (namespace, created_at);

-- Prevent updates/deletes on audit log
CREATE OR REPLACE FUNCTION rivo_audit_log_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Audit log is immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER rivo_audit_log_no_update
    BEFORE UPDATE OR DELETE ON rivo_audit_log
    FOR EACH ROW EXECUTE FUNCTION rivo_audit_log_immutable();

-- +migrate Down
DROP TRIGGER IF EXISTS rivo_audit_log_no_update ON rivo_audit_log;
DROP FUNCTION IF EXISTS rivo_audit_log_immutable();
DROP TABLE IF EXISTS rivo_audit_log;
