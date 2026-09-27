package agentadapter

import (
	"testing"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
)

func TestAdviceFactsCopiesRegisteredBalanceFacts(t *testing.T) {
	amount := int64(5000)
	after := int64(-4000)
	got := adviceFacts(domain.AdviceFacts{
		Category:                      "買い物",
		PlannedAmount:                 &amount,
		RegisteredCurrentBalance:      1000,
		RegisteredBalanceAfterPlanned: &after,
		RegisteredBalanceInsufficient: true,
		Verdict:                       "avoid",
	})
	if got.RegisteredCurrentBalance != 1000 || !got.RegisteredBalanceInsufficient {
		t.Fatalf("registered balance facts = %#v", got)
	}
	if got.RegisteredBalanceAfterPlanned == nil || *got.RegisteredBalanceAfterPlanned != -4000 {
		t.Fatalf("registered balance after planned = %#v", got.RegisteredBalanceAfterPlanned)
	}
}
