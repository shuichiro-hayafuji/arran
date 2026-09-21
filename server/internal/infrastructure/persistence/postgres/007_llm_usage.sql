CREATE TABLE llm_usage (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  operation TEXT NOT NULL,
  model TEXT NOT NULL,
  input_tokens BIGINT NOT NULL CHECK (input_tokens >= 0),
  cached_input_tokens BIGINT NOT NULL CHECK (cached_input_tokens >= 0),
  output_tokens BIGINT NOT NULL CHECK (output_tokens >= 0),
  reasoning_tokens BIGINT NOT NULL CHECK (reasoning_tokens >= 0),
  total_tokens BIGINT NOT NULL CHECK (total_tokens >= 0),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_llm_usage_user_time ON llm_usage(user_id, occurred_at);

INSERT INTO schema_migrations(version, applied_at) VALUES (7, CURRENT_TIMESTAMP);
