package csvimport

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/shuichirohayafuji/spendable-today/server/internal/domain"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

var columnCandidates = map[string][]string{
	"date": {
		"日付", "利用日", "ご利用日", "取引日", "date", "transaction_date",
	},
	"description": {
		"摘要", "内容", "利用先", "ご利用先", "加盟店名", "description", "merchant",
	},
	"amount": {
		"金額", "利用金額", "ご利用金額", "支払金額", "amount",
	},
	"debit": {
		"出金", "出金額", "支出", "debit", "withdrawal",
	},
	"credit": {
		"入金", "入金額", "収入", "credit", "deposit",
	},
}

var dateLayouts = []string{
	"2006-01-02", "2006/01/02", "2006.01.02", "2006年1月2日",
	"01/02/2006", "2-Jan-2006",
}

type Options struct {
	Mapping           domain.CSVMapping
	Source            string
	SourceAccountName string
	MerchantRules     map[string]string
	Now               time.Time
}

type ParseResult struct {
	Encoding     string
	Headers      []string
	Mapping      domain.CSVMapping
	NeedsMapping bool
	Transactions []domain.Transaction
	Warnings     []string
}

// Parse は文字コードと列を解決して明細を正規化する。
// 列の特定不足は NeedsMapping、不正な明細行は Warnings として返し、正常行を残す。
func Parse(data []byte, options Options) (ParseResult, error) {
	decoded, encoding, err := Decode(data)
	if err != nil {
		return ParseResult{}, err
	}
	reader := csv.NewReader(bytes.NewReader(decoded))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return ParseResult{}, fmt.Errorf("CSVを解析できません: %w", err)
	}
	if len(records) == 0 {
		return ParseResult{}, fmt.Errorf("CSVが空です")
	}
	headers := normalizeHeaders(records[0])
	if len(headers) == 0 {
		return ParseResult{}, fmt.Errorf("ヘッダーがありません")
	}
	mapping := options.Mapping
	if mapping.DateColumn == "" && mapping.DescriptionColumn == "" &&
		mapping.AmountColumn == "" && mapping.DebitColumn == "" {
		mapping = DetectColumns(headers)
	}
	needsMapping := mapping.DateColumn == "" || mapping.DescriptionColumn == "" ||
		(mapping.AmountColumn == "" && mapping.DebitColumn == "" && mapping.CreditColumn == "")
	result := ParseResult{
		Encoding:     encoding,
		Headers:      headers,
		Mapping:      mapping,
		NeedsMapping: needsMapping,
		Transactions: []domain.Transaction{},
		Warnings:     []string{},
	}
	if needsMapping {
		return result, nil
	}
	indexes, err := mappingIndexes(headers, mapping)
	if err != nil {
		return ParseResult{}, err
	}

	for rowIndex, record := range records[1:] {
		raw := make(map[string]any, len(headers))
		for index, header := range headers {
			if index < len(record) {
				raw[header] = strings.TrimSpace(record[index])
			} else {
				raw[header] = ""
			}
		}
		transaction, warning, err := normalizeRecord(record, raw, indexes, options)
		if err != nil {
			result.Warnings = append(
				result.Warnings,
				fmt.Sprintf("%d行目: %v", rowIndex+2, err),
			)
			continue
		}
		if warning != "" {
			result.Warnings = append(
				result.Warnings,
				fmt.Sprintf("%d行目: %s", rowIndex+2, warning),
			)
		}
		result.Transactions = append(result.Transactions, transaction)
	}
	return result, nil
}

func Decode(data []byte) ([]byte, string, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return data, "UTF-8", nil
	}
	reader := transform.NewReader(bytes.NewReader(data), japanese.ShiftJIS.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("Shift_JIS/CP932をUTF-8へ変換できません: %w", err)
	}
	if !utf8.Valid(decoded) {
		return nil, "", fmt.Errorf("文字コードを判定できません")
	}
	return decoded, "Shift_JIS/CP932", nil
}

