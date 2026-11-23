-- No policy is active until its reviewed description is installed explicitly.
CREATE TABLE ai_consent_policy_versions (
 version TEXT PRIMARY KEY CHECK (length(trim(version)) > 0),
 description TEXT NOT NULL CHECK (length(trim(description)) > 0)
);
-- Runtime can lock this row but cannot change the policy itself.
CREATE TABLE ai_consent_policy_lock (
 singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton)
);
INSERT INTO ai_consent_policy_lock(singleton) VALUES(TRUE);
CREATE TABLE ai_consent_policy (
 singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
 version TEXT CHECK (version IS NULL OR length(trim(version)) > 0),
 description TEXT,
 CHECK ((version IS NULL AND description IS NULL) OR
        (version IS NOT NULL AND description IS NOT NULL AND length(trim(description)) > 0))
);
INSERT INTO ai_consent_policy(singleton,version) VALUES(TRUE,NULL);
CREATE TABLE ai_user_consents (
 user_id BIGINT PRIMARY KEY REFERENCES users(id),
 granted_version TEXT,
 withdrawal_pending BOOLEAN NOT NULL DEFAULT FALSE,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE ai_consent_inflight (
 id TEXT PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 version TEXT NOT NULL,
 started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_ai_consent_inflight_user ON ai_consent_inflight(user_id);
-- Policy updates lock the singleton row and cannot pass an admitted send.
-- A new policy never revives a grant recorded for a prior policy.
CREATE FUNCTION ai_consent_change_policy() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
   RAISE EXCEPTION 'AI policy changes require read committed isolation';
 END IF;
 PERFORM singleton FROM ai_consent_policy_lock WHERE singleton=TRUE FOR UPDATE;
 IF NOT FOUND THEN
   RAISE EXCEPTION 'AI policy lock row missing';
 END IF;
 IF TG_OP = 'DELETE' THEN
   RAISE EXCEPTION 'AI policy row cannot be deleted';
 END IF;
 IF NEW.description IS DISTINCT FROM OLD.description AND NEW.version IS NOT DISTINCT FROM OLD.version THEN
   RAISE EXCEPTION 'AI policy description changes require a new version';
 END IF;
 IF NEW.version IS DISTINCT FROM OLD.version THEN
   IF NEW.version IS NOT NULL THEN
     INSERT INTO ai_consent_policy_versions(version,description) VALUES(NEW.version,NEW.description)
       ON CONFLICT(version) DO NOTHING;
     IF EXISTS (SELECT 1 FROM ai_consent_policy_versions WHERE version=NEW.version AND description IS DISTINCT FROM NEW.description) THEN
       RAISE EXCEPTION 'AI policy version was already used for a different description';
     END IF;
   END IF;
   IF EXISTS (SELECT 1 FROM ai_consent_inflight) THEN
     RAISE EXCEPTION 'AI policy change blocked by unresolved sends';
   END IF;
   UPDATE ai_user_consents SET granted_version=NULL, updated_at=CURRENT_TIMESTAMP;
 END IF;
 RETURN NEW;
END;
$$;
CREATE FUNCTION ai_consent_policy_versions_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'AI policy version history cannot be changed';
END;
$$;
CREATE TRIGGER ai_consent_policy_versions_immutable BEFORE UPDATE OR DELETE ON ai_consent_policy_versions
 FOR EACH ROW EXECUTE FUNCTION ai_consent_policy_versions_immutable();
CREATE TRIGGER ai_consent_policy_change BEFORE UPDATE OR DELETE ON ai_consent_policy
 FOR EACH ROW EXECUTE FUNCTION ai_consent_change_policy();
INSERT INTO schema_migrations(version,applied_at) VALUES(16,CURRENT_TIMESTAMP);
