CREATE TABLE purchase_csv_decisions (
 user_id BIGINT NOT NULL,
 transaction_id BIGINT NOT NULL,
 purchase_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('same','distinct')),
 ref TEXT NOT NULL,
 csv_snapshot TEXT NOT NULL,
 purchase_revision TEXT NOT NULL,
 prior_inclusion TEXT NOT NULL CHECK(prior_inclusion IN ('new_csv','unknown')),
 balance_resolved BOOLEAN NOT NULL DEFAULT FALSE,
 budget_resolved BOOLEAN NOT NULL DEFAULT FALSE,
 financial_status TEXT NOT NULL CHECK(financial_status IN ('pending','needs_information','applied')),
 recorded_at TEXT NOT NULL,
 FOREIGN KEY(transaction_id,user_id) REFERENCES transactions(id,user_id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,transaction_id,purchase_id),
 UNIQUE(user_id,ref)
);
CREATE UNIQUE INDEX idx_purchase_csv_same_transaction ON purchase_csv_decisions(user_id,transaction_id) WHERE action='same';
CREATE UNIQUE INDEX idx_purchase_csv_same_purchase ON purchase_csv_decisions(user_id,purchase_id) WHERE action='same';
CREATE TABLE purchase_csv_receipts (
 user_id BIGINT NOT NULL,
 ref TEXT NOT NULL,
 purchase_revision TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('needs_information','applied')),
 balance_resolved BOOLEAN NOT NULL,
 budget_resolved BOOLEAN NOT NULL,
 state_version BIGINT NOT NULL,
 recorded_at TEXT NOT NULL,
 PRIMARY KEY(user_id,ref,purchase_revision),
 FOREIGN KEY(user_id,ref) REFERENCES purchase_csv_decisions(user_id,ref) ON DELETE CASCADE
);
INSERT INTO schema_migrations(version,applied_at) VALUES(7,CURRENT_TIMESTAMP);
