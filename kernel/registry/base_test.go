package registry

import (
	"context"
	"errors"
	"testing"
)

type testCloser struct {
	name   string
	closed *int
	err    error
}

func (closer *testCloser) Name() string { return closer.name }
func (closer *testCloser) Close(context.Context) error {
	if closer.closed != nil {
		(*closer.closed)++
	}
	return closer.err
}

func TestEntriesFallback(t *testing.T) {
	entries := NewEntries[int]("")
	entries.Register("first", 1)
	entries.Register("second", 2)
	value, ok := entries.Entry("")
	if !ok || value != 1 {
		t.Fatalf("fallback entry = %d ok=%v", value, ok)
	}
	entries.SetFallback("second")
	value, ok = entries.Entry("")
	if !ok || value != 2 {
		t.Fatalf("fallback after set = %d ok=%v", value, ok)
	}
}

func TestBaseClosesDistinctDrivers(t *testing.T) {
	count := 0
	driver := &testCloser{name: "one", closed: &count}
	base := NewBase[*testCloser, int]("default")
	base.RegisterDriver("a", driver)
	base.RegisterDriver("b", driver)
	if err := base.Close(context.Background(), "driver"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if count != 1 {
		t.Fatalf("close count = %d", count)
	}
}

func TestCloseAllJoinsErrors(t *testing.T) {
	errExpected := errors.New("boom")
	err := CloseAll(context.Background(), map[string]*testCloser{
		"one": {name: "one", err: errExpected},
	}, "driver")
	if !errors.Is(err, errExpected) {
		t.Fatalf("error = %v", err)
	}
}
