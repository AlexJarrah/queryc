package mapper

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"
)

func unwrapAndAssign(dest any, scanTargets []any, mappings []*fieldMapping, hooks Hooks) error {
	destValue := reflect.ValueOf(dest).Elem()
	type pointerState struct {
		path    []int
		present bool
	}
	pointerStates := make(map[string]*pointerState)

	for i, mapping := range mappings {
		if mapping == nil {
			continue
		}
		for _, path := range pointerAncestorPaths(destValue.Type(), mapping.fieldPath) {
			key := fmt.Sprint(path)
			state, ok := pointerStates[key]
			if !ok {
				state = &pointerState{path: path}
				pointerStates[key] = state
			}
			state.present = state.present || scanTargetHasValue(scanTargets[i])
		}
	}
	nullAllowed := func(mapping *fieldMapping) bool {
		for _, path := range pointerAncestorPaths(destValue.Type(), mapping.fieldPath) {
			if state := pointerStates[fmt.Sprint(path)]; state != nil && !state.present {
				return true
			}
		}
		return false
	}
	nullError := func(mapping *fieldMapping) error {
		return fmt.Errorf("column %s is NULL but destination %s is not nullable", mapping.columnName, mapping.fieldType)
	}

	for i, target := range scanTargets {
		mapping := mappings[i]
		if mapping == nil {
			continue
		}

		fieldValue := getFieldByPath(destValue, mapping.fieldPath)
		if !fieldValue.IsValid() || !fieldValue.CanSet() {
			continue
		}

		if mapping.querycCodec == "json" {
			if err := decodeJSON(fieldValue, mapping.fieldType, mapping.columnName, target); err != nil {
				return err
			}
			continue
		}

		if hooks.IsNullableType(mapping.fieldType) {
			continue
		}
		if handled, err := hooks.AssignSpecialValue(fieldValue, target); handled {
			if err != nil {
				return err
			}
			continue
		}

		switch v := target.(type) {
		case *sql.NullString:
			if !v.Valid {
				if nullAllowed(mapping) {
					continue
				}
				return nullError(mapping)
			}
			if fieldValue.Kind() == reflect.String {
				fieldValue.SetString(v.String)
			}
		case *sql.NullInt64:
			if !v.Valid {
				if nullAllowed(mapping) {
					continue
				}
				return nullError(mapping)
			}
			switch fieldValue.Kind() {
			case reflect.Int, reflect.Int64:
				fieldValue.SetInt(v.Int64)
			case reflect.Int32, reflect.Int16, reflect.Int8:
				if fieldValue.OverflowInt(v.Int64) {
					return fmt.Errorf("failed to assign %d to %s field %s: value overflows", v.Int64, fieldValue.Type(), mapping.columnName)
				}
				fieldValue.SetInt(v.Int64)
			case reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
				if v.Int64 < 0 {
					return fmt.Errorf("failed to assign negative value to unsigned field %s", mapping.columnName)
				}
				if fieldValue.OverflowUint(uint64(v.Int64)) {
					return fmt.Errorf("failed to assign %d to %s field %s: value overflows", v.Int64, fieldValue.Type(), mapping.columnName)
				}
				fieldValue.SetUint(uint64(v.Int64))
			}
		case *sql.NullInt32:
			if !v.Valid {
				if nullAllowed(mapping) {
					continue
				}
				return nullError(mapping)
			}
			switch fieldValue.Kind() {
			case reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int, reflect.Int64:
				if fieldValue.OverflowInt(int64(v.Int32)) {
					return fmt.Errorf("failed to assign %d to %s field %s: value overflows", v.Int32, fieldValue.Type(), mapping.columnName)
				}
				fieldValue.SetInt(int64(v.Int32))
			case reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint, reflect.Uint64:
				if v.Int32 < 0 {
					return fmt.Errorf("failed to assign negative value to unsigned field %s", mapping.columnName)
				}
				if fieldValue.OverflowUint(uint64(v.Int32)) {
					return fmt.Errorf("failed to assign %d to %s field %s: value overflows", v.Int32, fieldValue.Type(), mapping.columnName)
				}
				fieldValue.SetUint(uint64(v.Int32))
			}
		case *sql.NullInt16:
			if !v.Valid {
				if nullAllowed(mapping) {
					continue
				}
				return nullError(mapping)
			}
			switch fieldValue.Kind() {
			case reflect.Int16, reflect.Int8, reflect.Int32, reflect.Int, reflect.Int64:
				if fieldValue.OverflowInt(int64(v.Int16)) {
					return fmt.Errorf("failed to assign %d to %s field %s: value overflows", v.Int16, fieldValue.Type(), mapping.columnName)
				}
				fieldValue.SetInt(int64(v.Int16))
			case reflect.Uint16, reflect.Uint8, reflect.Uint32, reflect.Uint, reflect.Uint64:
				if v.Int16 < 0 {
					return fmt.Errorf("failed to assign negative value to unsigned field %s", mapping.columnName)
				}
				if fieldValue.OverflowUint(uint64(v.Int16)) {
					return fmt.Errorf("failed to assign %d to %s field %s: value overflows", v.Int16, fieldValue.Type(), mapping.columnName)
				}
				fieldValue.SetUint(uint64(v.Int16))
			}
		case *sql.NullFloat64:
			if !v.Valid {
				if nullAllowed(mapping) {
					continue
				}
				return nullError(mapping)
			}
			switch fieldValue.Kind() {
			case reflect.Float64, reflect.Float32:
				if fieldValue.OverflowFloat(v.Float64) {
					return fmt.Errorf("failed to assign %v to %s field %s: value overflows", v.Float64, fieldValue.Type(), mapping.columnName)
				}
				fieldValue.SetFloat(v.Float64)
			}
		case *sql.NullBool:
			if !v.Valid {
				if nullAllowed(mapping) {
					continue
				}
				return nullError(mapping)
			}
			if fieldValue.Kind() == reflect.Bool {
				fieldValue.SetBool(v.Bool)
			}
		case *sql.NullTime:
			if !v.Valid {
				if nullAllowed(mapping) {
					continue
				}
				return nullError(mapping)
			}
			if fieldValue.Type() == reflect.TypeFor[time.Time]() {
				fieldValue.Set(reflect.ValueOf(v.Time))
			}
		}
	}

	// Embedded pointer result structs represent nullable joined tables. Scanning
	// requires temporary allocation. Restore nil when every mapped column from
	// that embedded value was SQL NULL.
	for _, state := range pointerStates {
		if state.present {
			continue
		}
		fieldValue := getFieldByPathWithoutAlloc(destValue, state.path)
		if fieldValue.IsValid() && fieldValue.CanSet() && fieldValue.Kind() == reflect.Pointer {
			fieldValue.SetZero()
		}
	}

	return nil
}

func pointerAncestorPaths(rootType reflect.Type, fieldPath []int) [][]int {
	for rootType.Kind() == reflect.Pointer {
		rootType = rootType.Elem()
	}

	var result [][]int
	current := rootType
	for i, fieldIndex := range fieldPath {
		if current.Kind() != reflect.Struct || fieldIndex >= current.NumField() {
			break
		}
		fieldType := current.Field(fieldIndex).Type
		if fieldType.Kind() == reflect.Pointer && i < len(fieldPath)-1 {
			path := append([]int(nil), fieldPath[:i+1]...)
			result = append(result, path)
			fieldType = fieldType.Elem()
		}
		current = fieldType
	}
	return result
}

func scanTargetHasValue(target any) bool {
	if target == nil {
		return false
	}
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return true
	}
	value = value.Elem()
	if value.Kind() == reflect.Struct {
		valid := value.FieldByName("Valid")
		if valid.IsValid() && valid.Kind() == reflect.Bool {
			return valid.Bool()
		}
	}
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return !value.IsNil()
	default:
		return true
	}
}
