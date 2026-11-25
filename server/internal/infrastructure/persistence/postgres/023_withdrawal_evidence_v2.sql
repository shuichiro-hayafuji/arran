-- v1 rows and their digest remain unchanged. v2 is a disconnected prepared
-- payload only; it is not evidence of a witnessed request or completion.
ALTER TABLE evidence_intents ADD COLUMN format_version SMALLINT NOT NULL DEFAULT 1;
ALTER TABLE evidence_intents ADD COLUMN request_time_utc TEXT;
ALTER TABLE evidence_intents ADD COLUMN request_operation_id TEXT;
ALTER TABLE evidence_intents ADD CONSTRAINT evidence_version_payload CHECK (
  (format_version=1 AND request_time_utc IS NULL AND request_operation_id IS NULL)
  OR (format_version=2 AND kind IN ('withdrawal_requested','withdrawal_completed') AND policy_version='' AND related_operation_id IS NULL AND request_time_utc IS NOT NULL AND request_time_utc <> '' AND state='prepared' AND attempt_generation IS NULL AND event_generation IS NULL AND ((kind='withdrawal_requested' AND request_operation_id IS NULL) OR (kind='withdrawal_completed' AND request_operation_id IS NOT NULL AND request_operation_id ~ '^[0-9a-f]{32}$' AND request_operation_id <> operation_id)))
);
ALTER TABLE evidence_intents ADD CONSTRAINT evidence_v2_request_fk FOREIGN KEY(user_id,request_operation_id) REFERENCES evidence_intents(user_id,operation_id);
CREATE UNIQUE INDEX evidence_v2_one_request_per_owner ON evidence_intents(user_id) WHERE format_version=2 AND kind='withdrawal_requested';
CREATE UNIQUE INDEX evidence_v2_one_completion_per_request ON evidence_intents(user_id,request_operation_id) WHERE format_version=2 AND kind='withdrawal_completed';
INSERT INTO schema_migrations(version, applied_at) VALUES (23, CURRENT_TIMESTAMP);
