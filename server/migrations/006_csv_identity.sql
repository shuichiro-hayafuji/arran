-- Executed once under BEGIN IMMEDIATE with foreign_keys disabled on this connection.
CREATE TABLE transactions_v2 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id BIGINT,
 transaction_date TEXT NOT NULL,
 description TEXT NOT NULL,
 normalized_merchant TEXT NOT NULL,
 amount INTEGER NOT NULL,
 transaction_type TEXT NOT NULL,
 category TEXT NOT NULL,
 source TEXT NOT NULL,
 source_account_name TEXT NOT NULL,
 fingerprint TEXT NOT NULL,
 imported_at TEXT NOT NULL,
 raw_data_json TEXT NOT NULL,
 identity_version INTEGER NOT NULL DEFAULT 1 CHECK(identity_version IN (1,2)),
 legacy_fingerprint TEXT NOT NULL DEFAULT '',
 provider_transaction_id TEXT NOT NULL DEFAULT '',
 import_file_key TEXT NOT NULL DEFAULT '',
 UNIQUE(user_id,fingerprint),
 UNIQUE(id,user_id)
);
INSERT INTO transactions_v2(id,user_id,transaction_date,description,normalized_merchant,amount,transaction_type,category,source,source_account_name,fingerprint,imported_at,raw_data_json)
 SELECT t.id,o.user_id,t.transaction_date,t.description,t.normalized_merchant,t.amount,t.transaction_type,t.category,t.source,t.source_account_name,t.fingerprint,t.imported_at,t.raw_data_json FROM transactions t LEFT JOIN chat_transaction_owners o ON o.transaction_id=t.id;
UPDATE sqlite_sequence SET seq=MAX(seq,COALESCE((SELECT seq FROM sqlite_sequence WHERE name='transactions'),0)) WHERE name='transactions_v2';
DROP TABLE transactions;
ALTER TABLE transactions_v2 RENAME TO transactions;
CREATE INDEX idx_transactions_date ON transactions(transaction_date DESC);
CREATE INDEX idx_transactions_category_date ON transactions(category,transaction_date DESC);
CREATE INDEX idx_transactions_identity_content ON transactions(user_id,lower(source),lower(source_account_name),transaction_date,transaction_type,amount);
CREATE TABLE csv_identity_file_rows (
 user_id BIGINT NOT NULL,
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
 user_id BIGINT NOT NULL,
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
INSERT INTO schema_migrations(version,applied_at) VALUES(6,CURRENT_TIMESTAMP);
