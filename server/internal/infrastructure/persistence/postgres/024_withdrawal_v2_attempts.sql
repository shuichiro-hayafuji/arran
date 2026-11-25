-- v2 can fix a prospective event, but remains unwitnessed and disconnected.
ALTER TABLE evidence_intents DROP CONSTRAINT evidence_version_payload;
ALTER TABLE evidence_intents ADD CONSTRAINT evidence_version_payload CHECK (
  (format_version=1 AND request_time_utc IS NULL AND request_operation_id IS NULL)
  OR (format_version=2 AND kind IN ('withdrawal_requested','withdrawal_completed') AND policy_version='' AND related_operation_id IS NULL AND request_time_utc IS NOT NULL AND request_time_utc <> '' AND state='prepared' AND event_generation IS NULL AND ((kind='withdrawal_requested' AND request_operation_id IS NULL) OR (kind='withdrawal_completed' AND request_operation_id IS NOT NULL AND request_operation_id ~ '^[0-9a-f]{32}$' AND request_operation_id <> operation_id)))
);
INSERT INTO schema_migrations(version, applied_at) VALUES (24, CURRENT_TIMESTAMP);
