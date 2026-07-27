package analyze

import (
	"fmt"
	"strings"

	"github.com/AlexJarrah/queryc/internal/gotype"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
	"github.com/AlexJarrah/queryc/internal/sqlscan"
	"github.com/AlexJarrah/queryc/internal/utils"
)

func parseSelectColumns(schema model.Schema, sql string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool, derivedSources map[string]map[string]model.ResultField, d model.Dialect) ([]model.ResultField, error) {
	return parseSelectColumnsWithDiagnostics(schema, sql, queryTables, nullableTables, nullableAliases, derivedSources, d, true)
}

func parseSourceColumns(schema model.Schema, sql string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool, derivedSources map[string]map[string]model.ResultField, d model.Dialect) []model.ResultField {
	fields, _ := parseSelectColumnsWithDiagnostics(schema, sql, queryTables, nullableTables, nullableAliases, derivedSources, d, false)
	return fields
}

func parseSelectColumnsWithDiagnostics(schema model.Schema, sql string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool, derivedSources map[string]map[string]model.ResultField, d model.Dialect, strict bool) ([]model.ResultField, error) {
	selectClause := extractMainSelectClause(sql)
	if selectClause == "" {
		return nil, nil
	}

	grouped := hasTopLevelGroupBy(sql)
	if strings.TrimSpace(stripMarkers(selectClause)) == "*" {
		var fromTable, tableAlias string
		if len(queryTables) > 0 {
			fromTable = queryTables[0][0]
			tableAlias = queryTables[0][1]
		}

		return []model.ResultField{{
			Name:       "*",
			DBName:     "*",
			GoType:     "any",
			Skip:       true,
			FromTable:  fromTable,
			FromColumn: "*",
			TableAlias: tableAlias,
		}}, nil
	}

	selectClause = stripDistinctClause(selectClause)
	rawColumns := utils.SplitByDelimiter(selectClause, ',')
	fields := make([]model.ResultField, 0, len(rawColumns))

	for _, raw := range rawColumns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		jsonSerialized := strings.Contains(raw, "/*queryc_json*/")
		cleanRaw := stripMarkers(raw)
		expr, explicitType := extractTypeAnnotation(cleanRaw)
		alias := ""

		if match := aliasExprRe.FindStringSubmatch(expr); match != nil {
			expr = strings.TrimSpace(match[1])
			alias = match[2]
		}

		field := parseExpression(schema, expr, cleanRaw, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
		if alias != "" {
			field.Name = parse.ToPascal(alias)
			field.DBName = alias
		}

		if explicitType != "" {
			field.GoType = gotype.ApplyNullability(convertExplicitType(explicitType, d), field.Nullable)
			field.ExplicitType = field.GoType
		} else if jsonSerialized {
			if inferred, ok := inferJSONField(schema, expr, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d); ok {
				field.GoType = gotype.ApplyNullability(inferred.GoType, inferred.Nullable)
				field.Nullable = inferred.Nullable
				field.ExplicitType = gotype.ApplyNullability(inferred.ExplicitType, inferred.Nullable)
				field.Serialization = "json"
				field.GeneratedStructName = inferred.GeneratedStructName
				field.GeneratedStructKind = inferred.GeneratedStructKind
				field.GeneratedFields = inferred.GeneratedFields
			} else {
				field.Serialization = "json"
			}
		}

		if jsonSerialized && field.Serialization == "" {
			field.Serialization = "json"
		}

		field.GoType = gotype.ApplyNullability(field.GoType, field.Nullable)
		if strict && !field.Skip && field.GeneratedStructKind == "" && explicitType == "" && strings.TrimPrefix(field.GoType, "*") == "any" {
			fieldName := field.DBName
			if fieldName == "" {
				fieldName = field.Name
			}
			return nil, fmt.Errorf("cannot infer Go type for selected field %q (expression %q); add an explicit type annotation", fieldName, expr)
		}
		fields = append(fields, field)
	}

	return fields, nil
}

func shouldGenerateStruct(name string, fields []model.ResultField, rawSQL string) (bool, string) {
	if !producesResultSet(rawSQL) {
		return false, ""
	}
	if len(fields) == 0 {
		return false, ""
	}
	return true, name + "Result"
}

// producesResultSet returns whether the SQL returns a typed result set.
func producesResultSet(rawSQL string) bool {
	trimmed := strings.TrimSpace(rawSQL)
	if trimmed == "" {
		return false
	}
	upper := strings.ToUpper(trimmed)
	if strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "WITH") {
		return true
	}
	return containsTopLevelKeyword(trimmed, "RETURNING")
}

func extractMainSelectClause(sql string) string {
	if clause := extractSelectList(sql); clause != "" {
		return clause
	}
	return extractReturningList(sql)
}

