package core

import (
	"testing"
	"time"
)

func TestClockUsesApplicationLocation(t *testing.T) {
	originalLocation := Location()
	defer SetLocation(originalLocation)
	defer SetClock(nil)

	location := time.FixedZone("RUNA", 8*60*60)
	SetLocation(location)
	SetClock(FixedClock(time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)))

	now := Now()
	if now.Location() != location {
		t.Fatalf("location = %v", now.Location())
	}
	if now.Hour() != 20 {
		t.Fatalf("hour = %d", now.Hour())
	}
}

func TestParseUsesApplicationLocation(t *testing.T) {
	originalLocation := Location()
	defer SetLocation(originalLocation)

	location := time.FixedZone("RUNA", 8*60*60)
	SetLocation(location)

	value, err := Parse("2006-01-02 15:04:05", "2026-06-30 12:00:00")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if value.Location() != location {
		t.Fatalf("location = %v", value.Location())
	}
	if value.UTC().Hour() != 4 {
		t.Fatalf("utc hour = %d", value.UTC().Hour())
	}
}
