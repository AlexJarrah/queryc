package codegen

import (
	"strings"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestGenerateUsesSchemaImportAlias(t *testing.T) {
	schema := model.Schema{
		Tables: map[string]model.Table{
			"users": {
				Name: "users",
				Columns: map[string]model.Column{
					"user_id": {Name: "user_id", SQLType: "UUID", Nullable: false},
					"email":   {Name: "email", SQLType: "TEXT", Nullable: false},
				},
				PrimaryKeys: []string{"user_id"},
			},
		},
	}
	imports := []model.Import{{
		Path:   "example.com/app/schemas",
		Alias:  "sc",
		Schema: true,
	}}
	out, err := Generate(schema, imports, nil, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "func AddUser(row *sc.User)") {
		t.Fatalf("expected sc.User prefix, got:\n%s", text)
	}
	if !strings.Contains(text, `sc "example.com/app/schemas"`) {
		t.Fatalf("expected aliased schemas import, got:\n%s", text)
	}
	if strings.Contains(text, "func isZeroValue") || strings.Contains(text, `"reflect"`) {
		t.Fatalf("zero-value reflection should live in runtime:\n%s", text)
	}
	if !strings.Contains(text, "queryc.IsZeroValue(row.UserID)") {
		t.Fatalf("expected runtime primary-key validation:\n%s", text)
	}
	if !strings.Contains(text, `Error: fmt.Errorf("row cannot be nil")`) {
		t.Fatalf("expected nil-row validation:\n%s", text)
	}
}
