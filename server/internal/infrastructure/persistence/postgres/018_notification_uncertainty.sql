-- Existing history cannot prove that no earlier delivery outcome was ambiguous.
ALTER TABLE admin_notifications ADD COLUMN withdrawal_uncertain BOOLEAN NOT NULL DEFAULT TRUE;
UPDATE admin_notifications SET withdrawal_uncertain=FALSE WHERE status='delivered' AND attempts=1 AND delivered_at IS NOT NULL;
ALTER TABLE admin_notifications ALTER COLUMN withdrawal_uncertain SET DEFAULT FALSE;
INSERT INTO schema_migrations(version, applied_at) VALUES (18, CURRENT_TIMESTAMP);
