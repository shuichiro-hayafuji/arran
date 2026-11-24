package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// This is the complete v17 FK contract touching the locked tables. Validate
// relation OIDs, ordered source/target columns and delete actions, not merely
// table names: an extra cascade between two known tables can delete another
// owner's data.
func verifyWithdrawalSchema(ctx context.Context, tx *sql.Tx) error {
	known := []string{"users", "withdrawal_requests", "schema_migrations", "auth_sessions", "auth_attempts", "user_consultation_limits", "monthly_consultation_usage", "admin_notifications", "llm_usage", "profiles", "transactions", "merchant_rules", "consultations", "consultation_messages", "memories", "monthly_reviews", "chat_states", "chat_threads", "chat_purchase_schedule", "chat_requests", "csv_identity_file_rows", "csv_identity_aliases", "purchase_csv_decisions", "purchase_csv_receipts", "csv_import_batches", "csv_import_evidence", "csv_mutations", "transaction_change_events", "csv_financial_receipts", "csv_financial_items", "csv_import_previews", "ai_user_consents", "ai_consent_inflight"}
	ids := make(map[int64]string, len(known))
	for _, name := range known {
		var oid int64
		var valid bool
		// to_regclass uses the same path as LOCK and DELETE. Every resolved table
		// must be a regular, nonpartitioned table in users' own current schema.
		err := tx.QueryRowContext(ctx, `SELECT c.oid::bigint, c.relnamespace=(SELECT relnamespace FROM pg_class WHERE oid=to_regclass('users')) AND n.nspname=current_schema() AND c.relkind='r' FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.oid=to_regclass($1)`, name).Scan(&oid, &valid)
		if err != nil || !valid {
			return errWithdrawalUnsafe
		}
		if _, exists := ids[oid]; exists {
			return errWithdrawalUnsafe
		}
		ids[oid] = name
	}
	// Unknown relations can contain owned data under any column name. The
	// remaining six v17 tables are global configuration/price/policy tables;
	// they are explicitly named rather than inferred from column names.
	var unknown bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relkind IN ('r','p','m','f') AND NOT (c.oid=ANY($1::oid[])) AND NOT (c.relname=ANY(ARRAY['service_settings','llm_model_prices','monthly_operating_costs','ai_consent_policy_versions','ai_consent_policy_lock','ai_consent_policy'])))`, oidsAsStrings(ids)).Scan(&unknown)
	if err != nil {
		return err
	}
	if unknown {
		return errWithdrawalUnsafe
	}
	// Even an owner or privileged operator must not rely on PostgreSQL's
	// role-dependent RLS bypass: a partially visible users snapshot can look
	// empty and cause a restored user's data to be exposed after cutover.
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_class WHERE oid=ANY($1::oid[]) AND (relrowsecurity OR relforcerowsecurity))`, oidsAsStrings(ids)).Scan(&unknown)
	if err != nil {
		return err
	}
	if unknown {
		return errWithdrawalUnsafe
	}
	// PostgreSQL DELETE on a parent normally includes its inheritance children,
	// including children in other schemas. Reject both directions: table names,
	// relation kind and FK checks do not establish safe inherited ownership.
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_inherits WHERE inhparent=ANY($1::oid[]) OR inhrelid=ANY($1::oid[]))`, oidsAsStrings(ids)).Scan(&unknown)
	if err != nil {
		return err
	}
	if unknown {
		return errWithdrawalUnsafe
	}
	// User-defined triggers and rewrite rules may write or delete other owners'
	// rows even when all foreign keys are known. The sole v17 trigger on these
	// tables is the free-user-limit BEFORE INSERT/UPDATE trigger on users. Its
	// relation, enabled state, event bits, update column, function identity and
	// full source must match the embedded migration. Internal FK triggers remain.
	migration, err := migrationFiles.ReadFile("006_free_user_limit.sql")
	if err != nil {
		return err
	}
	parts := strings.SplitN(string(migration), "$$", 3)
	if len(parts) != 3 {
		return errWithdrawalUnsafe
	}
	var totalTriggers, trustedTriggers int
	err = tx.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE
 t.tgrelid=to_regclass('users') AND t.tgname='users_enforce_free_user_limit'
 AND t.tgtype=23 AND t.tgenabled='O' AND t.tgconstraint=0
 AND t.tgnargs=0 AND t.tgqual IS NULL AND t.tgoldtable IS NULL AND t.tgnewtable IS NULL
 AND t.tgattr::text=(SELECT attnum::text FROM pg_attribute WHERE attrelid=to_regclass('users') AND attname='is_active' AND NOT attisdropped)
 AND p.proname='enforce_free_user_limit' AND p.pronamespace=(SELECT relnamespace FROM pg_class WHERE oid=to_regclass('users'))
 AND p.pronargs=0 AND p.prorettype='trigger'::regtype AND NOT p.proretset AND NOT p.prosecdef
 AND p.provolatile='v' AND p.proparallel='u' AND p.proconfig IS NULL
 AND l.lanname='plpgsql' AND p.prosrc=$2)
 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_language l ON l.oid=p.prolang
 WHERE t.tgrelid=ANY($1::oid[]) AND NOT t.tgisinternal`, oidsAsStrings(ids), parts[1]).Scan(&totalTriggers, &trustedTriggers)
	if err != nil {
		return err
	}
	if totalTriggers != 1 || trustedTriggers != 1 {
		return errWithdrawalUnsafe
	}
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=ANY($1::oid[]))`, oidsAsStrings(ids)).Scan(&unknown)
	if err != nil {
		return err
	}
	if unknown {
		return errWithdrawalUnsafe
	}
	expected := map[string]int{}
	add := func(src, dst, srcCols, dstCols, action string) {
		expected[fmt.Sprintf("%s|%s|%s|%s|%s", src, dst, srcCols, dstCols, action)]++
	}
	for _, name := range []string{"profiles", "transactions", "merchant_rules", "consultations", "consultation_messages", "memories", "monthly_reviews", "chat_states", "chat_threads", "chat_purchase_schedule", "chat_requests", "csv_identity_file_rows", "csv_identity_aliases", "purchase_csv_decisions", "csv_import_batches", "csv_mutations", "csv_import_previews", "ai_user_consents", "ai_consent_inflight", "withdrawal_requests"} {
		add(name, "users", "user_id", "id", "a")
	}
	for _, name := range []string{"auth_sessions", "user_consultation_limits", "monthly_consultation_usage", "admin_notifications", "llm_usage"} {
		add(name, "users", "user_id", "id", "c")
	}
	add("consultation_messages", "consultations", "consultation_id", "id", "c")
	add("consultation_messages", "consultations", "consultation_id,user_id", "id,user_id", "c")
	add("chat_threads", "consultations", "consultation_id", "id", "c")
	add("chat_threads", "consultations", "consultation_id,user_id", "id,user_id", "c")
	add("chat_purchase_schedule", "consultations", "consultation_id,user_id", "id,user_id", "c")
	for _, name := range []string{"csv_identity_file_rows", "csv_identity_aliases", "purchase_csv_decisions"} {
		add(name, "transactions", "transaction_id,user_id", "id,user_id", "c")
	}
	add("csv_identity_aliases", "csv_identity_file_rows", "user_id,file_key,source_scope,account_scope,transaction_type,row_number,transaction_id", "user_id,file_key,source_scope,account_scope,transaction_type,row_number,transaction_id", "c")
	add("purchase_csv_receipts", "purchase_csv_decisions", "user_id,ref", "user_id,ref", "c")
	add("csv_import_evidence", "csv_import_batches", "user_id,batch_id", "user_id,id", "a")
	add("csv_import_evidence", "transactions", "transaction_id,user_id", "id,user_id", "a")
	add("transaction_change_events", "csv_mutations", "user_id,mutation_id", "user_id,id", "a")
	add("csv_financial_receipts", "csv_mutations", "user_id,mutation_id", "user_id,id", "a")
	add("csv_financial_items", "csv_financial_receipts", "user_id,ref", "user_id,ref", "a")
	add("csv_financial_items", "transactions", "transaction_id,user_id", "id,user_id", "a")
	rows, err := tx.QueryContext(ctx, `SELECT k.conrelid::bigint,k.confrelid::bigint,k.confdeltype::text,k.convalidated,
 array_to_string(ARRAY(SELECT a.attname FROM unnest(k.conkey) WITH ORDINALITY AS x(attnum,ord) JOIN pg_attribute a ON a.attrelid=k.conrelid AND a.attnum=x.attnum ORDER BY x.ord),','),
 array_to_string(ARRAY(SELECT a.attname FROM unnest(k.confkey) WITH ORDINALITY AS x(attnum,ord) JOIN pg_attribute a ON a.attrelid=k.confrelid AND a.attnum=x.attnum ORDER BY x.ord),',')
 FROM pg_constraint k WHERE k.contype='f' AND (k.conrelid=ANY($1::oid[]) OR k.confrelid=ANY($1::oid[]))`, oidsAsStrings(ids))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var source, target int64
		var action, sourceCols, targetCols string
		var validated bool
		if err := rows.Scan(&source, &target, &action, &validated, &sourceCols, &targetCols); err != nil {
			return err
		}
		src, okS := ids[source]
		dst, okT := ids[target]
		if !okS || !okT || !validated {
			return errWithdrawalUnsafe
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%s", src, dst, sourceCols, targetCols, action)
		if expected[key] != 1 {
			return errWithdrawalUnsafe
		}
		delete(expected, key)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(expected) != 0 {
		return errWithdrawalUnsafe
	}
	return nil
}

// PostgreSQL accepts a text array literal cast to oid[]; all values originate
// from pg_class and therefore cannot contain user-provided SQL syntax.
func oidsAsStrings(ids map[int64]string) string {
	result := "{"
	for oid := range ids {
		if len(result) > 1 {
			result += ","
		}
		result += fmt.Sprint(oid)
	}
	return result + "}"
}
