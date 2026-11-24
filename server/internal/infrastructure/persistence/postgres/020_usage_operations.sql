ALTER TABLE llm_usage ADD CONSTRAINT llm_usage_owner_id_unique UNIQUE(user_id,id);
CREATE TABLE ai_usage_operations (
  user_id BIGINT NOT NULL REFERENCES users(id),
  operation_id TEXT NOT NULL,
  usage_id BIGINT UNIQUE,
  staged_usage JSONB,
  send_state TEXT NOT NULL DEFAULT 'prepared' CHECK (send_state IN ('prepared','sending')),
  no_usage_reason TEXT CHECK (no_usage_reason IN ('not_sent','no_usage')),
  CHECK (NOT (usage_id IS NOT NULL AND no_usage_reason IS NOT NULL)),
  CHECK (no_usage_reason IS NULL OR staged_usage IS NULL),
  CHECK (usage_id IS NULL OR staged_usage IS NOT NULL),
  CHECK (staged_usage IS NULL OR send_state = 'sending'),
  CHECK (no_usage_reason IS NULL OR (no_usage_reason = 'not_sent' AND send_state = 'prepared')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, operation_id),
  FOREIGN KEY (user_id, usage_id) REFERENCES llm_usage(user_id, id)
);
INSERT INTO schema_migrations(version, applied_at) VALUES (20, CURRENT_TIMESTAMP);
