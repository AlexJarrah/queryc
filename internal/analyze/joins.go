package analyze

import (
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
	"github.com/AlexJarrah/queryc/internal/utils"
)

func detectEmbeddedTables(schema model.Schema, sql string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool) []model.EmbeddedTable {
	var result []model.EmbeddedTable
	seen := map[string]bool{}
	selectClause := extractMainSelectClause(sql)

	for _, part := range utils.SplitByDelimiter(selectClause, ',') {
		part = strings.TrimSpace(stripMarkers(part))
		match := starSelectRe.FindStringSubmatch(part)
		if match == nil {
			continue
		}

		alias := match[1]
		tableName, ok := lookupQueryAlias(schema, queryTables, alias)
		if !ok || seen[tableName] {
			continue
		}

		seen[tableName] = true
		structName := parse.ToSingular(parse.ToPascal(tableName))
		result = append(result, model.EmbeddedTable{
			TableName:  tableName,
			StructName: structName,
			IsNullable: nullableTables[tableName] || nullableAliases[alias],
			TableAlias: alias,
		})
	}

	return result
}

var sqlReservedAliasWords = map[string]bool{
	"WHERE": true, "JOIN": true, "ON": true, "GROUP": true, "ORDER": true,
	"LIMIT": true, "HAVING": true, "UNION": true, "INTERSECT": true, "EXCEPT": true,
	"RETURNING": true, "SET": true, "INTO": true, "VALUES": true, "FROM": true,
	"SELECT": true, "WITH": true, "AS": true, "AND": true, "OR": true, "NOT": true,
	"LEFT": true, "RIGHT": true, "FULL": true, "INNER": true, "OUTER": true,
	"CROSS": true, "NATURAL": true, "USING": true, "WINDOW": true, "FETCH": true,
	"OFFSET": true, "FOR": true, "CASE": true, "WHEN": true, "THEN": true,
	"ELSE": true, "END": true, "DISTINCT": true, "ALL": true, "LATERAL": true,
	"RECURSIVE": true, "ASC": true, "DESC": true, "BY": true, "NULL": true,
	"TRUE": true, "FALSE": true, "IS": true, "IN": true, "BETWEEN": true,
	"LIKE": true, "ILIKE": true, "EXISTS": true,
}

func isReservedAlias(alias string) bool {
	return sqlReservedAliasWords[strings.ToUpper(alias)]
}

func getQueryTables(sql string) [][2]string {
	var result [][2]string
	masked := maskNestedSQL(sql)
	matches := fromJoinTableRe.FindAllStringSubmatch(masked, -1)

	for _, match := range matches {
		table := match[1]
		alias := match[2]
		if alias == "" || isReservedAlias(alias) {
			alias = table
		}
		result = append(result, [2]string{table, alias})
	}

	return result
}

func getTablesWithNullableJoin(sql string) (map[string]bool, map[string]bool) {
	tables := map[string]bool{}
	aliases := map[string]bool{}
	masked := maskNestedSQL(sql)

	// Track tables/aliases in order so RIGHT & FULL JOINs can mark the
	// preserved-side opposite correctly.
	var seenOrder [][2]string
	seenSet := map[string]bool{}
	addSeen := func(table, alias string) {
		key := table + "\x00" + alias
		if seenSet[key] {
			return
		}
		seenSet[key] = true
		seenOrder = append(seenOrder, [2]string{table, alias})
	}

	fromMatches := fromTableRe.FindAllStringSubmatch(masked, -1)
	for _, match := range fromMatches {
		table := match[1]
		alias := match[2]
		if alias == "" || isReservedAlias(alias) {
			alias = table
		}
		addSeen(table, alias)
	}

	joinRe := joinKindTableRe
	for _, match := range joinRe.FindAllStringSubmatch(masked, -1) {
		joinKind := strings.ToUpper(strings.TrimSpace(match[1]))
		table := match[2]
		alias := match[3]
		if alias == "" || isReservedAlias(alias) {
			alias = table
		}

		isLeft := strings.HasPrefix(joinKind, "LEFT")
		isRight := strings.HasPrefix(joinKind, "RIGHT")
		isFull := strings.HasPrefix(joinKind, "FULL")

		if isLeft || isFull {
			if table != "" {
				tables[table] = true
			}
			aliases[alias] = true
		}

		if isRight || isFull {
			for _, prev := range seenOrder {
				if prev[0] != "" {
					tables[prev[0]] = true
				}
				aliases[prev[1]] = true
			}
		}

		addSeen(table, alias)
	}
	return tables, aliases
}

func maskNestedSQL(sql string) string {
	var out strings.Builder
	out.Grow(len(sql))
	depth := 0
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		switch ch {
		case '(':
			depth++
			out.WriteByte(ch)
		case ')':
			if depth > 0 {
				depth--
			}
			out.WriteByte(ch)
		default:
			if depth > 0 {
				if ch == '\n' || ch == '\t' || ch == '\r' {
					out.WriteByte(ch)
				} else {
					out.WriteByte(' ')
				}
				continue
			}
			out.WriteByte(ch)
		}
	}
	return out.String()
}

func findUniqueColumn(schema model.Schema, columnName string) (model.Column, bool) {
	var (
		result model.Column
		found  bool
	)

	for _, table := range schema.Tables {
		col, ok := table.Columns[columnName]
		if !ok {
			continue
		}
		if found {
			return model.Column{}, false
		}

		result = col
		found = true
	}
	return result, found
}
