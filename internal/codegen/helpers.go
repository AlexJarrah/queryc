package codegen

import (
	"sort"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
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
