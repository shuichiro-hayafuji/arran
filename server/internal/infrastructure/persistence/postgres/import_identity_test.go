package postgres

import (
	"context"
	"fmt"
	"github.com/shuichirohayafuji/spendable-today/server/internal/csvimport"
	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"strings"
	"sync"
	"testing"
)

func verifyCSVIdentities(t *testing.T, r *Repository, owner, other context.Context) {
	t.Helper()

	base := "日付,摘要,金額\n2031-01-01,店舗123,2500\n2031-01-01,店舗123,2500\n2031-01-01,店舗123,-2500\n"
	parse := func(data, source, account string, id bool) []domain.Transaction {
		t.Helper()
		mapping := domain.CSVMapping{}
		if id {
			mapping = domain.CSVMapping{DateColumn: "日付", DescriptionColumn: "摘要", AmountColumn: "金額", TransactionIDColumn: "取引ID"}
		}
		p, err := csvimport.Parse([]byte(data), csvimport.Options{Source: source, SourceAccountName: account, Mapping: mapping})
		if err != nil {
			t.Fatal(err)
		}
		return p.Transactions
	}
	check := func(ctx context.Context, items []domain.Transaction, wantNew, wantDuplicate int) {
		t.Helper()
		result, err := r.ImportTransactions(ctx, items)
		if err != nil || result.ImportedCount != wantNew || result.DuplicateCount != wantDuplicate {
			t.Fatalf("import=%+v err=%v want=%d/%d", result, err, wantNew, wantDuplicate)
		}
	}
	snapshot := func(ctx context.Context, id int64) string {
		row, err := r.GetTransaction(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return domain.ImportIdentitySnapshot(row)
	}
	// The same physical CSV must still ask when identity strategy or scope changes.
	for _, scenario := range []string{"add_provider_id", "remove_provider_id", "source_case"} {
		func() {
			data := "日付,摘要,金額,取引ID\n2032-01-01,店舗,2500,id-1\n"
			source := "samefile-" + scenario
			initialID := scenario == "remove_provider_id"
			check(owner, parse(data, source, "口座A", initialID), 1, 0)
			nextSource := source
			if scenario == "source_case" {
				nextSource = strings.ToUpper(source)
			}
			incoming := parse(data, nextSource, "口座A", scenario == "add_provider_id")
			conflicts, err := r.PreviewTransactionIdentities(owner, incoming)
			if err != nil || len(conflicts) != 1 || len(conflicts[0].Candidates) != 1 {
				t.Fatalf("conflicts=%+v err=%v", conflicts, err)
			}
			if _, err = r.ImportTransactions(owner, incoming); err == nil {
				t.Fatal("unconfirmed strategy change added a duplicate")
			}
			check(owner, parse(data, source, "口座A", initialID), 0, 1)
			incoming[0].IdentityAction = "same"
			incoming[0].IdentityTransactionID = conflicts[0].Candidates[0].ID
			incoming[0].IdentityCandidateSnapshot = conflicts[0].CandidateSnapshots[incoming[0].IdentityTransactionID]
			check(owner, incoming, 0, 1)
			// A new strategy must not use distinct to post this physical row twice.
			otherKey := incoming[0]
			otherKey.Fingerprint += ":another-strategy"
			otherKey.IdentityAction = "distinct"
			otherKey.IdentityTransactionID = 0
			if _, err = r.ImportTransactions(owner, []domain.Transaction{otherKey}); err == nil {
				t.Fatal("one file row acquired two targets")
			}
			// Conversely a second physical row cannot claim the same target.
			otherKey.IdentityAction = "same"
			otherKey.IdentityTransactionID = incoming[0].IdentityTransactionID
			otherKey.ImportRow++
			if _, err = r.ImportTransactions(owner, []domain.Transaction{otherKey}); err == nil {
				t.Fatal("two file rows shared one target")
			}
			check(owner, parse(data, nextSource, "口座A", scenario == "add_provider_id"), 0, 1)
			check(owner, parse(data, source, "口座A", initialID), 0, 1)
		}()
	}

	// Provider duplicates must bind each new file, before later strategy changes.
	for _, mode := range []string{"alternate_id", "distinct_without_id"} {
		source := "reused-provider-" + mode
		read := func(data, column, account string) []domain.Transaction {
			p, err := csvimport.Parse([]byte(data), csvimport.Options{Source: source, SourceAccountName: account, Mapping: domain.CSVMapping{DateColumn: "日付", DescriptionColumn: "摘要", AmountColumn: "金額", TransactionIDColumn: column}})
			if err != nil {
				t.Fatal(err)
			}
			return p.Transactions
		}
		dataA := "日付,摘要,金額,取引ID\n2033-01-01,店,2500,id-1\n"
		dataB := "日付,摘要,金額,取引ID,第2ID,第3ID\n2033-01-01,店,2500,id-1,id-alt,id-third\n"
		check(owner, read(dataA, "取引ID", "口座A"), 1, 0)
		check(owner, read(dataB, "取引ID", "口座A"), 0, 1)
		column := ""
		if mode == "alternate_id" {
			column = "第2ID"
		}
		changed := read(dataB, column, "口座A")
		conflicts, err := r.PreviewTransactionIdentities(owner, changed)
		if err != nil || len(conflicts) != 1 || len(conflicts[0].Candidates) != 1 {
			t.Fatalf("%s: conflicts=%+v err=%v", mode, conflicts, err)
		}
		if _, err = r.ImportTransactions(owner, changed); err == nil {
			t.Fatal("reused file with changed strategy added a row without confirmation")
		}
		changed[0].IdentityAction = "distinct"
		if _, err = r.PreviewTransactionIdentities(owner, changed); err == nil {
			t.Fatal("preview allowed a known physical row to be distinct")
		}
		if _, err = r.ImportTransactions(owner, changed); err == nil {
			t.Fatal("commit allowed a known physical row to be distinct")
		}
		changed[0].IdentityAction = "same"
		changed[0].IdentityTransactionID = conflicts[0].Candidates[0].ID
		changed[0].IdentityCandidateSnapshot = conflicts[0].CandidateSnapshots[changed[0].IdentityTransactionID]
		check(owner, changed, 0, 1)
		third := read(dataB, "第3ID", "口座A")
		conflicts, err = r.PreviewTransactionIdentities(owner, third)
		if err != nil || len(conflicts) != 1 {
			t.Fatalf("second strategy change: %+v %v", conflicts, err)
		}
		third[0].IdentityAction = "same"
		third[0].IdentityTransactionID = changed[0].IdentityTransactionID
		third[0].IdentityCandidateSnapshot = conflicts[0].CandidateSnapshots[third[0].IdentityTransactionID]
		check(owner, third, 0, 1)
		for _, col := range []string{"取引ID", column, "第3ID"} {
			check(owner, read(dataB, col, "口座A"), 0, 1)
		}
		check(other, read(dataB, "取引ID", "口座A"), 1, 0)
		check(owner, read(dataB, "取引ID", "口座B"), 1, 0)
	}

	// Identical provider IDs within one CSV cannot collapse physical rows.
	identityCounts := func() string {
		counts := []int{}
		for _, table := range []string{"transactions", "csv_identity_file_rows", "csv_identity_aliases"} {
			var count int
			if err := r.db.QueryRowContext(owner, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
				t.Fatal(err)
			}
			counts = append(counts, count)
		}
		return fmt.Sprint(counts)
	}
	beforeRepeated := identityCounts()
	one := parse("日付,摘要,金額,取引ID\n2034-01-01,店,2500,repeated-id\n", "repeated-provider", "口座A", true)[0]
	valid := parse("日付,摘要,金額,取引ID\n2034-01-02,先行行,100,valid-id\n", "repeated-provider", "口座A", true)[0]
	repeated := one
	repeated.ImportRow++
	for _, alteredFingerprint := range []bool{false, true} {
		if alteredFingerprint {
			repeated.Fingerprint += ":different-key"
		}
		batch := []domain.Transaction{valid, one, repeated}
		if _, err := r.PreviewTransactionIdentities(owner, batch); err == nil {
			t.Fatal("preview accepted repeated provider ID")
		}
		if _, err := r.ImportTransactions(owner, batch); err == nil {
			t.Fatal("direct adapter accepted repeated provider ID")
		}
		if got := identityCounts(); got != beforeRepeated {
			t.Fatalf("repeated ID left partial records/bindings/aliases: %s != %s", got, beforeRepeated)
		}
	}

	rows := parse(base, "identity-base", "口座A", false)
	check(owner, rows, 3, 0)
	check(owner, rows, 0, 3)
	check(other, rows, 3, 0)
	check(owner, parse(base, "identity-base", "口座B", false), 3, 0)
	owned, err := r.ListTransactions(owner, "2031-01", 100)
	if err != nil {
		t.Fatal(err)
	}
	ownA := []domain.Transaction{}
	for _, row := range owned {
		if row.Source == "identity-base" && row.SourceAccountName == "口座A" {
			ownA = append(ownA, row)
		}
	}
	if len(ownA) != 3 {
		t.Fatal("same-day multiplicity/type/account/owner lost")
	}
	expenseIDs := []int64{}
	incomeID := int64(0)
	for _, row := range ownA {
		if row.TransactionType == "income" {
			incomeID = row.ID
		} else {
			expenseIDs = append(expenseIDs, row.ID)
		}
	}
	expanded := base + "2031-01-02,店舗123,2600\n"
	more := parse(expanded, "identity-base", "口座A", false)
	conflicts, err := r.PreviewTransactionIdentities(owner, more)
	if err != nil || len(conflicts) != 3 {
		t.Fatalf("expanded conflicts %d %v", len(conflicts), err)
	}
	if _, err = r.ImportTransactions(owner, more); err == nil {
		t.Fatal("ambiguous file changed finances without confirmation")
	}
	more[0].IdentityAction = "same"
	more[0].IdentityTransactionID = expenseIDs[0]
	more[0].IdentityCandidateSnapshot = snapshot(owner, expenseIDs[0])
	more[1].IdentityAction = "same"
	more[1].IdentityTransactionID = expenseIDs[0]
	more[1].IdentityCandidateSnapshot = snapshot(owner, expenseIDs[0])
	more[2].IdentityAction = "same"
	more[2].IdentityTransactionID = incomeID
	more[2].IdentityCandidateSnapshot = snapshot(owner, incomeID)
	if _, err = r.ImportTransactions(owner, more); err == nil {
		t.Fatal("same candidate assigned twice")
	}
	more[1].IdentityTransactionID = expenseIDs[1]
	more[1].IdentityCandidateSnapshot = snapshot(owner, expenseIDs[1])
	if _, err = r.UpdateTransactionCategory(owner, expenseIDs[0], "学習", false); err != nil {
		t.Fatal(err)
	}
	check(owner, more, 1, 3)
	plainMore := parse(expanded, "identity-base", "口座A", false)
	conflicts, err = r.PreviewTransactionIdentities(owner, plainMore)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("persisted decisions not reused: %v %+v", err, conflicts)
	}
	check(owner, plainMore, 0, 4)
	contradictory := parse(expanded, "identity-base", "口座A", false)
	contradictory[0].IdentityAction = "distinct"
	if _, err = r.ImportTransactions(owner, contradictory); err == nil {
		t.Fatal("concurrent/changed decision silently adopted")
	}
	restored, _ := r.ListTransactions(owner, "2031-01", 100)
	for _, row := range restored {
		if row.ID == expenseIDs[0] && row.Category != "学習" {
			t.Fatal("category correction overwritten")
		}
	}
	reordered := parse("日付,摘要,金額\n2031-01-01,店舗123,-2500\n2031-01-01,店舗123,2500\n2031-01-01,店舗123,2500\n", "identity-base", "口座A", false)
	conflicts, err = r.PreviewTransactionIdentities(owner, reordered)
	if err != nil || len(conflicts) != 3 {
		t.Fatal("reordered file automatically merged/inserted")
	}
	removed := parse("日付,摘要,金額\n2031-01-01,店舗123,2500\n2031-01-01,店舗123,-2500\n", "identity-base", "口座A", false)
	if _, err = r.ImportTransactions(owner, removed); err == nil {
		t.Fatal("missing identical occurrence inferred automatically")
	}
	removed[0].IdentityAction = "same"
	removed[0].IdentityTransactionID = expenseIDs[0]
	removed[0].IdentityCandidateSnapshot = snapshot(owner, expenseIDs[0])
	removed[1].IdentityAction = "same"
	removed[1].IdentityTransactionID = incomeID
	removed[1].IdentityCandidateSnapshot = snapshot(owner, incomeID)
	check(owner, removed, 0, 2)
	// Unknown old records require an explicit match; one legacy row is reused once.
	legacyRows := parse(base, "identity-legacy", "口座A", false)
	legacy := legacyRows[0]
	legacy.IdentityVersion = 1
	legacy.ImportFileKey = ""
	legacy.Fingerprint = csvimport.Fingerprint(legacy.TransactionDate, legacy.Amount, legacy.Description, legacy.Source, legacy.SourceAccountName)
	legacy.Category = "健康"
	check(owner, []domain.Transaction{legacy}, 1, 0)
	current, _ := r.ListTransactions(owner, "2031-01", 100)
	legacyID := int64(0)
	for _, row := range current {
		if row.Source == "identity-legacy" {
			legacyID = row.ID
		}
	}
	conflicts, err = r.PreviewTransactionIdentities(owner, legacyRows)
	if err != nil || len(conflicts) != 2 {
		t.Fatalf("legacy unknown match was automatic %v %d", err, len(conflicts))
	}
	if _, err = r.ImportTransactions(owner, legacyRows); err == nil {
		t.Fatal("legacy automatically migrated")
	}
	legacyRows[0].IdentityAction = "same"
	legacyRows[0].IdentityTransactionID = legacyID
	legacyRows[0].IdentityCandidateSnapshot = snapshot(owner, legacyID)
	partial := append([]domain.Transaction(nil), legacyRows...)
	if _, err = r.ImportTransactions(owner, partial); err == nil {
		t.Fatal("earlier legacy match removed later unanswered confirmation")
	}
	legacyRows[1].IdentityAction = "distinct"
	check(owner, legacyRows, 2, 1)
	check(owner, parse(base, "identity-legacy", "口座A", false), 0, 3)
	current, _ = r.ListTransactions(owner, "2031-01", 100)
	found := false
	for _, row := range current {
		if row.ID == legacyID {
			found = true
			if row.Category != "健康" {
				t.Fatal("legacy correction lost")
			}
		}
	}
	if !found {
		t.Fatal("legacy row ID lost")
	}
	var oldFingerprint string
	if err = r.db.QueryRowContext(owner, "SELECT legacy_fingerprint FROM transactions WHERE id=?", legacyID).Scan(&oldFingerprint); err != nil || oldFingerprint != legacy.Fingerprint {
		t.Fatal("legacy fingerprint trace lost", err)
	}
	// Income and expense collided under v1; an income legacy record stays income.
	incomeLegacy := parse(base, "identity-old-income", "口座A", false)
	oldIncome := incomeLegacy[2]
	oldIncome.IdentityVersion = 1
	oldIncome.ImportFileKey = ""
	oldIncome.Fingerprint = csvimport.Fingerprint(oldIncome.TransactionDate, oldIncome.Amount, oldIncome.Description, oldIncome.Source, oldIncome.SourceAccountName)
	check(owner, []domain.Transaction{oldIncome}, 1, 0)
	current, _ = r.ListTransactions(owner, "2031-01", 100)
	oldIncomeID := int64(0)
	for _, row := range current {
		if row.Source == "identity-old-income" {
			oldIncomeID = row.ID
		}
	}
	incomeLegacy[2].IdentityAction = "same"
	incomeLegacy[2].IdentityTransactionID = oldIncomeID
	incomeLegacy[2].IdentityCandidateSnapshot = snapshot(owner, oldIncomeID)
	check(owner, incomeLegacy, 2, 1)
	// Valid provider IDs establish identity across changed files, scoped by source/account/type/owner.
	ids := "日付,摘要,金額,取引ID\n2031-01-01,店舗123,2500,id-1\n2031-01-01,店舗123,2500,id-2\n"
	provider := parse(ids, "bank-A", "口座A", true)
	check(owner, provider, 2, 0)
	idsMore := ids + "2031-01-01,店舗123,2500,id-3\n"
	check(owner, parse(idsMore, "bank-A", "口座A", true), 1, 2)
	check(owner, parse(ids, "bank-B", "口座A", true), 2, 0)
	check(owner, parse(ids, "bank-A", "口座B", true), 2, 0)
	check(other, provider, 2, 0)
	invalid := parse("日付,摘要,金額,取引ID\n2031-01-01,新明細,999,id-new\n2031-01-01,店舗123,2600,id-1\n", "bank-A", "口座A", true)
	if _, err = r.ImportTransactions(owner, invalid); err == nil {
		t.Fatal("same provider ID changed amount accepted")
	}
	current, _ = r.ListTransactions(owner, "2031-01", 200)
	for _, row := range current {
		if row.Description == "新明細" {
			t.Fatal("failed batch partially saved")
		}
	}
	// Same-source/account old-row selection cannot reference another owner.
	wrong := parse(base+"\n", "identity-base", "口座A", false)
	wrong[0].IdentityAction = "same"
	foreign, _ := r.ListTransactions(other, "2031-01", 100)
	foreignID := int64(0)
	for _, row := range foreign {
		if row.Source == "identity-base" {
			foreignID = row.ID
			break
		}
	}
	wrong[0].IdentityTransactionID = foreignID
	if _, err = r.ImportTransactions(owner, wrong); err == nil {
		t.Fatal("cross-owner candidate adopted")
	}

	variantBase := parse("日付,摘要,金額\n2031-01-01,SHOP 123,2500\n", "identity-variant", "口座A", false)
	check(owner, variantBase, 1, 0)
	current, _ = r.ListTransactions(owner, "2031-01", 200)
	variantID := int64(0)
	for _, row := range current {
		if row.Source == "identity-variant" {
			variantID = row.ID
		}
	}
	variant := parse("日付,摘要,金額\n2031-01-01,shop 123,2500\n2031-01-01,別の店舗,2500\n", "identity-variant", "口座A", false)
	conflicts, err = r.PreviewTransactionIdentities(owner, variant)
	if err != nil || len(conflicts) != 2 {
		t.Fatal("description variation silently added")
	}
	variant[0].IdentityAction = "same"
	variant[0].IdentityTransactionID = variantID
	variant[0].IdentityCandidateSnapshot = snapshot(owner, variantID)
	variant[1].IdentityAction = "distinct"
	check(owner, variant, 1, 1)
	check(owner, parse("日付,摘要,金額\n2031-01-01,shop 123,2500\n2031-01-01,別の店舗,2500\n", "identity-variant", "口座A", false), 0, 2)
	old, err := r.GetTransaction(owner, variantID)
	if err != nil || old.Description != "SHOP 123" {
		t.Fatal("explicit match rewrote original description")
	}
	stale := parse("日付,摘要,金額\n2031-01-01,SHOP 123,2500\n2031-01-03,新規,999\n", "identity-variant", "口座A", false)
	stale[0].IdentityAction = "same"
	stale[0].IdentityTransactionID = variantID
	stale[0].IdentityCandidateSnapshot = snapshot(owner, variantID)
	if _, err = r.db.ExecContext(owner, "UPDATE transactions SET description=? WHERE id=?", "changed", variantID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ImportTransactions(owner, stale); err == nil {
		t.Fatal("stale candidate snapshot accepted")
	}
	if _, err = r.db.ExecContext(owner, "UPDATE transactions SET description=? WHERE id=?", "SHOP 123", variantID); err != nil {
		t.Fatal(err)
	}
	concurrent := parse(base, "identity-concurrent", "口座A", false)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := r.ImportTransactions(owner, concurrent); errs <- err }()
	}
	wg.Wait()
	close(errs)
	failures := 0
	for err := range errs {
		if err != nil {
			failures++
		}
	}
	if failures == 2 {
		t.Fatal("neither concurrent import succeeded")
	}
	check(owner, concurrent, 0, 3)
	current, _ = r.ListTransactions(owner, "2031-01", 200)
	count := 0
	for _, row := range current {
		if row.Source == "identity-concurrent" {
			count++
		}
	}
	if count != 3 {
		t.Fatal(fmt.Sprintf("concurrent import count=%d", count))
	}
}
