package codegen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestGeneratedBindingsCompileForBothDialects(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	goMod := fmt.Sprintf(`module example.com/queryccompile

go 1.26.4

require github.com/AlexJarrah/queryc v0.0.0

replace github.com/AlexJarrah/queryc => %q
`, filepath.ToSlash(root))
	writeTestFile(t, filepath.Join(temp, "go.mod"), goMod)
	writeTestFile(t, filepath.Join(temp, "schemas", "schemas.go"), `package schemas

type User struct {
	ID    int64  `+"`db:\"id\"`"+`
	Email string `+"`db:\"email\"`"+`
}

type UserMembership struct {
	UserID int64  `+"`db:\"user_id\"`"+`
	OrgID  int64  `+"`db:\"org_id\"`"+`
	Role   string `+"`db:\"role\"`"+`
}
`)

	schema := model.Schema{Tables: map[string]model.Table{
		"users": {
			Name: "users",
			Columns: map[string]model.Column{
				"id":    {Name: "id", SQLType: "INTEGER"},
				"email": {Name: "email", SQLType: "TEXT"},
			},
			PrimaryKeys: []string{"id"},
		},
		"user_memberships": {
			Name: "user_memberships",
			Columns: map[string]model.Column{
				"user_id": {Name: "user_id", SQLType: "INTEGER"},
				"org_id":  {Name: "org_id", SQLType: "INTEGER"},
				"role":    {Name: "role", SQLType: "TEXT"},
			},
			PrimaryKeys: []string{"user_id", "org_id"},
		},
	}}
	imports := []model.Import{{
		Path:   "example.com/queryccompile/schemas",
		Alias:  "schemas",
		Schema: true,
	}}
	for _, dialect := range []model.Dialect{model.DialectPostgres, model.DialectSQLite} {
		output, err := Generate(
			schema,
			imports,
			nil,
			dialect,
			"",
		)
		if err != nil {
			t.Fatalf("Generate(%d) error = %v", dialect, err)
		}
		writeTestFile(t, filepath.Join(temp, strconv.Itoa(int(dialect)), "bindings.go"), string(output))

		emptyOutput, err := Generate(
			model.Schema{Tables: map[string]model.Table{}},
			nil,
			nil,
			dialect,
			"",
		)
		if err != nil {
			t.Fatalf("Generate(empty, %d) error = %v", dialect, err)
		}
		writeTestFile(t, filepath.Join(temp, "empty-"+strconv.Itoa(int(dialect)), "bindings.go"), string(emptyOutput))
	}

	cmd := exec.Command("go", "test", "-mod=mod", "./...")
	cmd.Dir = temp
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated bindings do not compile: %v\n%s", err, output)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
