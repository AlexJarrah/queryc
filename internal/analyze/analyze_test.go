package analyze

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

func TestEmbeddedNullableJoinAndGeneratedStruct(t *testing.T) {
	schema, queries := loadFixtureQueries(t)
	query := findQuery(t, queries, "GetUserWithConfiguration")

	analyzed, err := Query(schema, query, model.DialectPostgres)
	if err != nil {
		t.Fatalf("analyze query: %v", err)
	}

	if len(analyzed.EmbeddedTables) != 2 {
		t.Fatalf("expected two embedded tables, got %+v", analyzed.EmbeddedTables)
	}

	var sawUser, sawConfiguration bool
	for _, embedded := range analyzed.EmbeddedTables {
		switch embedded.TableName {
		case "users":
			sawUser = embedded.StructName == "User" && !embedded.IsNullable
		case "user_configurations":
			sawConfiguration = embedded.StructName == "UserConfiguration" && embedded.IsNullable
		}
	}
	if !sawUser || !sawConfiguration {
		t.Fatalf("unexpected embedded tables: %+v", analyzed.EmbeddedTables)
	}

	preferences := findField(t, analyzed.Fields, "preferences")
	if preferences.GeneratedStructKind != "struct" {
		t.Fatalf("expected preferences to generate a struct, got %q", preferences.GeneratedStructKind)
	}
	expected := map[string]string{
		"NotificationsEnabled": "*bool",
		"Timezone":             "*string",
	}
	for _, field := range preferences.GeneratedFields {
		want, ok := expected[field.FieldName]
		if !ok {
			continue
		}
		if field.GoType != want {
			t.Fatalf("generated preferences field %s type=%s want %s", field.FieldName, field.GoType, want)
		}
	}
}

func TestLeftJoinLateralSubqueryFieldIsNullable(t *testing.T) {
	schema, queries := loadFixtureQueries(t)
	query := findQuery(t, queries, "GetUserEmailWithLateral")

	analyzed, err := Query(schema, query, model.DialectPostgres)
	if err != nil {
		t.Fatalf("analyze query: %v", err)
	}

	field := findField(t, analyzed.Fields, "lateral_email")
	if field.GoType != "*string" {
		t.Fatalf("lateral_email GoType=%s, want *string", field.GoType)
	}
}

func loadFixtureQueries(t *testing.T) (model.Schema, []model.Query) {
	t.Helper()

	root := filepath.Join("..", "..")
	schemaBytes, err := os.ReadFile(filepath.Join(root, "test", "schema.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	queryBytes, err := os.ReadFile(filepath.Join(root, "test", "queries.sql"))
	if err != nil {
		t.Fatalf("read queries: %v", err)
	}

	schema, err := parse.Schema(string(schemaBytes), model.DialectPostgres)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	queryFile, err := parse.QueryFile(string(queryBytes), schema, model.DialectPostgres)
	if err != nil {
		t.Fatalf("parse queries: %v", err)
	}
	return schema, queryFile.Queries
}

func findQuery(t *testing.T, queries []model.Query, name string) model.Query {
	t.Helper()
	for _, query := range queries {
		if query.Name == name {
			return query
		}
	}
	t.Fatalf("query %s not found", name)
	return model.Query{}
}

func findField(t *testing.T, fields []model.ResultField, dbName string) model.ResultField {
	t.Helper()
	for _, field := range fields {
		if field.DBName == dbName {
			return field
		}
	}
	t.Fatalf("field %s not found", dbName)
	return model.ResultField{}
}
