package postgres

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	qruntime "github.com/AlexJarrah/queryc/runtime"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ Preparer = (*pgx.Conn)(nil)

func TestQueryValidate(t *testing.T) {
	var nilQuery *Query[struct{}]
	if err := nilQuery.validate(); !errors.Is(err, qruntime.ErrInvalidQuery) {
		t.Fatalf("nil query error = %v, want ErrInvalidQuery", err)
	}

	empty := &Query[struct{}]{}
	if err := empty.validate(); !errors.Is(err, qruntime.ErrInvalidQuery) {
		t.Fatalf("empty query error = %v, want ErrInvalidQuery", err)
	}

	prepared := &Query[struct{}]{StmtName: "prepared_query"}
	if err := prepared.validate(); err != nil {
		t.Fatalf("prepared query validate() error = %v", err)
	}
}

type testConnection struct {
	rows         pgx.Rows
	lastQuerySQL string
	lastExecSQL  string
}

func (c *testConnection) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	c.lastQuerySQL = sql
	return c.rows, nil
}

func (c *testConnection) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	c.lastExecSQL = sql
	return pgconn.NewCommandTag("UPDATE 3"), nil
}

type testRows struct {
	values [][]any
	index  int
	closed bool
}

func (r *testRows) Close()                        { r.closed = true }
func (r *testRows) Err() error                    { return nil }
func (r *testRows) CommandTag() pgconn.CommandTag { return pgconn.NewCommandTag("SELECT 1") }
func (r *testRows) Values() ([]any, error)        { return r.values[r.index-1], nil }
func (r *testRows) RawValues() [][]byte           { return nil }
func (r *testRows) Conn() *pgx.Conn               { return nil }
func (r *testRows) FieldDescriptions() []pgconn.FieldDescription {
	return []pgconn.FieldDescription{{Name: "id"}}
}

func (r *testRows) Next() bool {
	if r.index >= len(r.values) {
		r.closed = true
		return false
	}
	r.index++
	return true
}

func (r *testRows) Scan(dest ...any) error {
	for i, value := range r.values[r.index-1] {
		if scanner, ok := dest[i].(sql.Scanner); ok {
			if err := scanner.Scan(value); err != nil {
				return err
			}
			continue
		}
		target := reflect.ValueOf(dest[i]).Elem()
		target.Set(reflect.ValueOf(value).Convert(target.Type()))
	}
	return nil
}

func TestQueryExecution(t *testing.T) {
	type row struct {
		ID int64 `db:"id"`
	}

	rows := &testRows{values: [][]any{{int64(7)}}}
	conn := &testConnection{rows: rows}
	query := &Query[row]{SQL: "SELECT id FROM users"}
	got, err := query.GetOne(context.Background(), conn)
	if err != nil {
		t.Fatalf("GetOne() error = %v", err)
	}
	if got.ID != 7 || !rows.closed {
		t.Fatalf("GetOne() = %#v, rows closed = %t", got, rows.closed)
	}
	if conn.lastQuerySQL != "SELECT id FROM users" {
		t.Fatalf("GetOne() sql = %q, want SQL", conn.lastQuerySQL)
	}

	execConn := &testConnection{}
	affected, err := (&Query[struct{}]{SQL: "UPDATE users"}).Execute(context.Background(), execConn)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if affected != 3 {
		t.Fatalf("Execute() affected = %d, want 3", affected)
	}
	if execConn.lastExecSQL != "UPDATE users" {
		t.Fatalf("Execute() sql = %q, want SQL", execConn.lastExecSQL)
	}
}

func TestPreparedStatementPreferred(t *testing.T) {
	type row struct {
		ID int64 `db:"id"`
	}

	rows := &testRows{values: [][]any{{int64(7)}}}
	conn := &testConnection{rows: rows}
	query := &Query[row]{SQL: "SELECT id FROM users", StmtName: "GetUser"}
	if _, err := query.Get(context.Background(), conn); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if conn.lastQuerySQL != "GetUser" {
		t.Fatalf("Get() sql = %q, want prepared statement name", conn.lastQuerySQL)
	}

	execConn := &testConnection{}
	if _, err := (&Query[struct{}]{SQL: "UPDATE users", StmtName: "UpdateUsers"}).Execute(context.Background(), execConn); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if execConn.lastExecSQL != "UpdateUsers" {
		t.Fatalf("Execute() sql = %q, want prepared statement name", execConn.lastExecSQL)
	}
}
