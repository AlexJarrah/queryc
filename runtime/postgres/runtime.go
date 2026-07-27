package postgres

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/AlexJarrah/queryc/internal/utils"
	queryc "github.com/AlexJarrah/queryc/runtime"
	"github.com/AlexJarrah/queryc/runtime/internal/mapper"
	querybase "github.com/AlexJarrah/queryc/runtime/internal/query"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Query is an executable generated PostgreSQL query returning T.
type Query[T any] struct {
	SQL      string
	StmtName string
	Args     []any
	Error    error
}

// Connection is the pgx behavior required to execute generated queries.
type Connection interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Preparer is the pgx behavior required by generated PrepareStatements.
type Preparer interface {
	Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error)
}

// Transaction is a transactional Connection.
type Transaction interface {
	Connection
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
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

	// Prefer statement name, falling back to SQL if unavailable.
	rows, err := c.Query(ctx, utils.Coalesce(q.StmtName, q.SQL), q.Args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	columnNames := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		columnNames[i] = fd.Name
	}

	return mapper.ScanAll[T](rows, columnNames, postgresHooks{})
}

// GetOne executes the query and enforces only a single result is returned.
func (q *Query[T]) GetOne(ctx context.Context, c Connection) (*T, error) {
	results, err := q.Get(ctx, c)
	if err != nil {
		return nil, err
	}

	switch len(results) {
	case 0:
		return nil, queryc.ErrNoResults
	case 1:
		return &results[0], nil
	default:
		return nil, queryc.ErrTooManyResults
	}
}

// Execute runs q and returns the affected row count.
func (q *Query[T]) Execute(ctx context.Context, c Connection) (int64, error) {
	if err := q.validate(); err != nil {
		return 0, err
	}

	cmdTag, err := c.Exec(ctx, utils.Coalesce(q.StmtName, q.SQL), q.Args...)
	if err != nil {
		return 0, fmt.Errorf("execute failed: %w", err)
	}
	return cmdTag.RowsAffected(), nil
}

func (q *Query[T]) validate() error {
	if q == nil {
		return querybase.NilQuery()
	}
	return querybase.Validate(q.SQL, q.StmtName, true, q.Error)
}

type postgresHooks struct{}

func (postgresHooks) IsNullableType(t reflect.Type) bool {
	return mapper.IsStandardNullableType(t) || strings.Contains(t.String(), "pgtype.")
}

func (postgresHooks) IsSpecialStruct(t reflect.Type) bool {
	return mapper.IsStandardSpecialType(t) || t == reflect.TypeFor[pgtype.UUID]()
}

func (postgresHooks) CreateSpecialScanTarget(_ reflect.Value, fieldType reflect.Type) (any, bool) {
	if fieldType == reflect.TypeFor[pgtype.UUID]() {
		return &pgtype.UUID{}, true
	}
	return nil, false
}

func (postgresHooks) AssignSpecialValue(fieldValue reflect.Value, target any) (bool, error) {
	uuidTarget, ok := target.(*pgtype.UUID)
	if !ok {
		return false, nil
	}
	if uuidTarget.Valid && fieldValue.Type() == reflect.TypeFor[pgtype.UUID]() {
		fieldValue.Set(reflect.ValueOf(*uuidTarget))
	}
	return true, nil
}
