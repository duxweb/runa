package auth

import (
	"fmt"
	"strings"

	"github.com/duxweb/runa/core"
)

type defaultChecker struct{}

// DefaultPermissionChecker checks Info.Data permission collections.
func DefaultPermissionChecker() PermissionChecker { return defaultChecker{} }

func (defaultChecker) Check(_ any, info *Info, id string) error {
	if info == nil {
		return fmt.Errorf("auth is required")
	}
	if id == "" {
		return nil
	}
	if hasPermission(info.Data["permissions"], id) || hasPermission(info.Data["permission"], id) || hasPermission(info.Data["can"], id) {
		return nil
	}
	return fmt.Errorf("permission denied: %s", id)
}

func hasPermission(value any, id string) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed == "*" || typed == id
	case []string:
		for _, item := range typed {
			if item == "*" || item == id {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if hasPermission(item, id) {
				return true
			}
		}
	case map[string]bool:
		return typed["*"] || typed[id]
	case map[string]any:
		if core.Cast[bool](typed["*"]) || core.Cast[bool](typed[id]) {
			return true
		}
	}
	return false
}

func ShortName(id string) string {
	parts := strings.Split(strings.Trim(id, "."), ".")
	if len(parts) == 0 {
		return id
	}
	return parts[len(parts)-1]
}

func GroupName(id string) string {
	parts := strings.Split(strings.Trim(id, "."), ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}
