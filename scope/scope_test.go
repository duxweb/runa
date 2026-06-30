package scope

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestScopeKindAndLocals(t *testing.T) {
	scope := New(context.Background(), HTTP)
	scope.Set("name", "runa")
	scope.Set("count", "12")

	if scope.Kind() != HTTP {
		t.Fatalf("kind = %s", scope.Kind())
	}
	if got := scope.Get("name"); got != "runa" {
		t.Fatalf("name = %v", got)
	}
	if got := scope.GetAs[int]("count"); got != 12 {
		t.Fatalf("count = %d", got)
	}

	items := scope.Locals()
	items["name"] = "changed"
	if scope.Get("name") != "runa" {
		t.Fatal("locals snapshot should not mutate scope")
	}
}

func TestScopeCloseRunsCleanupInReverseOrder(t *testing.T) {
	calls := []string{}
	scope := New(context.Background(), Task)
	scope.Set("name", "runa")
	scope.OnClose(func(context.Context) error {
		calls = append(calls, "first")
		return nil
	})
	scope.OnClose(func(context.Context) error {
		calls = append(calls, "second")
		return nil
	})

	if err := scope.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if !scope.Closed() {
		t.Fatal("scope should be closed")
	}
	if got := scope.Get("name"); got != nil {
		t.Fatalf("local should be cleared, got %v", got)
	}
	expected := []string{"second", "first"}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("calls = %#v, want %#v", calls, expected)
	}

	if err := scope.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if !reflect.DeepEqual(calls, expected) {
		t.Fatalf("cleanup should only run once: %#v", calls)
	}
}

func TestScopeCloseJoinsCleanupErrors(t *testing.T) {
	first := errors.New("first")
	second := errors.New("second")
	scope := New(context.Background(), Queue)
	scope.OnClose(func(context.Context) error { return first })
	scope.OnClose(func(context.Context) error { return second })

	err := scope.Close()
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("joined error = %v", err)
	}
}

func TestScopeDefaultContext(t *testing.T) {
	scope := New(nil, Command)
	if scope.Context() == nil {
		t.Fatal("context should not be nil")
	}
}
