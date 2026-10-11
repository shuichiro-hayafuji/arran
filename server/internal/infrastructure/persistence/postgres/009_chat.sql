CREATE TABLE IF NOT EXISTS chat_states (
 user_id BIGINT PRIMARY KEY REFERENCES users(id),
 version BIGINT NOT NULL,
 payload TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS chat_threads (
 consultation_id BIGINT PRIMARY KEY REFERENCES consultations(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL REFERENCES users(id),
 version BIGINT NOT NULL,
 payload TEXT NOT NULL,
 FOREIGN KEY (consultation_id,user_id) REFERENCES consultations(id,user_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chat_threads_user ON chat_threads(user_id);
INSERT INTO schema_migrations(version, applied_at) VALUES (9, CURRENT_TIMESTAMP);
