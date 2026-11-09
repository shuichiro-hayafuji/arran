ALTER TABLE transactions ADD COLUMN identity_version INTEGER NOT NULL DEFAULT 1 CHECK(identity_version IN (1,2));
ALTER TABLE transactions ADD COLUMN legacy_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN provider_transaction_id TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN import_file_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_transactions_identity_content ON transactions(user_id,lower(source),lower(source_account_name),transaction_date,transaction_type,amount);
ALTER TABLE transactions ADD UNIQUE(id,user_id);
CREATE TABLE csv_identity_file_rows (
 user_id BIGINT NOT NULL REFERENCES users(id),
 file_key TEXT NOT NULL,
 source_scope TEXT NOT NULL,
 account_scope TEXT NOT NULL,
 transaction_type TEXT NOT NULL,
 row_number INTEGER NOT NULL CHECK(row_number >= 2),
 transaction_id BIGINT NOT NULL,
 FOREIGN KEY(transaction_id,user_id) REFERENCES transactions(id,user_id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,file_key,source_scope,account_scope,transaction_type,row_number),
 UNIQUE(user_id,file_key,source_scope,account_scope,transaction_type,transaction_id),
 UNIQUE(user_id,file_key,source_scope,account_scope,transaction_type,row_number,transaction_id)
);
CREATE TABLE csv_identity_aliases (
 user_id BIGINT NOT NULL REFERENCES users(id),
 row_key TEXT NOT NULL,
 row_number INTEGER NOT NULL CHECK(row_number >= 2),
 incoming_snapshot TEXT NOT NULL,
 candidate_snapshot TEXT NOT NULL,
 decision_action TEXT NOT NULL CHECK(decision_action IN ('same','distinct')),
 file_key TEXT NOT NULL,
 source_scope TEXT NOT NULL,
 account_scope TEXT NOT NULL,
 transaction_type TEXT NOT NULL,
 transaction_id BIGINT NOT NULL,
 FOREIGN KEY(transaction_id,user_id) REFERENCES transactions(id,user_id) ON DELETE CASCADE,
 FOREIGN KEY(user_id,file_key,source_scope,account_scope,transaction_type,row_number,transaction_id) REFERENCES csv_identity_file_rows(user_id,file_key,source_scope,account_scope,transaction_type,row_number,transaction_id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,row_key)
);
INSERT INTO schema_migrations(version,applied_at) VALUES(12,CURRENT_TIMESTAMP);
