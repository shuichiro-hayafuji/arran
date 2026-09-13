PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS profiles (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  monthly_income INTEGER NOT NULL,
  current_balance INTEGER NOT NULL,
  monthly_fixed_costs INTEGER NOT NULL,
  monthly_free_budget INTEGER NOT NULL,
  monthly_savings_goal INTEGER NOT NULL,
  reduce_categories_json TEXT NOT NULL,
  allowed_categories_json TEXT NOT NULL,
  long_term_goal TEXT NOT NULL,
  advice_strictness TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  transaction_date TEXT NOT NULL,
  description TEXT NOT NULL,
  normalized_merchant TEXT NOT NULL,
  amount INTEGER NOT NULL,
  transaction_type TEXT NOT NULL,
  category TEXT NOT NULL,
  source TEXT NOT NULL,
  source_account_name TEXT NOT NULL,
  fingerprint TEXT NOT NULL UNIQUE,
  imported_at TEXT NOT NULL,
  raw_data_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(transaction_date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_category_date ON transactions(category, transaction_date DESC);

CREATE TABLE IF NOT EXISTS merchant_rules (
  normalized_merchant TEXT PRIMARY KEY,
  category TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS consultations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  created_at TEXT NOT NULL,
  user_message TEXT NOT NULL,
  planned_amount INTEGER,
  inferred_category TEXT NOT NULL,
  ai_recommendation TEXT NOT NULL,
  ai_reasoning_summary TEXT NOT NULL,
  current_situation TEXT NOT NULL,
  alternative TEXT NOT NULL,
  final_question TEXT NOT NULL,
  budget_snapshot_json TEXT NOT NULL,
  status TEXT NOT NULL,
  actual_amount INTEGER,
  user_decision_reason TEXT NOT NULL DEFAULT '',
  satisfaction_score INTEGER,
  regret_score INTEGER,
  note TEXT NOT NULL DEFAULT '',
  response_source TEXT NOT NULL,
  needs_follow_up INTEGER NOT NULL DEFAULT 0,
  follow_up_question TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS consultation_messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  consultation_id INTEGER NOT NULL REFERENCES consultations(id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS memories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL,
  content TEXT NOT NULL,
  evidence TEXT NOT NULL,
  confidence REAL NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS monthly_reviews (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  month TEXT NOT NULL,
  created_at TEXT NOT NULL,
  summary TEXT NOT NULL,
  candidates_json TEXT NOT NULL
);

INSERT OR IGNORE INTO schema_migrations(version, applied_at)
VALUES (1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
