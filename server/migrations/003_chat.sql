CREATE TABLE IF NOT EXISTS chat_states (
 user_id BIGINT PRIMARY KEY,
 version BIGINT NOT NULL,
 payload TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS chat_threads (
 consultation_id BIGINT PRIMARY KEY REFERENCES consultations(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL,
 version BIGINT NOT NULL,
 payload TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_threads_user ON chat_threads(user_id);

-- The legacy SQLite test schema is single-user. Only explicitly attributed
-- imports may be projected into the multi-user chat context.
CREATE TABLE IF NOT EXISTS chat_transaction_owners (
 transaction_id BIGINT PRIMARY KEY REFERENCES transactions(id) ON DELETE CASCADE,
 user_id BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chat_transaction_owners_user ON chat_transaction_owners(user_id);
