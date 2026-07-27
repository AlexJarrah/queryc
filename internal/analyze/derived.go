package analyze

import (
	"maps"
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/gotype"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
	"github.com/AlexJarrah/queryc/internal/sqlscan"
)

func buildDerivedSources(schema model.Schema, sql string, d model.Dialect) map[string]map[string]model.ResultField {
	return buildDerivedSourcesWithBase(schema, sql, nil, d)
}

func buildDerivedSourcesWithBase(schema model.Schema, sql string, base map[string]map[string]model.ResultField, d model.Dialect) map[string]map[string]model.ResultField {
	sources := map[string]map[string]model.ResultField{}
	maps.Copy(sources, base)

	for _, cte := range extractCTEs(sql) {
		cleanCTE := stripMarkers(cte.SQL)
		queryTables := getQueryTables(cleanCTE)
		nullableTables, nullableAliases := getTablesWithNullableJoin(cleanCTE)
		boundSources := bindDerivedSourceAliases(cleanCTE, sources)
		fields := parseSourceColumns(schema, cte.SQL, queryTables, nullableTables, nullableAliases, boundSources, d)

		if len(fields) == 0 {
			continue
		}

		sources[cte.Name] = resultFieldMap(schema, fields, d)
	}

	mainBound := bindDerivedSourceAliases(sql, sources)
	maps.Copy(mainBound, extractSubquerySources(schema, sql, mainBound, d))
	return mainBound
}

type cteDefinition struct {
	Name string
	SQL  string
}

func extractCTEs(sql string) []cteDefinition {
	trimmed := strings.TrimSpace(sql)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "WITH ") && !strings.HasPrefix(upper, "WITH\n") && !strings.HasPrefix(upper, "WITH\t") {
		return nil
	}

	var defs []cteDefinition
	i := len("WITH")
	for i < len(trimmed) && isWhitespace(trimmed[i]) {
		i++
	}

	// Skip optional RECURSIVE keyword
	if i+9 <= len(trimmed) && strings.EqualFold(trimmed[i:i+9], "RECURSIVE") {
		next := i + 9
		if next >= len(trimmed) || isWhitespace(trimmed[next]) || trimmed[next] == '(' {
			i = next
			for i < len(trimmed) && isWhitespace(trimmed[i]) {
				i++
			}
		}
	}

	for i < len(trimmed) {
		for i < len(trimmed) && isWhitespace(trimmed[i]) {
			i++
		}
		start := i
		for i < len(trimmed) && sqlscan.IsIdentifierByte(trimmed[i]) {
			i++
		}
		if start == i {
			break
		}
		name := trimmed[start:i]
		for i < len(trimmed) && isWhitespace(trimmed[i]) {
			i++
		}

		// Optional column list: name (a, b) AS (...)
		if i < len(trimmed) && trimmed[i] == '(' {
			_, closeIndex, err := sqlscan.ExtractBalanced(trimmed, i)
			if err != nil {
				break
			}
			i = closeIndex + 1
			for i < len(trimmed) && isWhitespace(trimmed[i]) {
				i++
			}
		}
		if i+2 > len(trimmed) || !strings.EqualFold(trimmed[i:i+2], "AS") {
			break
		}
		i += 2
		for i < len(trimmed) && isWhitespace(trimmed[i]) {
			i++
		}
		if i >= len(trimmed) || trimmed[i] != '(' {
			break
		}
		body, closeIndex, err := sqlscan.ExtractBalanced(trimmed, i)
		if err != nil {
			break
		}
		defs = append(defs, cteDefinition{Name: name, SQL: strings.TrimSpace(body)})
		i = closeIndex + 1
		for i < len(trimmed) && isWhitespace(trimmed[i]) {
			i++
		}
		if i >= len(trimmed) || trimmed[i] != ',' {
			break
		}
		i++
	}
	return defs
}

func extractSubquerySources(schema model.Schema, sql string, derivedSources map[string]map[string]model.ResultField, d model.Dialect) map[string]map[string]model.ResultField {
	result := map[string]map[string]model.ResultField{}
	upper := strings.ToUpper(sql)
	depth := 0

	for i := 0; i < len(sql); i++ {
		switch sql[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth != 0 {
			continue
		}
		if !strings.HasPrefix(upper[i:], "FROM (") && !strings.HasPrefix(upper[i:], "JOIN (") && !strings.HasPrefix(upper[i:], "JOIN LATERAL (") {
			continue
		}
		open := strings.Index(sql[i:], "(")
		if open == -1 {
			continue
		}
		open += i
		body, closeIndex, err := sqlscan.ExtractBalanced(sql, open)
		if err != nil {
			continue
		}
		next := closeIndex + 1
		alias, ok := extractAliasAfterSubquery(sql, next)
		if !ok {
			i = next
			continue
		}
		cleanBody := stripMarkers(body)
		queryTables := getQueryTables(cleanBody)
		nullableTables, nullableAliases := getTablesWithNullableJoin(cleanBody)
		boundSources := bindDerivedSourceAliases(cleanBody, derivedSources)
		fields := parseSourceColumns(schema, body, queryTables, nullableTables, nullableAliases, boundSources, d)
		if len(fields) > 0 {
			result[alias] = resultFieldMap(schema, fields, d)
		}
		i = next
	}
	return result
}

func extractAliasAfterSubquery(sql string, pos int) (string, bool) {
	for pos < len(sql) && isWhitespace(sql[pos]) {
		pos++
	}

	if pos+2 <= len(sql) && strings.EqualFold(sql[pos:pos+2], "AS") {
		pos += 2
		for pos < len(sql) && isWhitespace(sql[pos]) {
			pos++
		}
	}

	start := pos
	for pos < len(sql) && sqlscan.IsIdentifierByte(sql[pos]) {
		pos++
	}

	if start == pos {
		return "", false
	}

	return sql[start:pos], true
}

func bindDerivedSourceAliases(sql string, available map[string]map[string]model.ResultField) map[string]map[string]model.ResultField {
	bound := map[string]map[string]model.ResultField{}
	maps.Copy(bound, available)

	matches := fromJoinTableRe.FindAllStringSubmatch(sql, -1)
	for _, match := range matches {
		name := match[1]
		fields, ok := available[name]
		if !ok {
			continue
		}

		alias := match[2]
		if alias == "" || isReservedAlias(alias) {
			alias = name
		}

		bound[alias] = fields
	}

	return bound
}

func resultFieldMap(schema model.Schema, fields []model.ResultField, d model.Dialect) map[string]model.ResultField {
	result := map[string]model.ResultField{}
	for _, field := range fields {
		if field.Skip {
			if field.FromColumn == "*" && field.FromTable != "" {
				table, ok := schema.Tables[field.FromTable]
				if !ok {
					continue
				}

				for columnName, column := range table.Columns {
					result[columnName] = model.ResultField{
						Name:       parse.ToPascal(columnName),
						DBName:     columnName,
						GoType:     gotype.ApplyNullability(dialect.GoTypeForSQL(d, column.SQLType), column.Nullable),
						Nullable:   column.Nullable,
						FromTable:  field.FromTable,
						FromColumn: columnName,
						TableAlias: field.TableAlias,
					}
				}
			}
			continue
		}
		result[field.DBName] = field
	}

	return result
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r'
}
