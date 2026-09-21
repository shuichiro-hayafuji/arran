CREATE TABLE service_settings (
  key TEXT PRIMARY KEY,
  integer_value BIGINT NOT NULL CHECK (integer_value >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO service_settings(key, integer_value)
VALUES ('default_monthly_consultation_limit', 30);

CREATE TABLE user_consultation_limits (
  user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  monthly_limit BIGINT NOT NULL CHECK (monthly_limit >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_migrations(version, applied_at) VALUES (3, CURRENT_TIMESTAMP);
