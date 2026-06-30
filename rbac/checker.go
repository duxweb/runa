package rbac

import (
	"context"
	"fmt"

	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/core"
)

// Checker creates a permission checker backed by an RBAC store.
func Checker(store Store, items ...Option) auth.PermissionChecker {
	opts := options{subject: defaultSubject}
	for _, item := range items {
		if item != nil {
			item.applyRBAC(&opts)
		}
	}
	return auth.PermissionFunc(func(ctx any, info *auth.Info, id string) error {
		if id == "" {
			return nil
		}
		if store == nil {
			return fmt.Errorf("rbac store is required")
		}
		if info == nil {
			return fmt.Errorf("auth is required")
		}
		subject := opts.subject(ctx, info)
		if subject == "" {
			return fmt.Errorf("rbac subject is required")
		}
		base := contextOf(ctx)
		roles, err := store.Roles(base, subject)
		if err != nil {
			return err
		}
		rolePermissions, err := store.RolePermissions(base, roles)
		if err != nil {
			return err
		}
		subjectPermissions, err := store.SubjectPermissions(base, subject)
		if err != nil {
			return err
		}
		if allows(id, rolePermissions) || allows(id, subjectPermissions) {
			return nil
		}
		return fmt.Errorf("permission denied: %s", id)
	})
}

type contextGetter interface {
	Context() context.Context
}

func contextOf(ctx any) context.Context {
	if getter, ok := ctx.(contextGetter); ok && getter.Context() != nil {
		return getter.Context()
	}
	return context.Background()
}

func defaultSubject(_ any, info *auth.Info) string {
	if info == nil || info.Data == nil {
		return ""
	}
	for _, key := range []string{"id", "user_id", "subject", "sub"} {
		value := core.Cast[string](info.Data[key])
		if value != "" {
			return value
		}
	}
	return ""
}

func allows(id string, permissions []string) bool {
	for _, permission := range permissions {
		if match(permission, id) {
			return true
		}
	}
	return false
}

func match(pattern string, id string) bool {
	if pattern == "*" || pattern == id {
		return true
	}
	if len(pattern) > 2 && pattern[len(pattern)-2:] == ".*" {
		prefix := pattern[:len(pattern)-1]
		return len(id) >= len(prefix) && id[:len(prefix)] == prefix
	}
	return false
}
