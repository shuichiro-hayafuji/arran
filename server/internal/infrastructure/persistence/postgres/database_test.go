package postgres

import (
	"context"
	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
	"testing"
)

func TestBindConvertsQuestionPlaceholders(t *testing.T) {
	got := bind("UPDATE profiles SET monthly_income = ? WHERE id = ?")
	if got != "UPDATE profiles SET monthly_income = $1 WHERE id = $2" {
		t.Fatalf("bind = %q", got)
	}
}

func TestUserQueryUsesOnlyAuthenticatedIdentity(t *testing.T) {
	query := "UPDATE memories SET content = ? WHERE user_id = :user_id AND id = ?"
	got := userQuery(identity.WithUser(context.Background(), 42), query)
	if got != "UPDATE memories SET content = $1 WHERE user_id = 42 AND id = $2" {
		t.Fatalf("scoped query: %s", got)
	}
	got = userQuery(context.Background(), query)
	if got != "UPDATE memories SET content = $1 WHERE user_id = 0 AND id = $2" {
		t.Fatalf("missing identity: %s", got)
	}
}
