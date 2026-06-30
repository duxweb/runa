package runa

import (
	"context"
	"testing"
)

func TestDefaultAppUsesLatestRuntime(t *testing.T) {
	first := New()
	if err := first.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze first app: %v", err)
	}
	second := New()
	if err := second.Freeze(context.Background()); err != nil {
		t.Fatalf("freeze second app: %v", err)
	}
	if Default() != second {
		t.Fatal("new app did not replace default app")
	}
	if Config() == nil {
		t.Fatal("default app did not expose config")
	}
}
