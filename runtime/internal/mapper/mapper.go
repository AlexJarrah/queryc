package mapper

import (
	"fmt"
	"reflect"
)

// Rows is the driver independent row iterator required by ScanAll.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

// Hooks provides dialect-specific nullable and special-value behavior.
type Hooks interface {
	IsNullableType(t reflect.Type) bool
	IsSpecialStruct(t reflect.Type) bool
	CreateSpecialScanTarget(fieldValue reflect.Value, fieldType reflect.Type) (any, bool)
	AssignSpecialValue(fieldValue reflect.Value, target any) (bool, error)
}

// ScanAll maps every row into T using column and struct metadata.
func ScanAll[T any](rows Rows, columnNames []string, hooks Hooks) ([]T, error) {
	var dummy T
	metadata, err := getOrBuildMetadata(reflect.TypeOf(dummy), hooks)
	if err != nil {
		return nil, err
	}

	var results []T
	for rows.Next() {
		obj := new(T)
		scanTargets, mappings := buildScanTargets(obj, columnNames, metadata, hooks)

		if err := rows.Scan(scanTargets...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		if err := unwrapAndAssign(obj, scanTargets, mappings, hooks); err != nil {
			return nil, fmt.Errorf("failed to assign values: %w", err)
		}

		results = append(results, *obj)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}
