package app

import (
	"context"
	"testing"

	"github.com/shuichirohayafuji/spendable-today/server/internal/identity"
)

type withdrawalStoreFake struct{ ids []int64 }

func (s *withdrawalStoreFake) WithdrawalRequested(_ context.Context, id int64) (bool, error) {
	s.ids = append(s.ids, id)
	return false, nil
}
func (s *withdrawalStoreFake) RequestWithdrawal(_ context.Context, id int64) error {
	s.ids = append(s.ids, id)
	return nil
}
func TestWithdrawalRequestServiceBindsAuthenticatedOwner(t *testing.T) {
	store := &withdrawalStoreFake{}
	service := withdrawalRequestService{repository: store, enabled: true}
	if err := (withdrawalRequestService{repository: store}).RequestWithdrawal(identity.WithUser(context.Background(), 1)); err == nil {
		t.Fatal("request accepted when the batch is disabled")
	}
	if err := service.RequestWithdrawal(context.Background()); err == nil {
		t.Fatal("missing user accepted")
	}
	if _, err := service.WithdrawalRequested(context.Background()); err == nil {
		t.Fatal("missing user accepted")
	}
	if len(store.ids) != 0 {
		t.Fatal("missing user reached store")
	}
	if err := service.RequestWithdrawal(identity.WithUser(context.Background(), 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.WithdrawalRequested(identity.WithUser(context.Background(), 8)); err != nil {
		t.Fatal(err)
	}
	if len(store.ids) != 2 || store.ids[0] != 3 || store.ids[1] != 8 {
		t.Fatalf("wrong owners %v", store.ids)
	}
}
