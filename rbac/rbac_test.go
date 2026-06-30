package rbac

import (
	"context"
	"testing"

	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/core"
)

func TestMemoryStoreAndChecker(t *testing.T) {
	store := NewMemoryStore()
	store.AssignRole("1", "admin")
	store.Grant("admin", "system.user.*")
	store.GrantSubject("1", "system.report.view")

	checker := Checker(store)
	info := &auth.Info{Data: core.Map{"id": "1"}}
	if err := checker.Check(context.Background(), info, "system.user.edit"); err != nil {
		t.Fatalf("role permission denied: %v", err)
	}
	if err := checker.Check(context.Background(), info, "system.report.view"); err != nil {
		t.Fatalf("direct permission denied: %v", err)
	}
	if err := checker.Check(context.Background(), info, "system.audit.view"); err == nil {
		t.Fatal("expected permission denied")
	}
}

func TestMemoryStoreRevokes(t *testing.T) {
	store := NewMemoryStore()
	store.AssignRole("1", "admin", "editor")
	store.Grant("admin", "system.*")
	store.GrantSubject("1", "profile.view")

	store.RevokeRole("1", "admin")
	store.RevokeSubject("1", "profile.view")

	checker := Checker(store)
	info := &auth.Info{Data: core.Map{"id": "1"}}
	if err := checker.Check(context.Background(), info, "system.user.view"); err == nil {
		t.Fatal("expected revoked role permission denied")
	}
	if err := checker.Check(context.Background(), info, "profile.view"); err == nil {
		t.Fatal("expected revoked subject permission denied")
	}
}

func TestCheckerSubjectOption(t *testing.T) {
	store := NewMemoryStore()
	store.GrantSubject("tenant:7:user:1", "tenant.order.view")
	checker := Checker(store, Subject(func(ctx any, info *auth.Info) string {
		return "tenant:7:user:" + core.Cast[string](info.Data["id"])
	}))
	info := &auth.Info{Data: core.Map{"id": "1"}}
	if err := checker.Check(context.Background(), info, "tenant.order.view"); err != nil {
		t.Fatalf("custom subject denied: %v", err)
	}
}
