package mapper

import (
	"fmt"
	"reflect"
	"sync"
)

type structMetadata struct {
	fieldsByColumn    map[string][]*fieldMapping
	fieldsByQualified map[string]*fieldMapping
}

type fieldMapping struct {
	fieldPath   []int
	tableName   string
	columnName  string
	fieldType   reflect.Type
	querycCodec string
}

type metadataCacheKey struct {
	resultType reflect.Type
	hooksType  reflect.Type
}

// metadataCache holds reflection metadata for scanned struct types.
var metadataCache sync.Map

func getOrBuildMetadata(t reflect.Type, hooks Hooks) (*structMetadata, error) {
	if t == nil {
		return nil, fmt.Errorf("query result type must be a struct, got <nil>")
	}
	hooksType := reflect.TypeOf(hooks)
	if hooksType == nil {
		return nil, fmt.Errorf("mapper hooks must not be nil")
	}

	structType := t
	for structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
	}
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("query result type must be a struct or pointer to struct, got %s", t)
	}

	key := metadataCacheKey{resultType: t, hooksType: hooksType}
	if cached, ok := metadataCache.Load(key); ok {
		return cached.(*structMetadata), nil
	}

	meta := buildMetadata(structType, hooks)
	actual, _ := metadataCache.LoadOrStore(key, meta)
	return actual.(*structMetadata), nil
}

func buildMetadata(t reflect.Type, hooks Hooks) *structMetadata {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	meta := &structMetadata{
		fieldsByColumn:    make(map[string][]*fieldMapping),
		fieldsByQualified: make(map[string]*fieldMapping),
	}

	buildFieldMappings(t, []int{}, "", meta, hooks)
	return meta
}

func buildFieldMappings(t reflect.Type, path []int, tableName string, meta *structMetadata, hooks Hooks) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		currentPath := append(append([]int{}, path...), i)
		fieldType := field.Type
		querycCodec := field.Tag.Get("queryc")
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		if querycCodec == "" && fieldType.Kind() == reflect.Struct && !hooks.IsSpecialStruct(fieldType) {
			nestedTable := getTableNameFromField(field)
			buildFieldMappings(fieldType, currentPath, nestedTable, meta, hooks)
			continue
		}

		columnName := getColumnName(field)
		qualifiedName := getQualifiedName(field)
		mapping := &fieldMapping{
			fieldPath:   currentPath,
			tableName:   tableName,
			columnName:  columnName,
			fieldType:   field.Type,
			querycCodec: querycCodec,
		}

		meta.fieldsByColumn[columnName] = append(meta.fieldsByColumn[columnName], mapping)
		if qualifiedName != "" {
			meta.fieldsByQualified[qualifiedName] = mapping
			meta.fieldsByColumn[qualifiedName] = append(meta.fieldsByColumn[qualifiedName], mapping)
		}
		if tableName != "" && !isQualifiedName(columnName) {
			qualifiedFromTable := tableName + "__" + columnName
			meta.fieldsByQualified[qualifiedFromTable] = mapping
			meta.fieldsByColumn[qualifiedFromTable] = append(meta.fieldsByColumn[qualifiedFromTable], mapping)
		}
	}
}
