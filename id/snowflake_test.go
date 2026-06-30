package id

import (
	"testing"
)

func TestRandom(t *testing.T) {
	value, err := Random(16)
	if err != nil {
		t.Fatalf("random: %v", err)
	}
	if value == "" {
		t.Fatal("empty random id")
	}
	hex, err := RandomHex(16)
	if err != nil {
		t.Fatalf("hex: %v", err)
	}
	if len(hex) != 32 {
		t.Fatalf("hex len = %d", len(hex))
	}
}
