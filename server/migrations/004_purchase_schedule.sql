CREATE TABLE IF NOT EXISTS chat_purchase_schedule (
 user_id BIGINT NOT NULL,
 purchase_id TEXT NOT NULL,
 consultation_id BIGINT NOT NULL,
 due_at TEXT NOT NULL,
 first_sent BOOLEAN NOT NULL,
 PRIMARY KEY(user_id,purchase_id),
 FOREIGN KEY (consultation_id) REFERENCES chat_threads(consultation_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chat_purchase_due ON chat_purchase_schedule(first_sent,due_at);
