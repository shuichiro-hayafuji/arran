-- Run as postgres after every successful migrate. Runtime never owns objects
-- or inherits the migrator role. The ledger stays SELECT-only.
BEGIN;
GRANT USAGE ON SCHEMA public TO arran_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO arran_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO arran_runtime;
-- Users are provisioned by an operator, not by the runtime API. Runtime may
-- neither inject evidence identities nor physically remove accounts.
REVOKE INSERT, DELETE ON public.users FROM arran_runtime;
REVOKE INSERT, UPDATE, DELETE ON public.schema_migrations FROM arran_runtime;
GRANT SELECT ON public.schema_migrations TO arran_runtime;
-- Policy text and its history are installed only by a privileged operator.
REVOKE INSERT, UPDATE, DELETE ON public.ai_consent_policy, public.ai_consent_policy_versions FROM arran_runtime;
REVOKE INSERT, DELETE ON public.ai_consent_policy_lock FROM arran_runtime;

DO $$ BEGIN
  IF to_regclass('public.withdrawal_requests') IS NOT NULL THEN
    REVOKE INSERT, UPDATE, DELETE ON public.withdrawal_requests FROM arran_runtime;
    GRANT INSERT (user_id) ON public.withdrawal_requests TO arran_runtime;
    GRANT UPDATE (completed_at) ON public.withdrawal_requests TO arran_runtime;
  END IF;
  IF to_regclass('public.withdrawal_batch_health') IS NOT NULL THEN
    REVOKE INSERT, UPDATE, DELETE ON public.withdrawal_batch_health FROM arran_runtime;
    GRANT UPDATE (checked_at) ON public.withdrawal_batch_health TO arran_runtime;
  END IF;
  IF to_regclass('public.evidence_intents') IS NOT NULL THEN
    -- The outbox protocol is not connected or audited yet.
    REVOKE INSERT, UPDATE, DELETE ON public.evidence_intents FROM arran_runtime;
  END IF;
END $$;
COMMIT;
