CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS profiles (
  id BIGINT PRIMARY KEY CHECK (id = 1),
  monthly_income BIGINT NOT NULL, current_balance BIGINT NOT NULL,
  monthly_fixed_costs BIGINT NOT NULL, monthly_free_budget BIGINT NOT NULL,
  monthly_savings_goal BIGINT NOT NULL, reduce_categories_json TEXT NOT NULL,
  allowed_categories_json TEXT NOT NULL, long_term_goal TEXT NOT NULL,
  advice_strictness TEXT NOT NULL, updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
  id BIGSERIAL PRIMARY KEY, transaction_date TEXT NOT NULL, description TEXT NOT NULL,
  normalized_merchant TEXT NOT NULL, amount BIGINT NOT NULL, transaction_type TEXT NOT NULL,
  category TEXT NOT NULL, source TEXT NOT NULL, source_account_name TEXT NOT NULL,
  fingerprint TEXT NOT NULL UNIQUE, imported_at TEXT NOT NULL, raw_data_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(transaction_date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_category_date ON transactions(category, transaction_date DESC);

CREATE TABLE IF NOT EXISTS merchant_rules (normalized_merchant TEXT PRIMARY KEY, category TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS consultations (
  id BIGSERIAL PRIMARY KEY, created_at TEXT NOT NULL, user_message TEXT NOT NULL, planned_amount BIGINT,
  inferred_category TEXT NOT NULL, ai_recommendation TEXT NOT NULL, ai_reasoning_summary TEXT NOT NULL,
  current_situation TEXT NOT NULL, alternative TEXT NOT NULL, final_question TEXT NOT NULL,
  budget_snapshot_json TEXT NOT NULL, status TEXT NOT NULL, actual_amount BIGINT,
  user_decision_reason TEXT NOT NULL DEFAULT '', satisfaction_score INTEGER, regret_score INTEGER,
  note TEXT NOT NULL DEFAULT '', response_source TEXT NOT NULL, needs_follow_up BOOLEAN NOT NULL DEFAULT FALSE,
  follow_up_question TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS consultation_messages (
  id BIGSERIAL PRIMARY KEY, consultation_id BIGINT NOT NULL REFERENCES consultations(id) ON DELETE CASCADE,
  role TEXT NOT NULL, content TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS memories (
  id BIGSERIAL PRIMARY KEY, type TEXT NOT NULL, content TEXT NOT NULL, evidence TEXT NOT NULL,
  confidence DOUBLE PRECISION NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS monthly_reviews (
  id BIGSERIAL PRIMARY KEY, month TEXT NOT NULL, created_at TEXT NOT NULL, summary TEXT NOT NULL,
  candidates_json TEXT NOT NULL
);
INSERT INTO schema_migrations(version, applied_at) VALUES (1, CURRENT_TIMESTAMP) ON CONFLICT (version) DO NOTHING;
