package parse

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/utils"
)

var (
	createTableRe     = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_]+)\s*\((.*?)\);`)
	primaryKeyRe      = regexp.MustCompile(`(?i)^PRIMARY\s+KEY\s*\((.*?)\)`)
	tableConstraintRe = regexp.MustCompile(`(?i)^(FOREIGN|UNIQUE|CHECK|CONSTRAINT|EXCLUDE)\s`)
	columnDefRe       = regexp.MustCompile(`(?is)^([a-zA-Z0-9_]+)\s+(.+)$`)
)

// SchemaFile reads and parses schema SQL from a file or directory.
func SchemaFile(path string, d model.Dialect) (model.Schema, error) {
	content, err := ReadFileOrDir(path)
	if err != nil {
		return model.Schema{}, fmt.Errorf("read schema from %s: %w", path, err)
	}
	return Schema(string(content), d)
}

// Schema parses CREATE TABLE statements from content.
func Schema(content string, d model.Dialect) (model.Schema, error) {
	content = stripSQLComments(content)
	matches := createTableRe.FindAllStringSubmatch(content, -1)

	result := model.Schema{Tables: map[string]model.Table{}}
	for _, match := range matches {
		table, err := parseTable(match[1], match[2], d)
		if err != nil {
			return model.Schema{}, err
		}
		if _, exists := result.Tables[table.Name]; exists {
			return model.Schema{}, fmt.Errorf("duplicate table %q", table.Name)
		}
		result.Tables[table.Name] = table
	}

	return result, nil
}

func parseTable(name, body string, d model.Dialect) (model.Table, error) {
	table := model.Table{
		Name:    name,
		Columns: map[string]model.Column{},
	}

	lines := utils.SplitByDelimiter(body, ',')
	columnTypeKeys := schemaTypeKeys(d)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if match := primaryKeyRe.FindStringSubmatch(line); match != nil {
			for part := range strings.SplitSeq(match[1], ",") {
				table.PrimaryKeys = append(table.PrimaryKeys, strings.TrimSpace(part))
			}
			continue
		}

		if tableConstraintRe.MatchString(line) {
			continue
		}

		column, auto, hasUpdatedAt, pkInline, ok := parseColumn(line, columnTypeKeys, d)
		if !ok {
			return model.Table{}, fmt.Errorf("table %s: unsupported column or constraint definition %q", name, line)
		}
		if _, exists := table.Columns[column.Name]; exists {
			return model.Table{}, fmt.Errorf("table %s: duplicate column %q", name, column.Name)
		}
		table.Columns[column.Name] = column
		if auto {
			table.AutoFields = append(table.AutoFields, column.Name)
		}
		if hasUpdatedAt {
			table.HasUpdatedAt = true
		}
		if pkInline {
			table.PrimaryKeys = append(table.PrimaryKeys, column.Name)
		}
	}

	for _, pk := range table.PrimaryKeys {
		col, ok := table.Columns[pk]
		if !ok {
			return model.Table{}, fmt.Errorf("table %s: primary key column %q not found", name, pk)
		}
		col.Nullable = false
		table.Columns[pk] = col
	}

	table.Aliases = buildAliases(table.Name)
	return table, nil
}

func parseColumn(line string, typeKeys []string, d model.Dialect) (model.Column, bool, bool, bool, bool) {
	match := columnDefRe.FindStringSubmatch(line)
	if match == nil {
		return model.Column{}, false, false, false, false
	}

	colName := match[1]
	fullType := strings.ToUpper(match[2])
	code := executableSQL(fullType)
	baseType := resolveColumnSQLType(fullType, typeKeys, d)

	isNotNull := strings.Contains(code, "NOT NULL") || strings.Contains(code, "PRIMARY KEY")
	auto := false
	if d == model.DialectSQLite {
		auto = strings.Contains(code, "AUTOINCREMENT")
	} else {
		auto = strings.Contains(code, "GENERATED") || baseType == "SERIAL" || baseType == "BIGSERIAL" || baseType == "SMALLSERIAL"
	}

	return model.Column{
		Name:     colName,
		SQLType:  baseType,
		Nullable: !isNotNull,
	}, auto, colName == "updated_at", strings.Contains(code, "PRIMARY KEY"), true
}

func resolveColumnSQLType(fullType string, typeKeys []string, d model.Dialect) string {
	fullType = strings.TrimSpace(fullType)
	for _, key := range typeKeys {
		if !strings.HasPrefix(fullType, key) {
			continue
		}
		if len(fullType) == len(key) || isTypeBoundary(fullType[len(key)]) {
			return key
		}
	}

	fields := strings.Fields(fullType)
	if len(fields) == 0 {
		return "TEXT"
	}
	declared := strings.Trim(fields[0], "(),")
	if d != model.DialectSQLite {
		return declared
	}

	// SQLite applies type affinity by substring
	switch {
	case strings.Contains(declared, "INT"):
		return "INTEGER"
	case strings.Contains(declared, "CHAR"), strings.Contains(declared, "CLOB"), strings.Contains(declared, "TEXT"):
		return "TEXT"
	case declared == "" || strings.Contains(declared, "BLOB"):
		return "BLOB"
	// SQLite intentionally matches matches "DOUB".
	case strings.Contains(declared, "REAL"), strings.Contains(declared, "FLOA"), strings.Contains(declared, "DOUB"):
		return "REAL"
	default:
		return "NUMERIC"
	}
}

func isTypeBoundary(next byte) bool {
	return next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '('
}

func executableSQL(sql string) string {
	mask := codeMask(sql)
	var out strings.Builder
	out.Grow(len(sql))
	for i := range sql {
		if mask[i] {
			out.WriteByte(sql[i])
		} else {
			out.WriteByte(' ')
		}
	}
	return out.String()
}

func buildAliases(tableName string) []string {
	aliases := []string{tableName}
	if tableName != "" {
		first := strings.ToLower(string(tableName[0]))
		if !slices.Contains(aliases, first) {
			aliases = append(aliases, first)
		}
	}
	short := generateShortAlias(tableName)
	if short != "" && !slices.Contains(aliases, short) {
		aliases = append(aliases, short)
	}
	return aliases
}

func schemaTypeKeys(d model.Dialect) []string {
	typeMap := dialect.TypeMap(d)
	keys := make([]string, 0, len(typeMap))
	for key := range typeMap {
		keys = append(keys, key)
	}

	// Longest first so "DOUBLE PRECISION" matches before "DOUBLE".
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}

// FindTableByAlias returns a schema table name or registered alias.
func FindTableByAlias(schema model.Schema, alias string) (string, bool) {
	if _, ok := schema.Tables[alias]; ok {
		return alias, true
	}

	var match string
	for name, table := range schema.Tables {
		if slices.Contains(table.Aliases, alias) {
			if match != "" {
				return "", false
			}
			match = name
		}
	}
	return match, match != ""
}
