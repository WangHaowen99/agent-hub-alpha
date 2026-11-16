ALTER TABLE committed_results ADD COLUMN IF NOT EXISTS identity jsonb;
CREATE UNIQUE INDEX IF NOT EXISTS committed_results_logical_execution ON committed_results(
 scope, (identity->'logical_execution'->>'task_id'), (identity->'logical_execution'->>'graph_version'),
 (identity->'logical_execution'->>'node_path'), (identity->'logical_execution'->>'iteration_path')
) WHERE identity IS NOT NULL;
INSERT INTO schema_migrations(version) VALUES(2) ON CONFLICT DO NOTHING;
