CREATE TABLE withdrawal_requests (
  user_id BIGINT PRIMARY KEY REFERENCES users(id),
  requested_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  reviewed_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);
CREATE TABLE withdrawal_batch_health (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
  checked_at TIMESTAMPTZ NOT NULL DEFAULT 'epoch'
);
INSERT INTO withdrawal_batch_health(singleton) VALUES(TRUE);
INSERT INTO schema_migrations(version, applied_at) VALUES (17, CURRENT_TIMESTAMP);
