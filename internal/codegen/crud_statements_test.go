package codegen

import (
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestBuildCRUDStatements(t *testing.T) {
	table := model.Table{
		Name: "users",
		Columns: map[string]model.Column{
			"id":         {Name: "id", SQLType: "INTEGER"},
			"email":      {Name: "email", SQLType: "TEXT"},
			"updated_at": {Name: "updated_at", SQLType: "TIMESTAMP"},
		},
		PrimaryKeys:  []string{"id"},
		HasUpdatedAt: true,
	}

	got := buildCRUDStatements("users", table, model.DialectPostgres)
	if got.Add != "INSERT INTO users (email, id) VALUES ($1, $2) RETURNING *" {
		t.Fatalf("Add = %q", got.Add)
	}
	if got.Update != "UPDATE users SET email = $2, updated_at = NOW() WHERE id = $1" {
		t.Fatalf("Update = %q", got.Update)
	}
	if got.Set != "INSERT INTO users (email, id) VALUES ($1, $2) ON CONFLICT(id) DO UPDATE SET email = excluded.email, updated_at = NOW() RETURNING *" {
		t.Fatalf("Set = %q", got.Set)
	}
}
