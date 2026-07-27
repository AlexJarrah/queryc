package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	qruntime "github.com/AlexJarrah/queryc/runtime"
)

var (
	_ Connection  = (*sql.DB)(nil)
	_ Connection  = (*sql.Tx)(nil)
	_ Transaction = (*sql.Tx)(nil)
)

func TestQueryValidate(t *testing.T) {
	var nilQuery *Query[struct{}]
	if err := nilQuery.validate(); !errors.Is(err, qruntime.ErrInvalidQuery) {
		t.Fatalf("nil query error = %v, want ErrInvalidQuery", err)
	}

	empty := &Query[struct{}]{}
	if err := empty.validate(); !errors.Is(err, qruntime.ErrInvalidQuery) {
		t.Fatalf("empty query error = %v, want ErrInvalidQuery", err)
	}

	query := &Query[struct{}]{SQL: "SELECT 1"}
	if err := query.validate(); err != nil {
		t.Fatalf("query validate() error = %v", err)
	}
}

type runtimeTestDriver struct{}

func (runtimeTestDriver) Open(string) (driver.Conn, error) {
	return runtimeTestConn{}, nil
}

type runtimeTestConn struct{}

func (runtimeTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}
func (runtimeTestConn) Close() error { return nil }
func (runtimeTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (runtimeTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &runtimeTestRows{}, nil
}

func (runtimeTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(2), nil
}

type runtimeTestRows struct {
	read bool
}

func (*runtimeTestRows) Columns() []string { return []string{"id"} }
func (*runtimeTestRows) Close() error      { return nil }
func (r *runtimeTestRows) Next(values []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	values[0] = int64(9)
	return nil
}

func TestQueryExecution(t *testing.T) {
	const driverName = "queryc-runtime-test"
	sql.Register(driverName, runtimeTestDriver{})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	type row struct {
		ID int64 `db:"id"`
	}
	got, err := (&Query[row]{SQL: "SELECT id FROM users"}).GetOne(context.Background(), db)
	if err != nil {
		t.Fatalf("GetOne() error = %v", err)
	}
	if got.ID != 9 {
		t.Fatalf("GetOne() = %#v, want ID 9", got)
	}

	affected, err := (&Query[struct{}]{SQL: "UPDATE users"}).Execute(context.Background(), db)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if affected != 2 {
		t.Fatalf("Execute() affected = %d, want 2", affected)
	}
}
