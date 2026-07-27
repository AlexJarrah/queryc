package analyze

import (
	"strings"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestGetQueryTablesIgnoresSQLKeywordsAsAliases(t *testing.T) {
	tables := getQueryTables("SELECT email FROM users WHERE id = 1")
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %#v", tables)
	}
	if tables[0][0] != "users" || tables[0][1] != "users" {
		t.Fatalf("unexpected table/alias %#v", tables[0])
	}
}

func TestShouldGenerateStructForReturningAndNamedSelects(t *testing.T) {
	fields := []model.ResultField{{Name: "ID", DBName: "id", GoType: "uuid.UUID"}}
	if ok, name := shouldGenerateStruct("AdditionalEmails", fields, "SELECT id FROM users"); !ok || name != "AdditionalEmailsResult" {
		t.Fatalf("expected AdditionalEmails to emit result struct, got %v %q", ok, name)
	}
	if ok, name := shouldGenerateStruct("InsertUser", fields, "INSERT INTO users (id) VALUES ($1) RETURNING id"); !ok || name == "" {
		t.Fatalf("expected RETURNING query to emit result struct, got %v %q", ok, name)
	}
	if ok, _ := shouldGenerateStruct("DeleteUser", nil, "DELETE FROM users WHERE id = $1"); ok {
		t.Fatal("expected DELETE without RETURNING not to emit result struct")
	}
}

func TestExtractSelectWithoutFromAndReturning(t *testing.T) {
	if got := extractMainSelectClause("SELECT 1 AS n"); got != "1 AS n" {
		t.Fatalf("SELECT without FROM: got %q", got)
	}
	if got := extractMainSelectClause("INSERT INTO users (id) VALUES ($1) RETURNING id, email"); got != "id, email" {
		t.Fatalf("RETURNING: got %q", got)
	}
}

func TestExtractCTEsSkipsRecursiveKeyword(t *testing.T) {
	defs := extractCTEs(`WITH RECURSIVE tree AS (SELECT id FROM nodes) SELECT * FROM tree`)
	if len(defs) != 1 || defs[0].Name != "tree" {
		t.Fatalf("unexpected CTEs %#v", defs)
	}
}

func TestRightJoinMarksLeftSideNullable(t *testing.T) {
	tables, aliases := getTablesWithNullableJoin(`
SELECT * FROM users u
RIGHT JOIN posts p ON p.user_id = u.user_id
`)
	if !tables["users"] && !aliases["u"] {
		t.Fatalf("expected users/u nullable, tables=%v aliases=%v", tables, aliases)
	}
	if tables["posts"] || aliases["p"] {
		t.Fatalf("did not expect posts/p nullable, tables=%v aliases=%v", tables, aliases)
	}
}

func TestAnalyzeQueryRejectsUnresolvedResultType(t *testing.T) {
	query := model.Query{
		Name:      "UnknownResult",
		RawSQL:    "SELECT custom_function() AS value",
		SourceSQL: "SELECT custom_function() AS value",
	}

	_, err := Query(model.Schema{}, query, model.DialectPostgres)
	if err == nil {
		t.Fatal("expected unresolved result type error")
	}
	for _, want := range []string{"value", "custom_function()", "explicit type annotation"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error %q to contain %q", err, want)
		}
	}
}

func TestAnalyzeQueryAcceptsExplicitAndConcreteResultTypes(t *testing.T) {
	tests := []string{
		"SELECT custom_function() AS value:string",
		"SELECT CAST(custom_function() AS TEXT) AS value",
	}

	for _, sql := range tests {
		query := model.Query{Name: "TypedResult", RawSQL: sql, SourceSQL: sql}
		if _, err := Query(model.Schema{}, query, model.DialectPostgres); err != nil {
			t.Fatalf("Query(%q): %v", sql, err)
		}
	}
}

func TestInferCoalesceType(t *testing.T) {
	goType, nullable := inferCoalesceType(
		model.Schema{},
		"COALESCE(NULL, 42)",
		nil,
		nil,
		nil,
		nil,
		false,
		model.DialectPostgres,
	)
	if goType != "int64" || nullable {
		t.Fatalf("inferCoalesceType() = (%q, %t), want (int64, false)", goType, nullable)
	}
}

func TestLiteralNullability(t *testing.T) {
	if !isNullLiteral(" null ") {
		t.Fatal("NULL should be recognized case-insensitively")
	}
	for _, literal := range []string{"1", "'value'", "TRUE", "NOW()"} {
		if !isDefinitelyNonNullLiteral(literal) {
			t.Fatalf("%q should be definitely non-null", literal)
		}
	}
}
