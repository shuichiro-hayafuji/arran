-- This identifier is not a substitute for independently witnessed identity.
-- A restored database from before this migration will assign different IDs and
-- must never be matched to an external ledger by the numeric users.id.
ALTER TABLE users ADD COLUMN evidence_owner_id TEXT NOT NULL DEFAULT replace(gen_random_uuid()::text,'-','');
ALTER TABLE users ADD CONSTRAINT users_evidence_owner_id_format CHECK (evidence_owner_id ~ '^[0-9a-f]{32}$');
ALTER TABLE users ADD CONSTRAINT users_evidence_owner_id_unique UNIQUE (evidence_owner_id);
ALTER TABLE users ADD CONSTRAINT users_id_evidence_owner_unique UNIQUE (id, evidence_owner_id);
CREATE FUNCTION prevent_evidence_owner_id_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.evidence_owner_id IS DISTINCT FROM OLD.evidence_owner_id THEN
    RAISE EXCEPTION 'evidence owner identifier is immutable';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER users_evidence_owner_id_immutable BEFORE UPDATE OF evidence_owner_id ON users
FOR EACH ROW EXECUTE FUNCTION prevent_evidence_owner_id_change();
INSERT INTO schema_migrations(version, applied_at) VALUES (21, CURRENT_TIMESTAMP);
