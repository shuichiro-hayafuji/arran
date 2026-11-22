CREATE TABLE csv_import_previews (
 user_id BIGINT NOT NULL REFERENCES users(id),
 id TEXT NOT NULL,
 payload TEXT NOT NULL,
 expires_at BIGINT NOT NULL,
 result TEXT,
 PRIMARY KEY(user_id,id)
);
CREATE INDEX idx_csv_preview_expiry ON csv_import_previews(user_id,expires_at);
INSERT INTO schema_migrations(version,applied_at) VALUES(15,CURRENT_TIMESTAMP);
