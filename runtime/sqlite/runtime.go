package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/AlexJarrah/queryc/runtime"
	"github.com/AlexJarrah/queryc/runtime/internal/mapper"
	querybase "github.com/AlexJarrah/queryc/runtime/internal/query"
)

// Query is an executable generated SQLite query returning T.
type Query[T any] struct {
	SQL      string
	StmtName string
	Args     []any
	Error    error
}

// Connection is the database/sql behavior required by generated queries.
type Connection interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Transaction is a transactional Connection.
type Transaction interface {
	Connection
	Commit() error
	Rollback() error
}

// Retype returns a Query with a result type of a compatible shape.
func Retype[To any, From any](q *Query[From]) *Query[To] {
	if q == nil {
		return nil
	}
	return &Query[To]{
		SQL:      q.SQL,
		StmtName: q.StmtName,
		Args:     q.Args,
		Error:    q.Error,
	}
}

// Get executes q and maps every returned row.
func (q *Query[T]) Get(ctx context.Context, c Connection) ([]T, error) {
	if err := q.validate(); err != nil {
		return nil, err
	}

	rows, err := c.QueryContext(ctx, q.SQL, q.Args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	return mapper.ScanAll[T](rows, columnNames, mapper.StandardHooks{})
}

// GetOne executes the query and enforces only a single result is returned.
func (q *Query[T]) GetOne(ctx context.Context, c Connection) (*T, error) {
	results, err := q.Get(ctx, c)
	if err != nil {
		return nil, err
	}

	switch len(results) {
	case 0:
		return nil, runtime.ErrNoResults
	case 1:
		return &results[0], nil
	default:
		return nil, runtime.ErrTooManyResults
	}
}

// Execute runs q and returns the affected row count.
func (q *Query[T]) Execute(ctx context.Context, c Connection) (int64, error) {
	if err := q.validate(); err != nil {
		return 0, err
	}

	result, err := c.ExecContext(ctx, q.SQL, q.Args...)
	if err != nil {
		return 0, fmt.Errorf("execute failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to read rows affected: %w", err)
	}
	return rowsAffected, nil
}

func (q *Query[T]) validate() error {
	if q == nil {
		return querybase.NilQuery()
	}
	return querybase.Validate(q.SQL, "", false, q.Error)
}
