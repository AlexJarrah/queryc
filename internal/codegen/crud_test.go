package codegen

import (
	"strings"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestWriteGetMany(t *testing.T) {
	schema := model.Schema{Tables: map[string]model.Table{
		"users": {
			Name: "users",
			Columns: map[string]model.Column{
				"user_id": {Name: "user_id", SQLType: "UUID"},
				"email":   {Name: "email", SQLType: "TEXT"},
			},
			PrimaryKeys: []string{"user_id"},
		},
		"user_memberships": {
			Name: "user_memberships",
			Columns: map[string]model.Column{
				"user_id": {Name: "user_id", SQLType: "UUID"},
				"org_id":  {Name: "org_id", SQLType: "BIGINT"},
				"role":    {Name: "role", SQLType: "TEXT"},
			},
			PrimaryKeys: []string{"user_id", "org_id"},
		},
	}}
	imports := []model.Import{{
		Path:   "example.com/test/schemas",
		Alias:  "schemas",
		Schema: true,
	}}

	tests := []struct {
		name     string
		dialect  model.Dialect
		contains []string
	}{
		{
			name:    "postgres single pk",
			dialect: model.DialectPostgres,
			contains: []string{
				"func GetUsers(userIDs []uuid.UUID) *querycruntime.Query[schemas.User] {",
				`if len(userIDs) == 0 {`,
				`return &querycruntime.Query[schemas.User]{Error: fmt.Errorf("userIDs slice cannot be empty")}`,
				`parts = append(parts, fmt.Sprintf("$%d", counter))`,
				"counter++",
				"query := `SELECT * FROM users WHERE user_id IN (` + strings.Join(parts, \", \") + `)`",
				"args = append(args, k)",
			},
		},
		{
			name:    "postgres composite pk",
			dialect: model.DialectPostgres,
			contains: []string{
				"type UserMembershipPrimaryKeys struct {",
				"\tUserID uuid.UUID",
				"\tOrgID  int64",
				"func GetUserMemberships(keys []UserMembershipPrimaryKeys) *querycruntime.Query[schemas.UserMembership] {",
				`if len(keys) == 0 {`,
				`return &querycruntime.Query[schemas.UserMembership]{Error: fmt.Errorf("keys slice cannot be empty")}`,
				"args = append(args, k.UserID)",
				"args = append(args, k.OrgID)",
				`conds = append(conds, fmt.Sprintf("user_id = $%d", counter+0))`,
				`conds = append(conds, fmt.Sprintf("org_id = $%d", counter+1))`,
				`parts = append(parts, "("+strings.Join(conds, " AND ")+")")`,
				"counter += 2",
				"query := `SELECT * FROM user_memberships WHERE ` + strings.Join(parts, \" OR \")",
			},
		},
		{
			name:    "sqlite single pk",
			dialect: model.DialectSQLite,
			contains: []string{
				"func GetUsers(userIDs []uuid.UUID) *querycruntime.Query[schemas.User] {",
				`if len(userIDs) == 0 {`,
				`return &querycruntime.Query[schemas.User]{Error: fmt.Errorf("userIDs slice cannot be empty")}`,
				`parts = append(parts, "?")`,
				"query := `SELECT * FROM users WHERE user_id IN (` + strings.Join(parts, \", \") + `)`",
			},
		},
		{
			name:    "sqlite composite pk",
			dialect: model.DialectSQLite,
			contains: []string{
				"type UserMembershipPrimaryKeys struct {",
				"\tUserID uuid.UUID",
				"\tOrgID  int64",
				"func GetUserMemberships(keys []UserMembershipPrimaryKeys) *querycruntime.Query[schemas.UserMembership] {",
				`conds = append(conds, "user_id = ?")`,
				`conds = append(conds, "org_id = ?")`,
				"query := `SELECT * FROM user_memberships WHERE ` + strings.Join(parts, \" OR \")",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Generate(schema, imports, nil, tt.dialect, "")
			if err != nil {
				t.Fatalf("Generate error = %v", err)
			}
			text := string(out)
			for _, want := range tt.contains {
				if !strings.Contains(text, want) {
					t.Errorf("generated output missing %q\n--- output ---\n%s", want, text)
				}
			}
		})
	}
}

func TestWriteGetManySkipsTablesWithoutPrimaryKeys(t *testing.T) {
	schema := model.Schema{Tables: map[string]model.Table{
		"logs": {
			Name:    "logs",
			Columns: map[string]model.Column{"message": {Name: "message", SQLType: "TEXT"}},
		},
	}}
	imports := []model.Import{{
		Path:   "example.com/test/schemas",
		Alias:  "schemas",
		Schema: true,
	}}

	out, err := Generate(schema, imports, nil, model.DialectPostgres, "")
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	if strings.Contains(string(out), "GetLogs(") {
		t.Fatalf("generated GetLogs for table without primary keys:\n%s", out)
	}
}
