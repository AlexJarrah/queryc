package runtime

import (
	"database/sql"
	"testing"
	"time"
)

func TestIsZeroValue(t *testing.T) {
	zeroInt := 0
	cases := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "nil", value: nil, want: true},
		{name: "integer", value: 0, want: true},
		{name: "nonzero integer", value: 1, want: false},
		{name: "time", value: time.Time{}, want: true},
		{name: "slice", value: []string(nil), want: true},
		{name: "empty allocated slice", value: []string{}, want: false},
		{name: "nil pointer", value: (*int)(nil), want: true},
		{name: "non-nil pointer", value: &zeroInt, want: false},
		{name: "invalid nullable", value: sql.NullString{String: "ignored"}, want: true},
		{name: "valid nullable", value: sql.NullString{Valid: true}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsZeroValue(tc.value); got != tc.want {
				t.Fatalf("IsZeroValue(%#v) = %t, want %t", tc.value, got, tc.want)
			}
		})
	}
}
