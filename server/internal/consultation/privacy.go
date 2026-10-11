package consultation

import (
	"regexp"
	"strings"
	"time"
)

var longNumber = regexp.MustCompile(`[0-9０-９](?:[0-9０-９ -]*[0-9０-９]){6,}(?:円)?`)
var credentials = regexp.MustCompile(`(?i)(?:password|passwd|パスワード|認証コード|暗証番号|口座番号|カード番号|api[_ -]?key|bearer|access[_ -]?token|secret)[ :：=]*(?:は|が|is)?[ :：=]*[^\s、。]+`)

// Redact removes labelled credentials and unknown long numeric identifiers.
// Arbitrary unlabelled secrets cannot be identified reliably from free text.
var tokenPattern = regexp.MustCompile(`(?:sk-[A-Za-z0-9_-]{16,}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)`)

func Redact(text string) string {
	text = tokenPattern.ReplaceAllString(text, "[認証情報を除外]")
	return longNumber.ReplaceAllStringFunc(credentials.ReplaceAllString(text, "[機密情報を除外]"), func(value string) string {
		if _, err := time.Parse("2006-01-02", value); err == nil {
			return value
		}
		if strings.HasSuffix(value, "円") {
			return value
		}
		return "[番号を除外]"
	})
}
