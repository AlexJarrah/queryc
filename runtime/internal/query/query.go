package query

import (
	"fmt"
	"strings"

	qruntime "github.com/AlexJarrah/queryc/runtime"
)

// NilQuery returns the error used when a query pointer is nil.
func NilQuery() error {
	return fmt.Errorf("%w: query is nil", qruntime.ErrInvalidQuery)
}

// Validate checks construction errors and requires SQL or an allowed prepared
// statement name.
func Validate(sql, statement string, allowStatement bool, preset error) error {
	if preset != nil {
		return preset
	}
	if strings.TrimSpace(sql) != "" || allowStatement && strings.TrimSpace(statement) != "" {
		return nil
	}
	if allowStatement {
		return fmt.Errorf("%w: SQL and prepared statement name are empty", qruntime.ErrInvalidQuery)
	}
	return fmt.Errorf("%w: SQL is empty", qruntime.ErrInvalidQuery)
}
