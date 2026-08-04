package dialect

import (
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestDialectConfiguration(t *testing.T) {
	tests := []struct {
		dialect     model.Dialect
		integerType string
		placeholder string
		timestamp   string
		pkg         string
	}{
		{
			dialect:     model.DialectPostgres,
			integerType: "int32",
			placeholder: "$2",
			timestamp:   "NOW()",
			pkg:         "postgres",
		},
		{
			dialect:     model.DialectSQLite,
			integerType: "int64",
			placeholder: "?",
			timestamp:   "datetime('now')",
			pkg:         "sqlite",
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.dialect), func(t *testing.T) {
			if got := GoTypeForSQL(tc.dialect, "INTEGER"); got != tc.integerType {
				t.Fatalf("GoTypeForSQL(INTEGER) = %q, want %q", got, tc.integerType)
			}
			if got := Placeholder(tc.dialect, 2); got != tc.placeholder {
				t.Fatalf("Placeholder(2) = %q, want %q", got, tc.placeholder)
			}
			if got := CurrentTimestamp(tc.dialect); got != tc.timestamp {
				t.Fatalf("CurrentTimestamp() = %q, want %q", got, tc.timestamp)
			}
			if got := PackageName(tc.dialect); got != tc.pkg {
				t.Fatalf("PackageName() = %q, want %q", got, tc.pkg)
			}
		})
	}
}

func TestGoTypeForSQLReturnsAnyForUnknownType(t *testing.T) {
	if got := GoTypeForSQL(model.DialectPostgres, "GEOGRAPHY"); got != "any" {
		t.Fatalf("GoTypeForSQL(GEOGRAPHY) = %q, want any", got)
	}
}
