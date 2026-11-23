CREATE TABLE withdrawal_requests (
  user_id BIGINT PRIMARY KEY REFERENCES users(id),
  requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO schema_migrations(version, applied_at) VALUES (17, CURRENT_TIMESTAMP);
