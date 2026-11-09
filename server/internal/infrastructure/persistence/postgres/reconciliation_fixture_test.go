package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"
)

func openReconciliationRepository(t *testing.T) *Repository {
	t.Helper()
	address, err := url.Parse(os.Getenv("ARRAN_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("pgx", address.String())
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("reconciliation_%d", time.Now().UnixNano())
	if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	params := address.Query()
	params.Set("search_path", schema)
	address.RawQuery = params.Encode()
	repo, err := Open(context.Background(), address.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { repo.Close() })
	for _, id := range []int{701, 702, 711, 712, 713, 714, 715, 901, 902, 3430, 3431, 3434, 3435, 3436, 3437, 3438, 3439, 3440, 920, 921, 922, 923, 924, 925, 926} {
		if _, err = repo.db.Exec("INSERT INTO users(id,username,password_hash) VALUES($1,$2,'disabled-test-only')", id, fmt.Sprintf("fixture%d", id)); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}
func verifyReconciliationIntegration(t *testing.T) {
	r := openReconciliationRepository(t)
	verifyPurchaseCSVDecisionAtomicityAndCount(t, r)
	verifyReconciledCSVDoesNotPostAgainAndKeepsLatestCorrection(t, r)
	verifyCSVFirstUnknownInclusionHoldsFinancialFacts(t, r)
	verifyForgottenCSVDoesNotRequireOrReviveReconciliation(t, r)
	verifyDistinctAllResumesActualWithoutGuessingIdentity(t, r)
	verifyExpenseCountDateProvenanceAndUnpurchasedConflict(t, r)
	verifyNotPurchasedBankConflictCannotDisappearWithBalanceEcho(t, r)
	verifyLegacyEstimatedBasisSurvivesClassificationOnly(t, r)
	verifyInitialSameOnlyCompletesProvenAlreadyAppliedObservation(t, r)
	verifyImportEvidenceCancelRestoreCorrectionAndAudit(t, r)
	verifyBankChangesHoldUnknownInclusionAndSaveBothDimensionsAtomically(t, r)
	verifyPartialBankChangesAllowOnlyExplicitBothUserBaselines(t, r)
	verifyBankCorrectionExcludesPendingTripletAndCancelKeepsSamePurchase(t, r)
	verifyReconciliationMigrationRollbackAndConcurrentRestart(t, r)
	verifyImportManagementMigrationPreservesProvenAndUnknownLegacyAndRollsBack(t, r)
}
