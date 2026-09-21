CREATE TABLE monthly_consultation_usage (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  month TEXT NOT NULL CHECK (month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
  consultation_count BIGINT NOT NULL CHECK (consultation_count >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, month)
);

INSERT INTO schema_migrations(version, applied_at) VALUES (4, CURRENT_TIMESTAMP);
