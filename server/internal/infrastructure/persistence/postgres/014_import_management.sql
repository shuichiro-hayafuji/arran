ALTER TABLE transactions ADD COLUMN effective_date TEXT;
ALTER TABLE transactions ADD COLUMN effective_amount BIGINT CHECK(effective_amount>=0);
ALTER TABLE transactions ADD COLUMN effective_category TEXT;
ALTER TABLE transactions ADD COLUMN revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE transactions ADD COLUMN legacy_retained BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE transactions SET legacy_retained=TRUE WHERE NOT EXISTS(SELECT 1 FROM csv_identity_file_rows f WHERE f.transaction_id=transactions.id AND f.user_id=transactions.user_id);
CREATE TABLE csv_import_batches (
 id TEXT NOT NULL,user_id BIGINT NOT NULL REFERENCES users(id),commit_key TEXT NOT NULL,file_key TEXT NOT NULL,source_scope TEXT NOT NULL,account_scope TEXT NOT NULL,
 metadata TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN ('active','cancelled')),version BIGINT NOT NULL DEFAULT 1,
 recorded_at TEXT NOT NULL,result TEXT NOT NULL,legacy BOOLEAN NOT NULL DEFAULT FALSE,
 PRIMARY KEY(user_id,id),UNIQUE(user_id,commit_key)
);
CREATE TABLE csv_import_evidence (
 user_id BIGINT NOT NULL,batch_id TEXT NOT NULL,row_number INTEGER NOT NULL,transaction_type TEXT NOT NULL,transaction_id BIGINT NOT NULL,observation TEXT NOT NULL,
 PRIMARY KEY(user_id,batch_id,row_number,transaction_type),FOREIGN KEY(user_id,batch_id) REFERENCES csv_import_batches(user_id,id),FOREIGN KEY(transaction_id,user_id) REFERENCES transactions(id,user_id)
);
CREATE INDEX idx_csv_evidence_transaction ON csv_import_evidence(user_id,transaction_id);
CREATE TABLE csv_mutations (
 user_id BIGINT NOT NULL REFERENCES users(id),id TEXT NOT NULL,payload_digest TEXT NOT NULL,kind TEXT NOT NULL,target TEXT NOT NULL,recorded_at TEXT NOT NULL,result TEXT NOT NULL,
 PRIMARY KEY(user_id,id)
);
CREATE TABLE transaction_change_events (
 user_id BIGINT NOT NULL,id TEXT NOT NULL,mutation_id TEXT NOT NULL,kind TEXT NOT NULL,target TEXT NOT NULL,recorded_at TEXT NOT NULL,before_state TEXT NOT NULL,after_state TEXT NOT NULL,
 PRIMARY KEY(user_id,id),FOREIGN KEY(user_id,mutation_id) REFERENCES csv_mutations(user_id,id)
);
CREATE TABLE csv_financial_receipts (
 user_id BIGINT NOT NULL,ref TEXT NOT NULL,mutation_id TEXT NOT NULL,operation TEXT NOT NULL,before_data TEXT NOT NULL,after_data TEXT NOT NULL,
 included_count INTEGER NOT NULL,omitted_count INTEGER NOT NULL,status TEXT NOT NULL CHECK(status IN ('pending','needs_information','applied','baseline_adopted')),
 balance_resolved BOOLEAN NOT NULL DEFAULT FALSE,budget_resolved BOOLEAN NOT NULL DEFAULT FALSE,recorded_at TEXT NOT NULL,state_version BIGINT NOT NULL,
 PRIMARY KEY(user_id,ref),FOREIGN KEY(user_id,mutation_id) REFERENCES csv_mutations(user_id,id)
);
CREATE TABLE csv_financial_items (
 user_id BIGINT NOT NULL,ref TEXT NOT NULL,transaction_id BIGINT NOT NULL,
 PRIMARY KEY(user_id,ref,transaction_id),FOREIGN KEY(user_id,ref) REFERENCES csv_financial_receipts(user_id,ref),FOREIGN KEY(transaction_id,user_id) REFERENCES transactions(id,user_id)
);
CREATE VIEW effective_transactions AS SELECT t.id,t.user_id,COALESCE(t.effective_date,t.transaction_date) AS transaction_date,t.description,t.normalized_merchant,COALESCE(t.effective_amount,t.amount) AS amount,t.transaction_type,COALESCE(t.effective_category,t.category) AS category,t.source,t.source_account_name,t.fingerprint,t.imported_at,t.raw_data_json,t.identity_version,t.legacy_fingerprint,t.provider_transaction_id,t.import_file_key,t.revision FROM transactions t WHERE t.legacy_retained=TRUE OR EXISTS(SELECT 1 FROM csv_import_evidence e JOIN csv_import_batches b ON b.user_id=e.user_id AND b.id=e.batch_id WHERE e.user_id=t.user_id AND e.transaction_id=t.id AND b.status='active');
INSERT INTO csv_import_batches(id,user_id,commit_key,file_key,source_scope,account_scope,metadata,status,recorded_at,result,legacy)
SELECT 'legacy:'||md5(json_build_array(user_id,file_key,source_scope,account_scope)::text),user_id,'legacy:'||md5(json_build_array(user_id,file_key,source_scope,account_scope)::text),file_key,source_scope,account_scope,'{}','active','','{}',TRUE FROM csv_identity_file_rows WHERE file_key!='' GROUP BY user_id,file_key,source_scope,account_scope;
INSERT INTO csv_import_evidence(user_id,batch_id,row_number,transaction_type,transaction_id,observation) SELECT f.user_id,b.id,f.row_number,f.transaction_type,f.transaction_id,'{}' FROM csv_identity_file_rows f JOIN csv_import_batches b ON b.user_id=f.user_id AND b.file_key=f.file_key AND b.source_scope=f.source_scope AND b.account_scope=f.account_scope WHERE f.file_key!='';
UPDATE transactions SET legacy_retained=TRUE WHERE NOT EXISTS(SELECT 1 FROM csv_import_evidence e WHERE e.user_id=transactions.user_id AND e.transaction_id=transactions.id);
INSERT INTO schema_migrations(version,applied_at) VALUES(14,CURRENT_TIMESTAMP);
