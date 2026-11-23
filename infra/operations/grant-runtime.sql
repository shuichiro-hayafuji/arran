-- Run as postgres after every successful migrate. Runtime never owns objects
-- or inherits the migrator role. The ledger stays SELECT-only.
BEGIN;
GRANT USAGE ON SCHEMA public TO arran_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO arran_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO arran_runtime;
REVOKE INSERT, UPDATE, DELETE ON public.schema_migrations FROM arran_runtime;
GRANT SELECT ON public.schema_migrations TO arran_runtime;
-- Policy text and its history are installed only by a privileged operator.
REVOKE INSERT, UPDATE, DELETE ON public.ai_consent_policy, public.ai_consent_policy_versions FROM arran_runtime;
REVOKE INSERT, DELETE ON public.ai_consent_policy_lock FROM arran_runtime;

DO $$ BEGIN
  IF to_regclass('public.withdrawal_requests') IS NOT NULL THEN
    REVOKE INSERT, UPDATE, DELETE ON public.withdrawal_requests FROM arran_runtime;
    GRANT INSERT (user_id) ON public.withdrawal_requests TO arran_runtime;
  END IF;
END $$;
COMMIT;