func DetectColumns(headers []string) domain.CSVMapping {
	return domain.CSVMapping{
		DateColumn:        detect(headers, columnCandidates["date"]),
		DescriptionColumn: detect(headers, columnCandidates["description"]),
		AmountColumn:      detect(headers, columnCandidates["amount"]),
		DebitColumn:       detect(headers, columnCandidates["debit"]),
		CreditColumn:      detect(headers, columnCandidates["credit"]),
	}
}

// ParseAmount は通貨記号・桁区切り・全角数字を整数の円に変換する。
// 空欄は0、括弧付き金額は負数として扱い、小数は受け付けない。
func ParseAmount(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	negativeByParentheses := strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")")
	replacer := strings.NewReplacer(
		",", "", "，", "", "¥", "", "￥", "", "円", "", " ", "", "　", "",
		"(", "", ")", "", "＋", "+", "−", "-", "－", "-", "ー", "-",
	)
	value = replacer.Replace(value)
	value = toASCIIDigits(value)
	if value == "" || value == "+" || value == "-" {
		return 0, fmt.Errorf("金額 %q を解釈できません", value)
	}
	amount, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("金額 %q を解釈できません", value)
	}
	if negativeByParentheses && amount > 0 {
		amount = -amount
	}
	return amount, nil
}

