package gotype

import "testing"

func TestApplyNullability(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
		nullable bool
		want     string
	}{
		{name: "adds pointer", typeName: "string", nullable: true, want: "*string"},
		{name: "keeps pointer", typeName: "*string", nullable: true, want: "*string"},
		{name: "removes pointer", typeName: "*string", want: "string"},
		{name: "defaults empty type", nullable: true, want: "*any"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ApplyNullability(tc.typeName, tc.nullable); got != tc.want {
				t.Fatalf("ApplyNullability(%q, %t) = %q, want %q", tc.typeName, tc.nullable, got, tc.want)
			}
		})
	}
}
