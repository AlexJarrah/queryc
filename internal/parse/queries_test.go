package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestQueryFileParsesImportsParamsAndHashtags(t *testing.T) {
	schema := model.Schema{
		Tables: map[string]model.Table{
			"users": {
				Name: "users",
				Columns: map[string]model.Column{
					"user_id": {Name: "user_id", SQLType: "UUID"},
					"email":   {Name: "email", SQLType: "TEXT"},
				},
				Aliases: []string{"users", "u"},
			},
		},
	}

	content := `
@import({ path: "example.com/queryc/testschemas" })

@query({
  name: "GetUsers"
}) {
  SELECT u.*
  FROM users u
  WHERE u.user_id = $user_id:uuid.UUID
  ORDER BY #sort:Sort;
}
`

	file, err := QueryFile(content, schema, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}

	if got := len(file.Imports); got != 1 {
		t.Fatalf("expected 1 import, got %d", got)
	}
	if got := len(file.Queries); got != 1 {
		t.Fatalf("expected 1 query, got %d", got)
	}

	query := file.Queries[0]
	if query.Name != "GetUsers" {
		t.Fatalf("unexpected query name %q", query.Name)
	}
	if len(query.Params) != 1 || query.Params[0].Type != "uuid.UUID" {
		t.Fatalf("unexpected params %#v", query.Params)
	}
	if len(query.Hashtags) != 1 || query.Hashtags[0].Type != "Sort" {
		t.Fatalf("unexpected hashtags %#v", query.Hashtags)
	}
	if query.SQL == query.RawSQL {
		t.Fatalf("expected rewritten SQL to differ from raw SQL")
	}
}

func TestQueryFileUpgradesRepeatedParamToExplicitPointerSlice(t *testing.T) {
	schema := model.Schema{
		Tables: map[string]model.Table{
			"listens": {
				Name: "listens",
				Columns: map[string]model.Column{
					"user_id": {Name: "user_id", SQLType: "UUID"},
				},
				Aliases: []string{"listens", "l"},
			},
		},
	}

	content := `
@query({
  name: "GetTopAlbums"
}) {
  SELECT l.*
  FROM listens l
  WHERE l.user_id = ANY($user_ids)
     OR CAST($user_ids:*[]uuid.UUID AS UUID[]) IS NULL;
}
`

	file, err := QueryFile(content, schema, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}

	if got := len(file.Queries); got != 1 {
		t.Fatalf("expected 1 query, got %d", got)
	}
	query := file.Queries[0]
	if got := len(query.Params); got != 1 {
		t.Fatalf("expected 1 param, got %d", got)
	}
	if got := query.Params[0].Type; got != "*[]uuid.UUID" {
		t.Fatalf("expected param type *[]uuid.UUID, got %q", got)
	}
	if !query.Params[0].Explicit {
		t.Fatalf("expected param to be marked explicit")
	}
}

func TestQueryFilePathMergesDirectoryDeterministically(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"z.sql": `@import({ path: "example.com/types" })
@query({ name: "Zed" }) { SELECT 2 AS value; }`,
		"a.sql": `@import({ path: "example.com/types" })
@query({ name: "Alpha" }) { SELECT 1 AS value; }`,
		"ignored.txt": `@query({ name: "Ignored" }) { SELECT 0 AS value; }`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	file, err := QueryFilePath(dir, model.Schema{Tables: map[string]model.Table{}}, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFilePath() error = %v", err)
	}
	if len(file.Imports) != 1 {
		t.Fatalf("imports = %#v, want one deduplicated import", file.Imports)
	}
	if len(file.Queries) != 2 || file.Queries[0].Name != "Alpha" || file.Queries[1].Name != "Zed" {
		t.Fatalf("queries are not in deterministic path order: %#v", file.Queries)
	}
}

func TestQueryFilePathReportsLineAndColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queries.sql")
	content := "\n\nunexpected"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := QueryFilePath(path, model.Schema{}, model.DialectPostgres)
	if err == nil || !strings.Contains(err.Error(), "queries.sql: unexpected character") ||
		!strings.Contains(err.Error(), "at line 3, col 1") {
		t.Fatalf("QueryFilePath() error = %v", err)
	}
}

func TestQueryFileParsesSchemaImport(t *testing.T) {
	content := `
@import({ path: "example.com/app/models", alias: "models", schema: true })
@query({ name: "AllUsers" }) { SELECT * FROM users; }
`
	file, err := QueryFile(content, model.Schema{}, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}
	want := model.Import{Path: "example.com/app/models", Alias: "models", Schema: true}
	if len(file.Imports) != 1 || file.Imports[0] != want {
		t.Fatalf("imports = %#v, want %#v", file.Imports, want)
	}
}

func TestQueryFileRejectsImportConflicts(t *testing.T) {
	tests := map[string]string{
		"missing schema alias": `@import({ path: "example.com/models", schema: true })`,
		"duplicate alias": `
			@import({ path: "example.com/one", alias: "types" })
			@import({ path: "example.com/two", alias: "types" })
		`,
		"multiple schemas": `
			@import({ path: "example.com/one", alias: "one", schema: true })
			@import({ path: "example.com/two", alias: "two", schema: true })
		`,
		"compact string form": `@import("database/sql")`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := QueryFile(content, model.Schema{}, model.DialectPostgres); err == nil {
				t.Fatal("expected import declaration to fail")
			}
		})
	}
}

func TestQueryFileRejectsConflictingParameterTypes(t *testing.T) {
	content := `@query({ name: "Conflict" }) {
		SELECT $value:string, $value:int64;
	}`
	if _, err := QueryFile(content, model.Schema{Tables: map[string]model.Table{}}, model.DialectPostgres); err == nil {
		t.Fatal("expected conflicting parameter types to fail")
	}
}

func TestQueryFileRejectsMixedFragmentShapes(t *testing.T) {
	content := `@query({ name: "Conflict" }) {
		SELECT #value, #value[];
	}`
	if _, err := QueryFile(content, model.Schema{Tables: map[string]model.Table{}}, model.DialectPostgres); err == nil {
		t.Fatal("expected scalar/slice fragment conflict to fail")
	}
}
