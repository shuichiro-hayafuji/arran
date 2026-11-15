-- Run as postgres after every successful migrate. Runtime never owns objects
-- or inherits the migrator role. The ledger stays SELECT-only.
GRANT USAGE ON SCHEMA public TO arran_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO arran_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO arran_runtime;
REVOKE INSERT, UPDATE, DELETE ON public.schema_migrations FROM arran_runtime;
GRANT SELECT ON public.schema_migrations TO arran_runtime;