// Fingerprint は日付・金額・正規化した摘要・取込元・口座から重複判定キーを作る。
// 分類の修正で再取込が発生しないよう、カテゴリをキーに含めない。
func Fingerprint(
	date string,
	amount int64,
	description string,
	source string,
	account string,
) string {
	canonical := strings.Join([]string{
		date,
		strconv.FormatInt(amount, 10),
		NormalizeMerchant(description),
		strings.ToLower(strings.TrimSpace(source)),
		strings.ToLower(strings.TrimSpace(account)),
	}, "|")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func NormalizeMerchant(description string) string {
	description = strings.ToLower(strings.TrimSpace(description))
	description = strings.Join(strings.Fields(description), " ")
	re := regexp.MustCompile(`[\s　]*(支店|店|店舗)?\s*[0-9０-９-]+$`)
	description = re.ReplaceAllString(description, "")
	return strings.TrimSpace(description)
}

// Categorize は保存済みの加盟店ルールを優先し、未登録ならローカルの語句で分類する。
func Categorize(merchant string, merchantRules map[string]string) string {
	normalized := NormalizeMerchant(merchant)
	if category := merchantRules[normalized]; category != "" {
		return category
	}
	rules := []struct {
		keywords []string
		category string
	}{
		{[]string{"居酒屋", "bar", "ビール", "ワイン", "鳥貴族"}, "酒・飲み会"},
		{[]string{"netflix", "spotify", "apple.com/bill", "subscription"}, "サブスクリプション"},
		{[]string{"コンビニ", "セブン", "lawson", "ローソン", "ファミリーマート"}, "コンビニ"},
		{[]string{"タクシー", "taxi", "uber trip", "go taxi"}, "タクシー"},
		{[]string{"jr", "suica", "pasmo", "鉄道", "交通"}, "交通"},
		{[]string{"書籍", "book", "udemy", "技術書"}, "学習"},
		{[]string{"病院", "クリニック", "薬局", "ジム"}, "健康"},
		{[]string{"レストラン", "食堂", "カフェ", "cafe", "restaurant"}, "外食"},
		{[]string{"スーパー", "market", "食品"}, "食費"},
		{[]string{"amazon", "楽天", "百貨店"}, "買い物"},
		{[]string{"映画", "cinema", "ゲーム"}, "娯楽"},
	}
	for _, rule := range rules {
		for _, keyword := range rule.keywords {
			if strings.Contains(normalized, strings.ToLower(keyword)) {
				return rule.category
			}
		}
	}
	return "未分類"
}

type columnIndexes struct {
	date, description, amount, debit, credit int
}

func mappingIndexes(headers []string, mapping domain.CSVMapping) (columnIndexes, error) {
	indexOf := func(name string) int {
		return slices.Index(headers, name)
	}
	indexes := columnIndexes{
		date:        indexOf(mapping.DateColumn),
		description: indexOf(mapping.DescriptionColumn),
		amount:      indexOf(mapping.AmountColumn),
		debit:       indexOf(mapping.DebitColumn),
		credit:      indexOf(mapping.CreditColumn),
	}
	if indexes.date < 0 || indexes.description < 0 {
		return columnIndexes{}, fmt.Errorf("指定された日付または摘要列がありません")
	}
	if indexes.amount < 0 && indexes.debit < 0 && indexes.credit < 0 {
		return columnIndexes{}, fmt.Errorf("指定された金額列がありません")
	}
	return indexes, nil
}

// normalizeRecord は単一金額列なら負数を収入と解釈し、入出金別列なら出金を優先する。
// 取引種別を分離し、金額は絶対値として保持する。
func normalizeRecord(
	record []string,
	raw map[string]any,
	indexes columnIndexes,
	options Options,
) (domain.Transaction, string, error) {
	valueAt := func(index int) string {
		if index < 0 || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	date, err := parseDate(valueAt(indexes.date))
	if err != nil {
		return domain.Transaction{}, "", err
	}
	description := valueAt(indexes.description)
	if description == "" {
		return domain.Transaction{}, "", fmt.Errorf("摘要が空です")
	}

	transactionType := "expense"
	var amount int64
	if indexes.amount >= 0 {
		amount, err = ParseAmount(valueAt(indexes.amount))
		if err != nil {
			return domain.Transaction{}, "", err
		}
		if amount < 0 {
			transactionType = "income"
			amount = -amount
		}
	} else {
		debit, debitErr := ParseAmount(valueAt(indexes.debit))
		credit, creditErr := ParseAmount(valueAt(indexes.credit))
		if debitErr != nil {
			return domain.Transaction{}, "", debitErr
		}
		if creditErr != nil {
			return domain.Transaction{}, "", creditErr
		}
		switch {
		case debit != 0:
			amount = abs(debit)
			transactionType = "expense"
		case credit != 0:
			amount = abs(credit)
			transactionType = "income"
		default:
			return domain.Transaction{}, "", fmt.Errorf("入出金額が空です")
		}
	}
	if amount == 0 {
		return domain.Transaction{}, "", fmt.Errorf("金額が0円です")
	}
	merchant := NormalizeMerchant(description)
	category := Categorize(merchant, options.MerchantRules)
	importedAt := options.Now
	if importedAt.IsZero() {
		importedAt = time.Now()
	}
	source := strings.TrimSpace(options.Source)
	if source == "" {
		source = "csv"
	}
	transaction := domain.Transaction{
		TransactionDate:    date,
		Description:        description,
		NormalizedMerchant: merchant,
		Amount:             amount,
		TransactionType:    transactionType,
		Category:           category,
		Source:             source,
		SourceAccountName:  strings.TrimSpace(options.SourceAccountName),
		ImportedAt:         importedAt.UTC().Format(time.RFC3339),
		RawData:            raw,
	}
	transaction.Fingerprint = Fingerprint(
		transaction.TransactionDate,
		transaction.Amount,
		transaction.Description,
		transaction.Source,
		transaction.SourceAccountName,
	)
	return transaction, "", nil
}

func parseDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	for _, layout := range dateLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("日付 %q を解釈できません", value)
}

func normalizeHeaders(headers []string) []string {
	result := make([]string, len(headers))
	for index, header := range headers {
		result[index] = strings.TrimSpace(strings.TrimPrefix(header, "\ufeff"))
	}
	return result
}

func detect(headers, candidates []string) string {
	for _, candidate := range candidates {
		for _, header := range headers {
			if strings.EqualFold(strings.TrimSpace(header), candidate) {
				return header
			}
		}
	}
	return ""
}

func toASCIIDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= '０' && r <= '９':
			builder.WriteRune('0' + (r - '０'))
		case unicode.IsSpace(r):
			continue
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func abs(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
