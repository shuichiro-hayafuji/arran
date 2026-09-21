CREATE TABLE admin_notifications (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  month TEXT NOT NULL CHECK (month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
  notification_type TEXT NOT NULL,
  consultation_count BIGINT NOT NULL CHECK (consultation_count >= 0),
  consultation_limit BIGINT NOT NULL CHECK (consultation_limit >= 0),
  status TEXT NOT NULL CHECK (status IN ('sending', 'delivered', 'failed')),
  attempts INTEGER NOT NULL CHECK (attempts > 0),
  last_error TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMPTZ NOT NULL,
  delivered_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, month, notification_type)
);

INSERT INTO schema_migrations(version, applied_at) VALUES (5, CURRENT_TIMESTAMP);
