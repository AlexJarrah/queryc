package parse

import (
	"slices"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestSchemaTypeKeysLongestFirst(t *testing.T) {
	for _, d := range []model.Dialect{model.DialectPostgres, model.DialectSQLite} {
		keys := schemaTypeKeys(d)
		for i := 1; i < len(keys); i++ {
			if len(keys[i]) > len(keys[i-1]) {
				t.Errorf("dialect %v: keys not longest-first at index %d: %q (%d) after %q (%d)",
					d, i, keys[i], len(keys[i]), keys[i-1], len(keys[i-1]))
			}
		}
	}
}

func TestSchemaTypeKeysContainsExpectedTypes(t *testing.T) {
	// Verify a few critical types are present in each dialect.
	pgKeys := schemaTypeKeys(model.DialectPostgres)
	if !slices.Contains(pgKeys, "DOUBLE PRECISION") {
		t.Error("postgres keys missing DOUBLE PRECISION")
	}
	if !slices.Contains(pgKeys, "SMALLINT") {
		t.Error("postgres keys missing SMALLINT")
	}
	if !slices.Contains(pgKeys, "BYTEA") {
		t.Error("postgres keys missing BYTEA")
	}

	sqliteKeys := schemaTypeKeys(model.DialectSQLite)
	if !slices.Contains(sqliteKeys, "DOUBLE PRECISION") {
		t.Error("sqlite keys missing DOUBLE PRECISION")
	}
	if !slices.Contains(sqliteKeys, "INTEGER") {
		t.Error("sqlite keys missing INTEGER")
	}
	if slices.Contains(sqliteKeys, "BYTEA") {
		t.Error("sqlite keys should not contain BYTEA")
	}
}

func TestFindTableByAliasRejectsAmbiguousGeneratedAlias(t *testing.T) {
	schema := model.Schema{Tables: map[string]model.Table{
		"users":          {Name: "users", Aliases: []string{"users", "u"}},
		"user_settings":  {Name: "user_settings", Aliases: []string{"user_settings", "u"}},
		"unrelated_rows": {Name: "unrelated_rows", Aliases: []string{"unrelated_rows", "ur"}},
	}}

	if table, ok := FindTableByAlias(schema, "u"); ok {
		t.Fatalf("ambiguous alias resolved to %q", table)
	}
	if table, ok := FindTableByAlias(schema, "users"); !ok || table != "users" {
		t.Fatalf("exact table name resolved to %q, %v", table, ok)
	}
}

func TestSchemaDoesNotTreatConstraintWordsInsideDefaultsAsConstraints(t *testing.T) {
	schema, err := Schema(`
CREATE TABLE examples (
	id INTEGER PRIMARY KEY,
	label TEXT DEFAULT 'NOT NULL',
	note TEXT DEFAULT 'SERIAL'
);
`, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Schema() error = %v", err)
	}
	table := schema.Tables["examples"]
	if !table.Columns["label"].Nullable {
		t.Fatal("NOT NULL inside a string default changed nullability")
	}
	if slices.Contains(table.AutoFields, "note") {
		t.Fatal("SERIAL inside a string default marked the field automatic")
	}
}

func TestSchemaPreservesUnknownPostgresTypeAndUsesSQLiteAffinity(t *testing.T) {
	pg, err := Schema(`CREATE TABLE events (duration INTERVAL);`, model.DialectPostgres)
	if err != nil {
		t.Fatal(err)
	}
	if got := pg.Tables["events"].Columns["duration"].SQLType; got != "INTERVAL" {
		t.Fatalf("Postgres type = %q, want INTERVAL", got)
	}

	sqlite, err := Schema(`CREATE TABLE events (counter CUSTOMINT);`, model.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	if got := sqlite.Tables["events"].Columns["counter"].SQLType; got != "INTEGER" {
		t.Fatalf("SQLite affinity = %q, want INTEGER", got)
	}
}

func TestSchemaRecognizesPostgresArrayTypes(t *testing.T) {
	schema, err := Schema(`CREATE TABLE examples (tags TEXT[] NOT NULL);`, model.DialectPostgres)
	if err != nil {
		t.Fatal(err)
	}
	if got := schema.Tables["examples"].Columns["tags"].SQLType; got != "TEXT[]" {
		t.Fatalf("array type = %q, want TEXT[]", got)
	}
}
