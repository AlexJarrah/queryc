package codegen

import (
	"slices"
	"sort"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

func sortedTables(schema model.Schema) []string {
	out := make([]string, 0, len(schema.Tables))
	for name := range schema.Tables {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sortedColumns(table model.Table) []string {
	out := make([]string, 0, len(table.Columns))
	for name := range table.Columns {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func escapeBackticks(s string) string {
	return strings.ReplaceAll(s, "`", "` + \"`\" + `")
}

func defaultableColumns(table model.Table) []string {
	cols := sortedColumns(table)
	var out []string
	for _, col := range cols {
		if !table.Columns[col].HasDefault {
			continue
		}
		if slices.Contains(table.AutoFields, col) {
			continue
		}
		if table.HasUpdatedAt && col == "updated_at" {
			continue
		}
		out = append(out, col)
	}
	return out
}

func defaultableFieldType(tableName string) string {
	return parse.ToPascal(tableName) + "DefaultableField"
}

func defaultableFieldConst(tableName, column string) string {
	return defaultableFieldType(tableName) + parse.ToPascal(column)
}

func insertColumns(table model.Table) []string {
	cols := sortedColumns(table)
	var out []string
	for _, col := range cols {
		if slices.Contains(table.AutoFields, col) {
			continue
		}
		if col == "updated_at" && table.HasUpdatedAt && !table.Columns[col].Nullable {
			continue
		}
		out = append(out, col)
	}
	return out
}

func nonPKColumns(table model.Table) []string {
	cols := sortedColumns(table)
	var out []string
	for _, col := range cols {
		if slices.Contains(table.PrimaryKeys, col) {
			continue
		}
		out = append(out, col)
	}
	return out
}

func setUpdateColumns(table model.Table) []string {
	cols := sortedColumns(table)
	var out []string
	for _, col := range cols {
		if slices.Contains(table.PrimaryKeys, col) {
			continue
		}
		if slices.Contains(table.AutoFields, col) {
			continue
		}
		if col == "created_at" {
			continue
		}
		out = append(out, col)
	}
	return out
}
