CREATE TABLE sync_consultation_reservations (
  id TEXT NOT NULL,
  user_id BIGINT NOT NULL REFERENCES users(id),
  month TEXT NOT NULL,
  consultation_count BIGINT NOT NULL,
  consultation_limit BIGINT NOT NULL,
  quota_reserved BOOLEAN NOT NULL DEFAULT TRUE,
  state TEXT NOT NULL CHECK (state IN ('active', 'completed', 'released')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, id)
);
CREATE INDEX sync_consultation_reservations_active_user ON sync_consultation_reservations(user_id) WHERE state='active';
INSERT INTO schema_migrations(version, applied_at) VALUES (19, CURRENT_TIMESTAMP);
