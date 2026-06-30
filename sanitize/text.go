package sanitize

import (
	"net/url"
	"strings"
	"unicode"
)

// Text strips HTML and removes control characters except common whitespace.
func Text(input string) string {
	clean := PlainText().Sanitize(input)
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, clean)
}

// URL returns a safe http(s), mailto, tel, absolute path, or empty string.
func URL(input string) string {
	value := strings.TrimSpace(input)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "vbscript:") {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" {
		if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
			return value
		}
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "mailto", "tel":
		return value
	default:
		return ""
	}
}
