package csvimport

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestDetectColumns(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		want    domain.CSVMapping
	}{
		{
			name:    "english",
			headers: []string{"date", "merchant", "amount"},
			want: domain.CSVMapping{
				DateColumn:        "date",
				DescriptionColumn: "merchant",
				AmountColumn:      "amount",
			},
		},
		{
			name:    "Japanese debit and credit",
			headers: []string{"取引日", "内容", "出金額", "入金額"},
			want: domain.CSVMapping{
				DateColumn:        "取引日",
				DescriptionColumn: "内容",
				DebitColumn:       "出金額",
				CreditColumn:      "入金額",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := DetectColumns(test.headers)
			if got != test.want {
				t.Fatalf("DetectColumns() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestParseUTF8CSV(t *testing.T) {
	data := readSample(t, "utf8_general.csv")
	result, err := Parse(data, Options{
		Source:            "credit_card",
		SourceAccountName: "personal",
		Now:               fixedNow(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Encoding != "UTF-8" {
		t.Fatalf("encoding = %q", result.Encoding)
	}
	if result.NeedsMapping {
		t.Fatal("expected automatic mapping")
	}
	if len(result.Transactions) != 6 {
		t.Fatalf("transactions = %d, want 6", len(result.Transactions))
	}
	if result.Transactions[1].Category != "酒・飲み会" {
		t.Fatalf("category = %q", result.Transactions[1].Category)
	}
}

func TestParseShiftJISCSV(t *testing.T) {
	utf8Data := []byte("ご利用日,ご利用先,ご利用金額\n2026/07/20,鳥貴族,4500\n")
	var encoded []byte
	writer := transform.NewWriter(byteSliceWriter{target: &encoded}, japanese.ShiftJIS.NewEncoder())
	if _, err := writer.Write(utf8Data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	result, err := Parse(encoded, Options{Now: fixedNow()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Encoding != "Shift_JIS/CP932" {
		t.Fatalf("encoding = %q", result.Encoding)
	}
	if len(result.Transactions) != 1 {
		t.Fatalf("transactions = %d", len(result.Transactions))
	}
	if result.Transactions[0].Description != "鳥貴族" {
		t.Fatalf("description = %q", result.Transactions[0].Description)
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"1,200", 1200},
		{"¥1,200", 1200},
		{"-1200", -1200},
		{"1200円", 1200},
		{"(1,200)", -1200},
		{"￥１，２００", 1200},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseAmount(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("ParseAmount(%q) = %d, want %d", test.input, got, test.want)
			}
		})
	}
}

func TestFingerprintIsStableAndSourceSensitive(t *testing.T) {
	first := Fingerprint("2026-07-01", 1200, " セブン  ", "card", "main")
	second := Fingerprint("2026-07-01", 1200, "セブン", "card", "main")
	otherSource := Fingerprint("2026-07-01", 1200, "セブン", "bank", "main")
	if first != second {
		t.Fatal("normalized equal inputs should have the same fingerprint")
	}
	if first == otherSource {
		t.Fatal("different sources should not have the same fingerprint")
	}
}

func readSample(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "sample_data", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixedNow() time.Time {
	return time.Date(2026, 7, 31, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
}

type byteSliceWriter struct {
	target *[]byte
}

func (w byteSliceWriter) Write(data []byte) (int, error) {
	*w.target = append(*w.target, data...)
	return len(data), nil
}

func TestRecordIdentityIsTypedExactAndStableAcrossEncoding(t *testing.T) {
	data := []byte("日付,摘要,金額\n2031-01-01,店舗123,2500\n2031-01-01,店舗123,2500\n2031-01-01,店舗123,-2500\n2031-01-01,店舗456,2500\n")
	options := Options{Source: "bank", SourceAccountName: "account"}
	parsed, err := Parse(data, options)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range parsed.Transactions {
		if seen[row.Fingerprint] {
			t.Fatal("legitimate records collided")
		}
		seen[row.Fingerprint] = true
	}
	encoded, _, err := transform.Bytes(japanese.ShiftJIS.NewEncoder(), data)
	if err != nil {
		t.Fatal(err)
	}
	other, err := Parse(encoded, options)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range other.Transactions {
		if row.Fingerprint != parsed.Transactions[i].Fingerprint || row.ImportFileKey != parsed.Transactions[i].ImportFileKey {
			t.Fatal("encoding changed record identity")
		}
	}
	options.MerchantRules = map[string]string{"店舗": "学習"}
	classified, _ := Parse(data, options)
	if classified.Transactions[0].Fingerprint != parsed.Transactions[0].Fingerprint {
		t.Fatal("classification affected identity")
	}
}
func TestProviderIDColumnRequiresRealExplicitColumn(t *testing.T) {
	data := []byte("日付,摘要,金額,取引ID\n2031-01-01,店,2500,a\n2031-01-01,店,2600,a\n")
	options := Options{Source: "bank", SourceAccountName: "account", Mapping: domain.CSVMapping{DateColumn: "日付", DescriptionColumn: "摘要", AmountColumn: "金額", TransactionIDColumn: "取引ID"}}
	if _, err := Parse(data, options); err == nil {
		t.Fatal("same ID with different content accepted")
	}
	options.Mapping.TransactionIDColumn = "missing"
	if _, err := Parse(data, options); err == nil {
		t.Fatal("unknown ID column accepted")
	}
}

func TestExplicitIDColumnSurvivesOtherColumnAutoDetection(t *testing.T) {
	result, err := Parse([]byte("日付,摘要,金額,取引ID\n2031-01-01,店,2500,a\n"), Options{Source: "bank", SourceAccountName: "account", Mapping: domain.CSVMapping{TransactionIDColumn: "取引ID"}})
	if err != nil || result.Mapping.TransactionIDColumn != "取引ID" || result.Transactions[0].ProviderTransactionID != "a" {
		t.Fatalf("explicit ID lost %+v %v", result, err)
	}
}

func TestRepeatedProviderIDRejectsEntireCSV(t *testing.T) {
	data := []byte("日付,摘要,金額,取引ID\n2034-01-01,店,2500,id-1\n2034-01-01,店,2500,id-1\n")
	opts := Options{Source: "bank", SourceAccountName: "account", Mapping: domain.CSVMapping{TransactionIDColumn: "取引ID"}}
	result, err := Parse(data, opts)
	if err == nil || len(result.Transactions) != 0 {
		t.Fatalf("repeated ID must reject the whole input: %+v %v", result, err)
	}
	// Removing the ID is not an automatic recovery: the caller must choose it.
	opts.Mapping.TransactionIDColumn = ""
	result, err = Parse(data, opts)
	if err != nil || len(result.Transactions) != 2 || result.Transactions[0].Fingerprint == result.Transactions[1].Fingerprint {
		t.Fatalf("ID-less legitimate occurrences lost: %+v %v", result, err)
	}
}
