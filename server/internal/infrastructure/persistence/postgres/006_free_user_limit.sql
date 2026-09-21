INSERT INTO service_settings(key, integer_value)
VALUES ('free_user_limit', 30);

CREATE FUNCTION enforce_free_user_limit() RETURNS trigger AS $$
DECLARE
  configured_limit BIGINT;
  active_users BIGINT;
BEGIN
  IF NEW.is_active AND (TG_OP = 'INSERT' OR (TG_OP = 'UPDATE' AND NOT OLD.is_active)) THEN
    PERFORM pg_advisory_xact_lock(81729422);
    SELECT integer_value INTO configured_limit
    FROM service_settings WHERE key = 'free_user_limit';
    SELECT count(*) INTO active_users FROM users WHERE is_active;
    IF active_users >= configured_limit THEN
      RAISE EXCEPTION 'free user limit reached (%)', configured_limit
        USING ERRCODE = '23514';
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_enforce_free_user_limit
BEFORE INSERT OR UPDATE OF is_active ON users
FOR EACH ROW EXECUTE FUNCTION enforce_free_user_limit();

INSERT INTO schema_migrations(version, applied_at) VALUES (6, CURRENT_TIMESTAMP);
