package parse

import (
	"strings"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestParamsAndHashtagsIgnoreStringLiterals(t *testing.T) {
	schema := model.Schema{
		Tables: map[string]model.Table{
			"users": {
				Name: "users",
				Columns: map[string]model.Column{
					"email": {Name: "email", SQLType: "TEXT"},
				},
				Aliases: []string{"users"},
			},
		},
	}
	content := `
@query({ name: "LiteralSafe" }) {
  SELECT '$email' AS literal_email, '#sort' AS literal_sort
  FROM users
  WHERE email = $email
  ORDER BY #sort:SortDirection;
}
`
	file, err := QueryFile(content, schema, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}
	q := file.Queries[0]
	if !strings.Contains(q.SQL, "'$email'") {
		t.Fatalf("expected literal $email preserved, got SQL:\n%s", q.SQL)
	}
	if !strings.Contains(q.SQL, "'#sort'") {
		t.Fatalf("expected literal #sort preserved, got SQL:\n%s", q.SQL)
	}
	if len(q.Params) != 1 || q.Params[0].Name != "email" {
		t.Fatalf("unexpected params %#v", q.Params)
	}
	if len(q.Hashtags) != 1 || q.Hashtags[0].Name != "sort" || q.Hashtags[0].IsSlice {
		t.Fatalf("unexpected hashtags %#v", q.Hashtags)
	}
}

func TestQueryParsingPreservesTaggedDollarQuotedLiterals(t *testing.T) {
	content := `
@query({ name: "DollarQuoted" }) {
  SELECT $body$-- not a comment
} @slice(not_a_feature) $ignored #ignored$body$ AS body,
         $actual:string AS actual;
}
`
	file, err := QueryFile(content, model.Schema{Tables: map[string]model.Table{}}, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}
	q := file.Queries[0]
	if !strings.Contains(q.SQL, "$body$-- not a comment\n} @slice(not_a_feature) $ignored #ignored$body$") {
		t.Fatalf("dollar-quoted body was changed:\n%s", q.SQL)
	}
	if len(q.Params) != 1 || q.Params[0].Name != "actual" {
		t.Fatalf("unexpected params %#v", q.Params)
	}
	if len(q.Hashtags) != 0 {
		t.Fatalf("unexpected hashtags %#v", q.Hashtags)
	}
}

func TestQueryBodyAllowsBackslashEscapeCharacterLiteral(t *testing.T) {
	content := `
@query({
  name: "GetAlbumsByName"
}) {
  SELECT *
  FROM albums
  WHERE name ILIKE $name ESCAPE '\'
  LIMIT 25;
}
`
	file, err := QueryFile(content, model.Schema{Tables: map[string]model.Table{}}, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}
	if len(file.Queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(file.Queries))
	}
	if !strings.Contains(file.Queries[0].SQL, `ESCAPE '\'`) {
		t.Fatalf("escape literal was changed:\n%s", file.Queries[0].SQL)
	}
}

func TestQueryBodySupportsPostgresEscapeStrings(t *testing.T) {
	content := `
@query({ name: "EscapedText" }) {
  SELECT E'it\'s valid' AS text;
}
`
	file, err := QueryFile(content, model.Schema{Tables: map[string]model.Table{}}, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}
	if !strings.Contains(file.Queries[0].SQL, `E'it\'s valid'`) {
		t.Fatalf("escape string was changed:\n%s", file.Queries[0].SQL)
	}
}

func TestHashtagSliceSyntaxVsType(t *testing.T) {
	schema := model.Schema{Tables: map[string]model.Table{}}
	content := `
@query({ name: "SliceFrag" }) {
  SELECT 1 WHERE true IN (#cols[]:[]string) AND #field:[]string = 'x';
}
`
	file, err := QueryFile(content, schema, model.DialectPostgres)
	if err != nil {
		t.Fatalf("QueryFile() error = %v", err)
	}
	byName := map[string]model.Hashtag{}
	for _, h := range file.Queries[0].Hashtags {
		byName[h.Name] = h
	}
	if !byName["cols"].IsSlice {
		t.Fatalf("expected #cols[] to be a slice fragment")
	}
	if byName["field"].IsSlice {
		t.Fatalf("expected #field:[]string not to be treated as a slice fragment")
	}
}

func TestSchemaRejectsUnknownPrimaryKey(t *testing.T) {
	_, err := Schema(`CREATE TABLE users (id UUID, PRIMARY KEY (missing));`, model.DialectPostgres)
	if err == nil {
		t.Fatal("expected error for unknown primary key column")
	}
}

func TestSchemaPreservesDefaultWithDashes(t *testing.T) {
	schema, err := Schema(`
CREATE TABLE users (
  id UUID PRIMARY KEY,
  note TEXT DEFAULT '-- not a comment'
);
`, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Schema() error = %v", err)
	}
	if _, ok := schema.Tables["users"]; !ok {
		t.Fatal("expected users table to be parsed")
	}
}

func TestDuplicateQueryNameRejected(t *testing.T) {
	content := `
@query({ name: "A" }) { SELECT 1; }
@query({ name: "A" }) { SELECT 2; }
`
	_, err := QueryFile(content, model.Schema{Tables: map[string]model.Table{}}, model.DialectPostgres)
	if err == nil {
		t.Fatal("expected duplicate query name error")
	}
}
