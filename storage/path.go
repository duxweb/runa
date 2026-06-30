package storage

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

func cleanPath(value string) (string, error) {
	value = filepath.ToSlash(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "/")
	if value == "" || value == "." {
		return "", fmt.Errorf("storage path is required")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("storage path is required")
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", fmt.Errorf("storage path escapes root: %s", value)
	}
	return cleaned, nil
}

func joinPath(prefix string, name string) (string, error) {
	cleaned, err := cleanPath(name)
	if err != nil {
		return "", err
	}
	prefix = cleanPrefix(prefix)
	if prefix == "" {
		return cleaned, nil
	}
	return path.Join(prefix, cleaned), nil
}

func cleanPrefix(value string) string {
	value = filepath.ToSlash(strings.TrimSpace(value))
	value = strings.Trim(value, "/")
	if value == "" || value == "." {
		return ""
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return ""
	}
	return cleaned
}

func cleanURLPrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return ""
	}
	return "/" + strings.Trim(strings.ReplaceAll(value, "\\", "/"), "/")
}

func cleanDomain(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func joinURL(domain string, parts ...string) string {
	var clean []string
	for _, part := range parts {
		part = strings.Trim(strings.ReplaceAll(part, "\\", "/"), "/")
		if part != "" {
			clean = append(clean, part)
		}
	}
	joined := strings.Join(clean, "/")
	if domain == "" {
		if joined == "" {
			return "/"
		}
		return "/" + joined
	}
	if joined == "" {
		return domain
	}
	return domain + "/" + joined
}
