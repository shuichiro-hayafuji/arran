CREATE TABLE IF NOT EXISTS chat_requests (
 user_id BIGINT NOT NULL,
 request_id TEXT NOT NULL,
 payload_hash TEXT NOT NULL,
 month TEXT NOT NULL,
 new_consultation BOOLEAN NOT NULL,
 quota_reserved BOOLEAN NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('processing','completed','cancelled','failed')),
 expires_at BIGINT NOT NULL,
 cancel_requested BOOLEAN NOT NULL DEFAULT FALSE,
 error_code TEXT NOT NULL DEFAULT '',
 result_json TEXT,
 PRIMARY KEY(user_id,request_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_requests_active_owner ON chat_requests(user_id) WHERE status='processing';