func extractSelectList(sql string) string {
	upper := strings.ToUpper(sql)
	selectPos := -1
	endPos := -1
	depth := 0

	for i := 0; i < len(sql); i++ {
		next, skipped := sqlscan.SkipLiteralOrComment(sql, i)
		if skipped {
			i = next - 1
			continue
		}
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
		if selectPos == -1 && isKeywordAt(upper, i, "SELECT") {
			selectPos = i + len("SELECT")
			i += len("SELECT") - 1
			continue
		}
		if selectPos == -1 {
			continue
		}
		if isKeywordAt(upper, i, "FROM") ||
			isKeywordAt(upper, i, "WHERE") ||
			isKeywordAt(upper, i, "GROUP") ||
			isKeywordAt(upper, i, "ORDER") ||
			isKeywordAt(upper, i, "LIMIT") ||
			isKeywordAt(upper, i, "HAVING") ||
			isKeywordAt(upper, i, "UNION") ||
			isKeywordAt(upper, i, "INTERSECT") ||
			isKeywordAt(upper, i, "EXCEPT") ||
			isKeywordAt(upper, i, "FETCH") ||
			isKeywordAt(upper, i, "OFFSET") ||
			isKeywordAt(upper, i, "FOR") ||
			isKeywordAt(upper, i, "WINDOW") {
			endPos = i
			break
		}
		if sql[i] == ';' {
			endPos = i
			break
		}
	}
	if selectPos == -1 {
		return ""
	}
	if endPos == -1 {
		endPos = len(sql)
	}
	if endPos < selectPos {
		return ""
	}
	return strings.TrimSpace(sql[selectPos:endPos])
}

func extractReturningList(sql string) string {
	upper := strings.ToUpper(sql)
	retPos := -1
	depth := 0

	for i := 0; i < len(sql); i++ {
		next, skipped := sqlscan.SkipLiteralOrComment(sql, i)
		if skipped {
			i = next - 1
			continue
		}
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
		if isKeywordAt(upper, i, "RETURNING") {
			retPos = i + len("RETURNING")
			break
		}
	}
	if retPos == -1 {
		return ""
	}
	endPos := len(sql)
	for i := retPos; i < len(sql); i++ {
		next, skipped := sqlscan.SkipLiteralOrComment(sql, i)
		if skipped {
			i = next - 1
			continue
		}
		if sql[i] == ';' {
			endPos = i
			break
		}
	}
	return strings.TrimSpace(sql[retPos:endPos])
}

func containsTopLevelKeyword(sql, keyword string) bool {
	upper := strings.ToUpper(sql)
	depth := 0

	for i := 0; i < len(sql); i++ {
		next, skipped := sqlscan.SkipLiteralOrComment(sql, i)
		if skipped {
			i = next - 1
			continue
		}
		switch sql[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && isKeywordAt(upper, i, keyword) {
			return true
		}
	}
	return false
}

func isKeywordAt(upper string, i int, keyword string) bool {
	if !strings.HasPrefix(upper[i:], keyword) {
		return false
	}

	end := i + len(keyword)
	if i > 0 {
		prev := upper[i-1]
		if (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9') || prev == '_' {
			return false
		}
	}

	if end < len(upper) {
		next := upper[end]
		if (next >= 'A' && next <= 'Z') || (next >= '0' && next <= '9') || next == '_' {
			return false
		}
	}

	return true
}

func hasTopLevelGroupBy(sql string) bool {
	return containsTopLevelKeyword(sql, "GROUP BY")
}

func fieldNameForExpr(rawExpr, expr string) string {
	if match := asAliasRe.FindStringSubmatch(rawExpr); match != nil {
		return match[1]
	}

	clean := strings.TrimSpace(stripMarkers(expr))
	if idx := strings.IndexAny(clean, " \t("); idx >= 0 {
		clean = clean[:idx]
	}

	return strings.ReplaceAll(clean, ".", "_")
}

func dbNameForExpr(rawExpr, expr string) string {
	if match := asAliasRe.FindStringSubmatch(rawExpr); match != nil {
		return match[1]
	}
	return fieldNameForExpr(rawExpr, expr)
}

func extractTypeAnnotation(raw string) (string, string) {
	depth := 0

	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth != 0 {
				continue
			}
			if i+1 < len(raw) && raw[i+1] == ':' {
				i++
				continue
			}
			remaining := strings.TrimSpace(raw[i+1:])
			if remaining == "" {
				continue
			}
			typeName := remaining
			if idx := strings.IndexAny(typeName, " \t"); idx >= 0 {
				typeName = typeName[:idx]
			}
			return strings.TrimSpace(raw[:i]), strings.TrimRight(typeName, ")")
		}
	}
	return raw, ""
}

func stripMarkers(sql string) string {
	return parse.StripQueryCMarkers(sql)
}

func stripDistinctClause(s string) string {
	return stripLeadingDistinct(s)
}

func stripLeadingDistinct(s string) string {
	trimmed := strings.TrimSpace(s)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "DISTINCT") {
		return trimmed
	}

	rest := trimmed[len("DISTINCT"):]
	if rest == "" {
		return ""
	}

	if rest[0] != ' ' && rest[0] != '\t' && rest[0] != '\n' && rest[0] != '\r' {
		// e.g. "DISTINCTION" != "DISTINCT"
		if (rest[0] >= 'A' && rest[0] <= 'Z') || (rest[0] >= 'a' && rest[0] <= 'z') || rest[0] == '_' {
			return trimmed
		}
	}

	return strings.TrimSpace(rest)
}
