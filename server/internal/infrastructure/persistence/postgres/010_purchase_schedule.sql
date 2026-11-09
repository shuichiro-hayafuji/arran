CREATE TABLE IF NOT EXISTS chat_purchase_schedule (
 user_id BIGINT NOT NULL REFERENCES users(id),
 purchase_id TEXT NOT NULL,
 consultation_id BIGINT NOT NULL,
 due_at TEXT NOT NULL,
 first_sent BOOLEAN NOT NULL,
 PRIMARY KEY(user_id,purchase_id),
 FOREIGN KEY (consultation_id,user_id) REFERENCES consultations(id,user_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chat_purchase_due ON chat_purchase_schedule(first_sent,due_at);
INSERT INTO schema_migrations(version, applied_at) VALUES (10, CURRENT_TIMESTAMP);
