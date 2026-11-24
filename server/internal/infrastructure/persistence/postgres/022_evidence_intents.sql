-- A prepared intent is not an externally witnessed event. No API is enabled
-- solely by the existence of this table.
CREATE TABLE evidence_intents (
  user_id BIGINT NOT NULL,
  operation_id TEXT NOT NULL CHECK (operation_id ~ '^[0-9a-f]{32}$'),
  owner_id TEXT NOT NULL CHECK (owner_id ~ '^[0-9a-f]{32}$'),
  ledger_id TEXT NOT NULL CHECK (ledger_id ~ '^[0-9a-f]{32}$'),
  kind TEXT NOT NULL CHECK (kind IN ('withdrawal_requested','withdrawal_completed','consent_granted','consent_withdrawn','send_started','send_completed')),
  related_operation_id TEXT CHECK (related_operation_id IS NULL OR related_operation_id ~ '^[0-9a-f]{32}$'),
  policy_version TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'prepared' CHECK (state IN ('prepared','witnessed')),
  attempt_generation BIGINT,
  attempt_previous TEXT,
  attempt_digest TEXT,
  event_generation BIGINT,
  event_digest TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(user_id,operation_id),
  UNIQUE(operation_id),
  FOREIGN KEY(user_id,owner_id) REFERENCES users(id,evidence_owner_id),
  UNIQUE(user_id,related_operation_id,kind),
  FOREIGN KEY(user_id,related_operation_id) REFERENCES evidence_intents(user_id,operation_id),
  CHECK ((state='prepared' AND event_generation IS NULL AND event_digest IS NULL) OR (state='witnessed' AND event_generation IS NOT NULL AND event_generation > 0 AND event_digest IS NOT NULL AND event_digest ~ '^[0-9a-f]{64}$')),
  CHECK ((attempt_generation IS NULL AND attempt_previous IS NULL AND attempt_digest IS NULL) OR (attempt_generation IS NOT NULL AND attempt_generation > 0 AND attempt_previous IS NOT NULL AND (attempt_previous='' OR attempt_previous ~ '^[0-9a-f]{64}$') AND attempt_digest IS NOT NULL AND attempt_digest ~ '^[0-9a-f]{64}$')),
  CHECK (attempt_generation IS NULL OR (attempt_generation=1 AND attempt_previous='') OR (attempt_generation>1 AND attempt_previous ~ '^[0-9a-f]{64}$')),
  CHECK (state <> 'witnessed' OR (attempt_generation IS NOT NULL AND event_generation=attempt_generation AND event_digest=attempt_digest)),
  CHECK ((kind NOT IN ('consent_granted','consent_withdrawn','send_started','send_completed') OR policy_version <> '')),
  CHECK ((kind = 'send_completed' AND related_operation_id IS NOT NULL) OR (kind <> 'send_completed' AND related_operation_id IS NULL))
);
INSERT INTO schema_migrations(version, applied_at) VALUES (22, CURRENT_TIMESTAMP);
