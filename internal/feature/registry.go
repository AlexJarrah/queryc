package feature

import (
	"fmt"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/sqlscan"
)

// Handler rewrites a feature invocation.
type Handler func(content string, dialect model.Dialect) (string, error)

// Registry stores feature handlers by name.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry returns a registry containing queryc's built-in features.
func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{
		"SLICE":  sliceHandler,
		"STRUCT": structHandler,
	}}
}

// Convert scans `sql` and replaces feature calls with the returned SQL.
func (r *Registry) Convert(sql string, dialect model.Dialect) (string, error) {
	result := sql
	for {
		start, name, open := findFeatureCall(result)
		if start == -1 {
			return result, nil
		}

		content, closeIndex, err := sqlscan.ExtractBalanced(result, open)
		if err != nil {
			return "", err
		}

		handler, ok := r.handlers[strings.ToUpper(name)]
		if !ok {
			return "", fmt.Errorf("unknown feature @%s", name)
		}

		inner, err := r.Convert(content, dialect)
		if err != nil {
			return "", err
		}

		replacement, err := handler(inner, dialect)
		if err != nil {
			return "", err
		}
		result = result[:start] + replacement + result[closeIndex+1:]
	}
}

// findFeatureCall locates the next @IDENT( in sql and returns its start
// position, the feature name, and the index of the opening '('.
func findFeatureCall(sql string) (int, string, int) {
	for i := 0; i < len(sql); i++ {
		if next, ok := sqlscan.SkipLiteralOrComment(sql, i); ok {
			i = next - 1
			continue
		}
		if sql[i] != '@' {
			continue
		}
		j := i + 1
		for j < len(sql) && sqlscan.IsIdentifierByte(sql[j]) {
			j++
		}
		if j >= len(sql) || sql[j] != '(' {
			continue
		}
		return i, sql[i+1 : j], j
	}
	return -1, "", -1
}
