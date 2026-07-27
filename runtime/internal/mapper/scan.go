package mapper

import (
	"database/sql"
	"reflect"
	"time"
)

func buildScanTargets(dest any, columnNames []string, meta *structMetadata, hooks Hooks) ([]any, []*fieldMapping) {
	destValue := reflect.ValueOf(dest).Elem()
	targets := make([]any, len(columnNames))
	mappings := make([]*fieldMapping, len(columnNames))
	usedTables := make(map[string]map[string]bool)

	for i, colName := range columnNames {
		mapping := findBestMapping(colName, meta, usedTables)
		if mapping == nil {
			var dummy any
			targets[i] = &dummy
			continue
		}

		mappings[i] = mapping
		if mapping.tableName != "" {
			if usedTables[mapping.tableName] == nil {
				usedTables[mapping.tableName] = make(map[string]bool)
			}
			usedTables[mapping.tableName][colName] = true
			if mapping.columnName != "" {
				usedTables[mapping.tableName][mapping.columnName] = true
			}
		}

		fieldValue := getFieldByPath(destValue, mapping.fieldPath)
		if !fieldValue.IsValid() {
			var dummy any
			targets[i] = &dummy
			continue
		}

		targets[i] = createScanTarget(fieldValue, mapping.fieldType, mapping.querycCodec, hooks)
	}

	return targets, mappings
}

func findBestMapping(colName string, meta *structMetadata, usedTables map[string]map[string]bool) *fieldMapping {
	colTable := extractTableFromQualified(colName)
	colColumn := extractColumnFromQualified(colName)

	if colTable != "" {
		if mapping, ok := meta.fieldsByQualified[colName]; ok {
			return mapping
		}
	}

	if mapping, ok := meta.fieldsByQualified[colColumn]; ok {
		if colTable == "" || mapping.tableName == colTable {
			if !isTableUsedForColumn(usedTables, mapping.tableName, colColumn) {
				return mapping
			}
		}
	}

	candidates := meta.fieldsByColumn[colColumn]
	if len(candidates) == 0 {
		candidates = meta.fieldsByColumn[colName]
		if len(candidates) == 0 {
			return nil
		}
	}

	for _, candidate := range candidates {
		if candidate.tableName == "" {
			continue
		}
		if isTableUsedForColumn(usedTables, candidate.tableName, colColumn) {
			continue
		}
		if colTable != "" && candidate.tableName != colTable {
			continue
		}
		return candidate
	}

	for _, candidate := range candidates {
		if candidate.tableName == colTable {
			return candidate
		}
	}
	for _, candidate := range candidates {
		if candidate.tableName != "" {
			return candidate
		}
	}
	return candidates[0]
}

func isTableUsedForColumn(usedTables map[string]map[string]bool, tableName, columnName string) bool {
	if usedTables[tableName] == nil {
		return false
	}
	return usedTables[tableName][columnName]
}

func createScanTarget(fieldValue reflect.Value, fieldType reflect.Type, querycCodec string, hooks Hooks) any {
	if querycCodec == "json" {
		raw := []byte(nil)
		return &raw
	}

	if hooks.IsNullableType(fieldType) {
		return fieldValue.Addr().Interface()
	}
	if special, ok := hooks.CreateSpecialScanTarget(fieldValue, fieldType); ok {
		return special
	}

	switch fieldType.Kind() {
	case reflect.String:
		return &sql.NullString{}
	case reflect.Int, reflect.Int64:
		return &sql.NullInt64{}
	case reflect.Int32:
		return &sql.NullInt32{}
	case reflect.Int16:
		return &sql.NullInt16{}
	case reflect.Float64, reflect.Float32:
		return &sql.NullFloat64{}
	case reflect.Bool:
		return &sql.NullBool{}
	case reflect.Struct:
		if fieldType == reflect.TypeFor[time.Time]() {
			return &sql.NullTime{}
		}
	}

	return fieldValue.Addr().Interface()
}
