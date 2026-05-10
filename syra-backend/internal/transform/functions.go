package transform

import (
	"fmt"
	"strings"
	"unicode"
)

func formatAmount(value any) (string, error) {
	amount, err := toInt64(value)
	if err != nil {
		return "", err
	}
	if amount < 0 {
		return "", fmt.Errorf("amount must not be negative")
	}
	return fmt.Sprintf("%012d", amount), nil
}

func currencyNumeric(value any) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(toString(value))) {
	case "IDR":
		return "360", nil
	case "USD":
		return "840", nil
	case "EUR":
		return "978", nil
	case "JPY":
		return "392", nil
	case "SGD":
		return "702", nil
	default:
		return "", fmt.Errorf("unsupported currency %q", toString(value))
	}
}

func maskPan(value string) string {
	digits := onlyDigits(value)
	if len(digits) <= 10 {
		return strings.Repeat("*", len(digits))
	}

	prefix := digits[:6]
	suffix := digits[len(digits)-4:]
	return prefix + strings.Repeat("*", len(digits)-10) + suffix
}

func maskSensitiveValue(value any) string {
	raw := toString(value)
	digits := onlyDigits(raw)
	if len(digits) >= 12 && len(digits) <= 19 {
		return maskPan(raw)
	}
	if raw == "" {
		return ""
	}
	return "***"
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}
