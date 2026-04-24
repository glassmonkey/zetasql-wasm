package zetasql

import "testing"

func TestParseResumeLocation(t *testing.T) {
	sql := "SELECT 1; SELECT 2"
	loc := NewParseResumeLocation(sql)

	if got, want := loc.Input(), sql; got != want {
		t.Errorf("Input() = %q, want %q", got, want)
	}
	if got, want := loc.BytePosition(), int32(0); got != want {
		t.Errorf("BytePosition() = %d, want %d", got, want)
	}
	if got, want := loc.AtEnd(), false; got != want {
		t.Errorf("AtEnd() = %v, want %v", got, want)
	}
}

func TestParseResumeLocation_AtEnd(t *testing.T) {
	sql := "SELECT 1"
	loc := NewParseResumeLocation(sql)

	// Simulate consuming all input
	loc.bytePosition = int32(len(sql))

	if got, want := loc.AtEnd(), true; got != want {
		t.Errorf("AtEnd() = %v, want %v", got, want)
	}
}
