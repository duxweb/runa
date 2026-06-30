package rbac

import (
	"context"

	"github.com/duxweb/runa/auth"
)

// Store resolves roles and permissions for one subject.
type Store interface {
	Roles(ctx context.Context, subject string) ([]string, error)
	RolePermissions(ctx context.Context, roles []string) ([]string, error)
	SubjectPermissions(ctx context.Context, subject string) ([]string, error)
}

// SubjectFunc resolves the current permission subject.
type SubjectFunc func(ctx any, info *auth.Info) string

// Option configures the RBAC checker.
type Option interface {
	applyRBAC(*options)
}

type optionFunc func(*options)

func (fn optionFunc) applyRBAC(options *options) { fn(options) }

type options struct {
	subject SubjectFunc
}

// Subject sets the current subject resolver.
func Subject(fn SubjectFunc) Option {
	return optionFunc(func(options *options) {
		if fn != nil {
			options.subject = fn
		}
	})
}
