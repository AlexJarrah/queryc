package utils

import "testing"

func TestCoalesce(t *testing.T) {
	if got := Coalesce("first", "second"); got != "first" {
		t.Fatalf("Coalesce(first, second) = %q, want first", got)
	}
	if got := Coalesce("", "second"); got != "second" {
		t.Fatalf("Coalesce(\"\", second) = %q, want second", got)
	}
	if got := Coalesce("", "", "third"); got != "third" {
		t.Fatalf("Coalesce(\"\", \"\", third) = %q, want third", got)
	}
	if got := Coalesce(""); got != "" {
		t.Fatalf("Coalesce(\"\") = %q, want empty", got)
	}
	if got := Coalesce(0, 2); got != 2 {
		t.Fatalf("Coalesce(0, 2) = %d, want 2", got)
	}
	if got := Coalesce(1, 2); got != 1 {
		t.Fatalf("Coalesce(1, 2) = %d, want 1", got)
	}
}
