package core

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestCastBasicTypes(t *testing.T) {
	if got := Cast[int]("12"); got != 12 {
		t.Fatalf("Cast[int] = %d", got)
	}
	if got := Cast[string]([]byte("abc")); got != "abc" {
		t.Fatalf("Cast[string] = %q", got)
	}
	if got := Cast[uint64]("42"); got != 42 {
		t.Fatalf("Cast[uint64] = %d", got)
	}
	if got := Cast[uint8]("300", 7); got != 7 {
		t.Fatalf("Cast[uint8] fallback = %d", got)
	}
	if got := Cast[int]("bad", 7); got != 7 {
		t.Fatalf("Cast[int] fallback = %d", got)
	}
	if got := Cast[bool]("on"); !got {
		t.Fatal("Cast[bool] on should be true")
	}
	if got := Cast[bool]("off", true); got {
		t.Fatal("Cast[bool] off should be false")
	}
}

func TestCastNamedNumericTypes(t *testing.T) {
	type userID int64
	type enabled bool
	type score float64

	if got := Cast[int](userID(42)); got != 42 {
		t.Fatalf("named int source = %d", got)
	}
	if got := Cast[userID]("42"); got != 42 {
		t.Fatalf("named int target = %d", got)
	}
	if got := Cast[bool](enabled(true)); !got {
		t.Fatal("named bool source should be true")
	}
	if got := Cast[score]("1.5"); got != 1.5 {
		t.Fatalf("named float target = %f", got)
	}
}

func TestCastRejectsLossyFloatToInteger(t *testing.T) {
	if _, ok := CastOK[int](1.2); ok {
		t.Fatal("expected fractional float to int failure")
	}
	if _, ok := CastOK[uint](1.2); ok {
		t.Fatal("expected fractional float to uint failure")
	}
	if got, ok := CastOK[int](1.0); !ok || got != 1 {
		t.Fatalf("whole float to int = %d %v", got, ok)
	}
}

func TestCastSlicesAndMaps(t *testing.T) {
	if got := Cast[[]string]("go,runa,web"); !reflect.DeepEqual(got, []string{"go", "runa", "web"}) {
		t.Fatalf("slice = %#v", got)
	}
	if got := Cast[[]int]([]any{"1", 2, float64(3)}); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("int slice = %#v", got)
	}
	if got := Cast[[]byte]("abc"); string(got) != "abc" {
		t.Fatalf("bytes = %q", string(got))
	}
	m := Cast[Map](`{"name":"Runa","enabled":true}`)
	if m["name"] != "Runa" || m["enabled"] != true {
		t.Fatalf("map = %#v", m)
	}
	typed := Cast[map[string]int](Map{"a": "1", "b": 2})
	if typed["a"] != 1 || typed["b"] != 2 {
		t.Fatalf("typed map = %#v", typed)
	}
}

func TestCastJSONRawStructAndPointer(t *testing.T) {
	raw := Cast[JSONRaw](Map{"name": "Runa"})
	if !json.Valid(raw.Bytes()) || raw.Map()["name"] != "Runa" {
		t.Fatalf("raw = %s", raw.String())
	}
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	value := Cast[user](`{"name":"Runa","age":"3"}`)
	if value.Name != "Runa" || value.Age != 3 {
		t.Fatalf("struct = %+v", value)
	}
	ptr := Cast[*user](Map{"name": "Ptr", "age": 4})
	if ptr == nil || ptr.Name != "Ptr" || ptr.Age != 4 {
		t.Fatalf("ptr = %+v", ptr)
	}
}

func TestCastTimeTypes(t *testing.T) {
	if got := Cast[time.Duration]("2s"); got != 2*time.Second {
		t.Fatalf("duration = %s", got)
	}
	when := Cast[time.Time]("2026-06-27T10:00:00Z")
	if when.IsZero() || when.UTC().Year() != 2026 {
		t.Fatalf("time = %s", when)
	}
}

func TestCastOKFailure(t *testing.T) {
	if _, ok := CastOK[[]int]("1,bad"); ok {
		t.Fatal("expected slice cast failure")
	}
	if _, ok := CastOK[Map](`[1,2]`); ok {
		t.Fatal("expected non-object map failure")
	}
}
