CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT clock_timestamp());
CREATE TABLE IF NOT EXISTS tasks (
    scope text NOT NULL, id text NOT NULL, document jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(scope,id)
);
CREATE TABLE IF NOT EXISTS commands (
    scope text NOT NULL, principal text NOT NULL, kind text NOT NULL, key text NOT NULL,
    payload_digest text NOT NULL, receipt jsonb NOT NULL,
    PRIMARY KEY(scope,principal,kind,key)
);
CREATE TABLE IF NOT EXISTS run_leases (
    scope text NOT NULL, task_id text NOT NULL, domain text NOT NULL CHECK(domain IN ('graph','delivery')),
    epoch bigint NOT NULL, owner text NOT NULL, control_session text NOT NULL,
    expires_at timestamptz NOT NULL, status text NOT NULL,
    PRIMARY KEY(scope,task_id,domain), FOREIGN KEY(scope,task_id) REFERENCES tasks(scope,id)
);
CREATE TABLE IF NOT EXISTS artifacts (
    scope text NOT NULL, id text NOT NULL, digest text NOT NULL, size bigint NOT NULL,
    schema_version text NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY(scope,id), UNIQUE(scope,digest)
);
CREATE TABLE IF NOT EXISTS committed_results (
    scope text NOT NULL, task_id text NOT NULL, node_id text NOT NULL, input_digest text NOT NULL,
    artifact_id text NOT NULL, artifact_digest text NOT NULL, receipt text NOT NULL,
    output jsonb NOT NULL, valid boolean NOT NULL DEFAULT true,
    PRIMARY KEY(scope,task_id,node_id), FOREIGN KEY(scope,task_id) REFERENCES tasks(scope,id),
    FOREIGN KEY(scope,artifact_id) REFERENCES artifacts(scope,id)
);
CREATE TABLE IF NOT EXISTS artifact_dependencies (
    scope text NOT NULL, task_id text NOT NULL, node_id text NOT NULL, upstream_artifact text NOT NULL,
    PRIMARY KEY(scope,task_id,node_id,upstream_artifact),
    FOREIGN KEY(scope,task_id,node_id) REFERENCES committed_results(scope,task_id,node_id)
);
CREATE TABLE IF NOT EXISTS node_attempts (
    scope text NOT NULL, task_id text NOT NULL, node_id text NOT NULL, attempt integer NOT NULL,
    epoch bigint NOT NULL, input_digest text NOT NULL, status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(scope,task_id,node_id,attempt)
);
CREATE TABLE IF NOT EXISTS remote_calls (
    scope text NOT NULL, id text NOT NULL, task_id text NOT NULL, node_id text NOT NULL,
    attempt integer NOT NULL, input_digest text NOT NULL, message_id text NOT NULL,
    state text NOT NULL, document jsonb NOT NULL, PRIMARY KEY(scope,id),
    UNIQUE(scope,task_id,node_id,attempt)
);
CREATE TABLE IF NOT EXISTS observations (
    scope text NOT NULL, source text NOT NULL, event_id text NOT NULL, call_id text NOT NULL,
    payload_digest text NOT NULL, document jsonb NOT NULL,
    received_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(scope,source,event_id)
);
CREATE TABLE IF NOT EXISTS checks (
    scope text NOT NULL, id text NOT NULL, task_id text NOT NULL, object_digest text NOT NULL,
    checker_version text NOT NULL, outcome text NOT NULL CHECK(outcome IN ('PASS','FAIL','REVISE','HUMAN','UNKNOWN')),
    document jsonb NOT NULL, PRIMARY KEY(scope,id)
);
CREATE TABLE IF NOT EXISTS approvals (
    scope text NOT NULL, id text NOT NULL, task_id text NOT NULL, action text NOT NULL,
    object_digest text NOT NULL, principal text NOT NULL, expires_at timestamptz NOT NULL,
    PRIMARY KEY(scope,id)
);
CREATE TABLE IF NOT EXISTS delivery_intents (
    scope text NOT NULL, id text NOT NULL, task_id text NOT NULL, payload_digest text NOT NULL,
    owner_epoch bigint NOT NULL, state text NOT NULL, receipt jsonb NOT NULL, PRIMARY KEY(scope,id)
);
CREATE TABLE IF NOT EXISTS cases_and_validity (
    scope text NOT NULL, id text NOT NULL, artifact_id text NOT NULL, reason text NOT NULL,
    correction_ref text NOT NULL DEFAULT '', principal text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(scope,id)
);
CREATE TABLE IF NOT EXISTS sessions (
    id_digest text PRIMARY KEY, principal text NOT NULL, csrf_digest text NOT NULL,
    expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_outbox (
    sequence bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, scope text NOT NULL,
    aggregate text NOT NULL, event_id text NOT NULL UNIQUE, kind text NOT NULL, document jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS events_for_task ON audit_outbox(scope,aggregate,sequence);
CREATE TABLE IF NOT EXISTS extraction_reuse (
    scope text NOT NULL, reuse_key text NOT NULL, task_id text NOT NULL, node_id text NOT NULL,
    PRIMARY KEY(scope,reuse_key,task_id,node_id),
    FOREIGN KEY(scope,task_id,node_id) REFERENCES committed_results(scope,task_id,node_id)
);
INSERT INTO schema_migrations(version) VALUES(1) ON CONFLICT DO NOTHING;
