package query

import (
	"errors"
	"testing"

	"github.com/AlexJarrah/queryc/internal/utils"
	"github.com/AlexJarrah/queryc/runtime"
)

func TestValidate(t *testing.T) {
	preset := errors.New("preset")
	if err := Validate("SELECT 1", "", false, nil); err != nil {
		t.Fatalf("Validate(SQL) error = %v", err)
	}
	if err := Validate("", "prepared", true, nil); err != nil {
		t.Fatalf("Validate(statement) error = %v", err)
	}
	if err := Validate("", "", false, nil); !errors.Is(err, runtime.ErrInvalidQuery) {
		t.Fatalf("Validate(empty) error = %v", err)
	}
	if err := Validate("SELECT 1", "", false, preset); !errors.Is(err, preset) {
		t.Fatalf("Validate(preset) error = %v", err)
	}
	if err := NilQuery(); !errors.Is(err, runtime.ErrInvalidQuery) {
		t.Fatalf("NilQuery() error = %v", err)
	}
}

func TestPreparedStatementPreference(t *testing.T) {
	// Postgres runtime prefers StmtName so PrepareStatements is honored.
	if got := utils.Coalesce("stmt", "SELECT 1"); got != "stmt" {
		t.Fatalf("utils.Coalesce(stmt, SQL) = %q, want stmt", got)
	}
	if got := utils.Coalesce("", "SELECT 1"); got != "SELECT 1" {
		t.Fatalf("utils.Coalesce(\"\", SQL) = %q, want SQL", got)
	}
}
