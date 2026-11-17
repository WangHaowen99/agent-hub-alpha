CREATE TABLE IF NOT EXISTS selection_records (
 scope text NOT NULL, id text NOT NULL, digest text NOT NULL, document jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(scope,id)
);
INSERT INTO schema_migrations(version) VALUES(3) ON CONFLICT DO NOTHING;
