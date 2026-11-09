package csvimport

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

func TestRequestedBankColumnsAndYearFirstDates(t *testing.T) {
	for _, date := range []string{"2026/7/2", "2026/07/2", "2026/7/02", "2026-7-2", "2026.7.2", "2026年7月2日"} {
		t.Run(date, func(t *testing.T) {
			data := []byte("年月日,お取り扱い内容,お引出し,お預入れ,残高\n" + date + ",合成出金,1200,,9999999\n" + date + ",合成入金,,3500,10003499\n")
			result, err := Parse(data, Options{Now: fixedNow()})
			if err != nil || result.NeedsMapping || len(result.Transactions) != 2 || len(result.Warnings) != 0 {
				t.Fatalf("bank columns/date not supported: %+v %v", result, err)
			}
			if result.Mapping.AmountColumn != "" || result.Mapping.DebitColumn != "お引出し" || result.Mapping.CreditColumn != "お預入れ" {
				t.Fatal(result.Mapping)
			}
			debit, credit := result.Transactions[0], result.Transactions[1]
			if debit.TransactionDate != "2026-07-02" || debit.Amount != 1200 || debit.TransactionType != "expense" || credit.Amount != 3500 || credit.TransactionType != "income" {
				t.Fatal(debit, credit)
			}
			if debit.RawData["残高"] != "9999999" {
				t.Fatal("original balance field lost")
			}
		})
	}
}
func TestBalanceColumnCannotSupplyOrOverrideTransactionAmount(t *testing.T) {
	only, err := Parse([]byte("年月日,お取り扱い内容,残高\n2026/7/2,合成記録,900000\n"), Options{})
	if err != nil || !only.NeedsMapping || len(only.Transactions) != 0 || only.Mapping.AmountColumn != "" {
		t.Fatal("balance was inferred as transaction amount", only, err)
	}
	proper, err := Parse([]byte("年月日,お取り扱い内容,残高,金額\n2026/7/2,合成記録,not-an-amount,1200\n"), Options{})
	if err != nil || proper.NeedsMapping || len(proper.Transactions) != 1 || proper.Transactions[0].Amount != 1200 {
		t.Fatal("balance overrode amount", proper, err)
	}
}
func TestInvalidYearFirstDateWarnsWithoutFabricatingTransaction(t *testing.T) {
	result, err := Parse([]byte("年月日,お取り扱い内容,お引出し,お預入れ\n2026/2/30,不正な合成日付,1200,\n2026/7/2,正常な合成日付,1200,\n"), Options{})
	if err != nil || result.NeedsMapping || len(result.Transactions) != 1 || len(result.Warnings) != 1 || result.Transactions[0].TransactionDate != "2026-07-02" {
		t.Fatal(result, err)
	}
}

// This committed byte fixture is synthetic, not an authentic bank export.
func TestSyntheticCP932BankBytesKeepIncomeExpenseAndExtensions(t *testing.T) {
	encoded := readSample(t, "synthetic_bank_cp932.csv")
	source := readSample(t, "synthetic_bank_cp932_source_utf8.csv")
	if utf8.Valid(encoded) || !bytes.Contains(encoded, []byte{0x87, 0x40}) || !bytes.Contains(encoded, []byte{0xee, 0xe0}) {
		t.Fatal("fixture is not independent CP932 bytes with extensions")
	}
	decoded, encoding, err := Decode(encoded)
	if err != nil || encoding != "Shift_JIS/CP932" || !bytes.Equal(decoded, source) {
		t.Fatal("CP932 extension decode differs from declared synthetic source", encoding, err)
	}
	options := Options{Source: "synthetic_bank", SourceAccountName: "synthetic_account", Now: fixedNow()}
	result, err := Parse(encoded, options)
	if err != nil || result.Encoding != "Shift_JIS/CP932" || result.NeedsMapping || len(result.Transactions) != 2 || len(result.Warnings) != 0 {
		t.Fatal(result, err)
	}
	counterpart, err := Parse(source, options)
	if err != nil || len(counterpart.Transactions) != 2 {
		t.Fatal(counterpart, err)
	}
	for i, want := range []struct {
		description, date, kind, balance string
		amount                           int64
	}{{"合成出金①髙", "2026-07-02", "expense", "998,800", 1200}, {"合成入金㈱テスト", "2026-07-03", "income", "1,002,300", 3500}} {
		got := result.Transactions[i]
		if got.Description != want.description || got.TransactionDate != want.date || got.TransactionType != want.kind || got.Amount != want.amount || got.RawData["残高"] != want.balance {
			t.Fatal("CP932 record lost direction/original characters or used balance", got)
		}
		if got.Fingerprint != counterpart.Transactions[i].Fingerprint || got.ImportFileKey != counterpart.Transactions[i].ImportFileKey {
			t.Fatal("same decoded records changed identity across encodings")
		}
	}
}
