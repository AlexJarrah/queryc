package mapper

import (
	"database/sql"
	"reflect"
	"strings"
	"time"
)

var (
	standardSpecialTypes = map[reflect.Type]struct{}{
		reflect.TypeFor[time.Time]():       {},
		reflect.TypeFor[sql.NullString]():  {},
		reflect.TypeFor[sql.NullInt64]():   {},
		reflect.TypeFor[sql.NullInt32]():   {},
		reflect.TypeFor[sql.NullInt16]():   {},
		reflect.TypeFor[sql.NullFloat64](): {},
		reflect.TypeFor[sql.NullBool]():    {},
		reflect.TypeFor[sql.NullTime]():    {},
	}

	standardNullableTypes = map[reflect.Type]struct{}{
		reflect.TypeFor[sql.NullString]():  {},
		reflect.TypeFor[sql.NullInt64]():   {},
		reflect.TypeFor[sql.NullInt32]():   {},
		reflect.TypeFor[sql.NullInt16]():   {},
		reflect.TypeFor[sql.NullFloat64](): {},
		reflect.TypeFor[sql.NullBool]():    {},
		reflect.TypeFor[sql.NullTime]():    {},
		reflect.TypeFor[sql.NullByte]():    {},
	}
)

// IsStandardSpecialType returns whether t requires direct struct scanning.
func IsStandardSpecialType(t reflect.Type) bool {
	_, ok := standardSpecialTypes[t]
	return ok
}

// IsStandardNullableType returns whether t can represent SQL NULL.
func IsStandardNullableType(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		return true
	}
	_, ok := standardNullableTypes[t]
	return ok
}

func getFieldByPath(v reflect.Value, path []int) reflect.Value {
	current := v
	for _, idx := range path {
		if current.Kind() == reflect.Pointer {
			if current.IsNil() {
				current.Set(reflect.New(current.Type().Elem()))
			}
			current = current.Elem()
		}
		if idx >= current.NumField() {
			return reflect.Value{}
		}
		current = current.Field(idx)
	}
	return current
}

func getFieldByPathWithoutAlloc(v reflect.Value, path []int) reflect.Value {
	current := v
	for _, idx := range path {
		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return reflect.Value{}
			}
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct || idx >= current.NumField() {
			return reflect.Value{}
		}
		current = current.Field(idx)
	}
	return current
}

func getTableNameFromField(field reflect.StructField) string {
	if tag := field.Tag.Get("table"); tag != "" {
		return tag
	}

	typeName := field.Type.Name()
	if field.Type.Kind() == reflect.Pointer {
		typeName = field.Type.Elem().Name()
	}
	return pluralizeTableName(toSnakeCase(typeName))
}

func pluralizeTableName(snake string) string {
	if snake == "" {
		return snake
	}
	parts := strings.Split(snake, "_")
	parts[len(parts)-1] = pluralize(parts[len(parts)-1])
	return strings.Join(parts, "_")
}

func pluralize(s string) string {
	if len(s) == 0 {
		return s
	}

	if strings.HasSuffix(s, "y") && len(s) > 1 {
		prevChar := s[len(s)-2]
		if !isVowel(prevChar) {
			return s[:len(s)-1] + "ies"
		}
	}

	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") || strings.HasSuffix(s, "ch") || strings.HasSuffix(s, "sh") {
		return s + "es"
	}

	return s + "s"
}

func isVowel(c byte) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u'
}

func getColumnName(field reflect.StructField) string {
	if tag := field.Tag.Get("column"); tag != "" {
		return extractColumnFromQualified(tag)
	}
	if tag := field.Tag.Get("db"); tag != "" {
		return extractColumnFromQualified(tag)
	}
	return toSnakeCase(field.Name)
}

func getQualifiedName(field reflect.StructField) string {
	if tag := field.Tag.Get("column"); tag != "" {
		if isQualifiedName(tag) {
			return tag
		}
		return ""
	}
	if tag := field.Tag.Get("db"); tag != "" {
		if isQualifiedName(tag) {
			return tag
		}
		return ""
	}
	return ""
}

func isQualifiedName(name string) bool {
	return strings.Contains(name, "__")
}

func extractColumnFromQualified(name string) string {
	if idx := strings.LastIndex(name, "__"); idx >= 0 {
		return name[idx+2:]
	}
	return name
}

func extractTableFromQualified(name string) string {
	if idx := strings.LastIndex(name, "__"); idx >= 0 {
		return name[:idx]
	}
	return ""
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prevLower := s[i-1] >= 'a' && s[i-1] <= 'z'
			nextLower := i < len(s)-1 && s[i+1] >= 'a' && s[i+1] <= 'z'
			if prevLower || nextLower {
				result.WriteByte('_')
			}
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
