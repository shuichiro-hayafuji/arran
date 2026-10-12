package consultation

import (
	"testing"
	"time"
)

func TestDataChangeBaselineCurrentJSTPeriodAndProvenance(t *testing.T) {
	before := time.Date(2026, 10, 31, 14, 59, 59, 0, time.UTC)
	after := before.Add(time.Second)
	changes := []Change{{Key: "current_balance", Value: "100000円", Operation: "set", Kind: "fact", Basis: "user_report", Period: "2026-10"}, {Key: "monthly_free_budget", Value: "30000円", Operation: "set", Kind: "fact", Basis: "user_report", Period: "2026-10"}}
	if err := ValidateDataChangeBaseline(changes, before); err != nil {
		t.Fatal("current JST month rejected", err)
	}
	if err := ValidateDataChangeBaseline(changes, after); err == nil {
		t.Fatal("month crossed before save; old period accepted")
	}
	changes[0].Period = "2026-11-01"
	changes[1].Period = "2026-11"
	if err := ValidateDataChangeBaseline(changes, after); err != nil {
		t.Fatal("current balance day/month budget rejected", err)
	}
	changes[0].Period = ""
	changes[1].Period = ""
	if err := ValidateDataChangeBaseline(changes, after); err != nil {
		t.Fatal("empty legacy period rejected", err)
	}
	changes[0].Kind = "inference"
	if err := ValidateDataChangeBaseline(changes, after); err == nil {
		t.Fatal("inference accepted as latest fact")
	}
}
