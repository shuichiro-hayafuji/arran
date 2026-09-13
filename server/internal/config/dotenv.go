package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// loadDotEnv loads local development settings without overriding values that
// were explicitly supplied by the process environment. ENV_FILE can be used
// to select an exact file. Otherwise, the first existing file from the server
// directory and repository root is used.
func loadDotEnv() error {
	if path := strings.TrimSpace(os.Getenv("ENV_FILE")); path != "" {
		return loadDotEnvFile(path)
	}

	for _, path := range []string{".env", "../.env"} {
		err := loadDotEnvFile(path)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return err
		}
	}
	return nil
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		key, value, ok, err := parseDotEnvLine(scanner.Text())
		if err != nil {
			return fmt.Errorf("load %s line %d: %w", path, lineNumber, err)
		}
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func parseDotEnvLine(line string) (key, value string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	}

	key, rawValue, found := strings.Cut(line, "=")
	key = strings.TrimSpace(key)
	if !found || !validEnvKey(key) {
		return "", "", false, fmt.Errorf("invalid environment assignment")
	}
	rawValue = strings.TrimSpace(rawValue)
	if rawValue == "" {
		return key, "", true, nil
	}

	switch rawValue[0] {
	case '\'':
		if len(rawValue) < 2 || rawValue[len(rawValue)-1] != '\'' {
			return "", "", false, fmt.Errorf("unterminated single-quoted value")
		}
		return key, rawValue[1 : len(rawValue)-1], true, nil
	case '"':
		unquoted, unquoteErr := strconv.Unquote(rawValue)
		if unquoteErr != nil {
			return "", "", false, fmt.Errorf("invalid double-quoted value")
		}
		return key, unquoted, true, nil
	default:
		return key, stripInlineComment(rawValue), true, nil
	}
}

func validEnvKey(key string) bool {
	for index, char := range key {
		if char == '_' || unicode.IsLetter(char) || (index > 0 && unicode.IsDigit(char)) {
			continue
		}
		return false
	}
	return key != ""
}

func stripInlineComment(value string) string {
	for index, char := range value {
		if char == '#' && index > 0 && unicode.IsSpace(rune(value[index-1])) {
			return strings.TrimSpace(value[:index])
		}
	}
	return strings.TrimSpace(value)
}
