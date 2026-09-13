CREATE TABLE IF NOT EXISTS users (
 id BIGSERIAL PRIMARY KEY CHECK (id > 0),
 username TEXT NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9][a-z0-9_.@+-]{2,127}$'),
 password_hash TEXT NOT NULL,
 is_active BOOLEAN NOT NULL DEFAULT TRUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS auth_sessions (
 token_hash TEXT PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 password_hash TEXT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions(expires_at);
CREATE TABLE IF NOT EXISTS auth_attempts (
 key TEXT PRIMARY KEY, attempts INTEGER NOT NULL, window_end TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_attempts_expiry ON auth_attempts(window_end);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions(user_id);
-- Unowned legacy records remain NULL and cannot be accessed through the API.
ALTER TABLE profiles ADD COLUMN user_id BIGINT REFERENCES users(id);
ALTER TABLE profiles DROP CONSTRAINT profiles_pkey;
ALTER TABLE profiles ADD UNIQUE (user_id);
ALTER TABLE transactions ADD COLUMN user_id BIGINT REFERENCES users(id);
CREATE INDEX idx_transactions_user ON transactions(user_id);
ALTER TABLE merchant_rules ADD COLUMN user_id BIGINT REFERENCES users(id);
CREATE INDEX idx_merchant_rules_user ON merchant_rules(user_id);
ALTER TABLE consultations ADD COLUMN user_id BIGINT REFERENCES users(id);
CREATE INDEX idx_consultations_user ON consultations(user_id);
ALTER TABLE consultation_messages ADD COLUMN user_id BIGINT REFERENCES users(id);
CREATE INDEX idx_consultation_messages_user ON consultation_messages(user_id);
ALTER TABLE memories ADD COLUMN user_id BIGINT REFERENCES users(id);
CREATE INDEX idx_memories_user ON memories(user_id);
ALTER TABLE monthly_reviews ADD COLUMN user_id BIGINT REFERENCES users(id);
CREATE INDEX idx_monthly_reviews_user ON monthly_reviews(user_id);
ALTER TABLE transactions DROP CONSTRAINT transactions_fingerprint_key;
ALTER TABLE transactions ADD UNIQUE (user_id, fingerprint);
ALTER TABLE merchant_rules DROP CONSTRAINT merchant_rules_pkey;
ALTER TABLE merchant_rules ADD UNIQUE (user_id, normalized_merchant);
ALTER TABLE consultations ADD UNIQUE (id, user_id);
ALTER TABLE consultation_messages ADD FOREIGN KEY (consultation_id, user_id) REFERENCES consultations(id, user_id) ON DELETE CASCADE;
INSERT INTO schema_migrations(version, applied_at) VALUES (2, CURRENT_TIMESTAMP);
