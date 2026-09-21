CREATE TABLE llm_model_prices (
  model TEXT NOT NULL,
  effective_from DATE NOT NULL,
  input_usd_per_million NUMERIC(12, 4) NOT NULL CHECK (input_usd_per_million >= 0),
  cached_input_usd_per_million NUMERIC(12, 4) NOT NULL CHECK (cached_input_usd_per_million >= 0),
  output_usd_per_million NUMERIC(12, 4) NOT NULL CHECK (output_usd_per_million >= 0),
  source_url TEXT NOT NULL,
  checked_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (model, effective_from)
);

INSERT INTO llm_model_prices(
  model, effective_from, input_usd_per_million,
  cached_input_usd_per_million, output_usd_per_million,
  source_url, checked_at
) VALUES (
  'gpt-5.6-terra', DATE '2026-09-21', 2.0000, 0.2000, 12.0000,
  'https://developers.openai.com/api/docs/models/gpt-5.6-terra',
  TIMESTAMPTZ '2026-09-21T00:00:00Z'
);

CREATE TABLE monthly_operating_costs (
  month TEXT PRIMARY KEY CHECK (month ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
  infrastructure_cost_usd NUMERIC(12, 4) NOT NULL CHECK (infrastructure_cost_usd >= 0),
  support_case_count INTEGER NOT NULL CHECK (support_case_count >= 0),
  support_minutes INTEGER NOT NULL CHECK (support_minutes >= 0),
  notes TEXT NOT NULL DEFAULT '',
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO schema_migrations(version, applied_at) VALUES (8, CURRENT_TIMESTAMP);
