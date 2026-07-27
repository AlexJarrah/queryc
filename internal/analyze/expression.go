package analyze

import (
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/gotype"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
	"github.com/AlexJarrah/queryc/internal/utils"
)

func parseExpression(schema model.Schema, expr, rawExpr string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool, derivedSources map[string]map[string]model.ResultField, grouped bool, d model.Dialect) model.ResultField {
	expr = strings.TrimSpace(expr)
	typeMap := dialect.TypeMap(d)

	if match := aliasExprRe.FindStringSubmatch(rawExpr); match != nil {
		inner := strings.TrimSpace(match[1])
		inner, explicitType := extractTypeAnnotation(inner)
		alias := match[2]
		innerField := parseExpression(schema, inner, inner, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)

		if explicitType != "" {
			innerField.GoType = explicitType
			innerField.ExplicitType = explicitType
		}

		innerField.Name = parse.ToPascal(alias)
		innerField.DBName = alias
		return innerField
	}

	if match := qualifiedColRe.FindStringSubmatch(expr); match != nil {
		tableAlias := match[1]
		columnName := match[2]

		if columnName == "*" {
			tableName, _ := lookupQueryAlias(schema, queryTables, tableAlias)
			return model.ResultField{
				Name:       "*",
				DBName:     "*",
				GoType:     "any",
				Skip:       true,
				FromTable:  tableName,
				FromColumn: "*",
				TableAlias: tableAlias,
			}
		}

		if tableName, ok := lookupQueryAlias(schema, queryTables, tableAlias); ok {
			if col, ok := schema.Tables[tableName].Columns[columnName]; ok {
				nullable := col.Nullable || nullableTables[tableName] || nullableAliases[tableAlias]
				goType := gotype.ApplyNullability(dialect.GoTypeForSQL(d, col.SQLType), nullable)
				return model.ResultField{
					Name:       parse.ToPascal(columnName),
					DBName:     columnName,
					GoType:     goType,
					Nullable:   nullable,
					FromTable:  tableName,
					FromColumn: columnName,
					TableAlias: tableAlias,
				}
			}
		}

		if sourceColumns, ok := derivedSources[tableAlias]; ok {
			if sourceField, ok := sourceColumns[columnName]; ok {
				nullable := sourceField.Nullable || nullableAliases[tableAlias]
				return model.ResultField{
					Name:       parse.ToPascal(columnName),
					DBName:     columnName,
					GoType:     gotype.ApplyNullability(strings.TrimPrefix(sourceField.GoType, "*"), nullable),
					Nullable:   nullable,
					FromTable:  sourceField.FromTable,
					FromColumn: sourceField.FromColumn,
					TableAlias: tableAlias,
				}
			}
		}

		if col, ok := findUniqueColumn(schema, columnName); ok {
			nullable := col.Nullable || nullableAliases[tableAlias]
			return model.ResultField{
				Name:       parse.ToPascal(columnName),
				DBName:     columnName,
				GoType:     gotype.ApplyNullability(dialect.GoTypeForSQL(d, col.SQLType), nullable),
				Nullable:   nullable,
				FromColumn: columnName,
				TableAlias: tableAlias,
			}
		}

		nullable := nullableAliases[tableAlias]
		return model.ResultField{
			Name:       parse.ToPascal(columnName),
			DBName:     columnName,
			GoType:     gotype.ApplyNullability("any", nullable),
			Nullable:   nullable,
			FromColumn: columnName,
			TableAlias: tableAlias,
		}
	}

	if match := simpleIdentRe.FindStringSubmatch(expr); match != nil {
		columnName := match[1]
		for _, pair := range queryTables {
			tableName := pair[0]
			tableAlias := pair[1]

			if col, ok := schema.Tables[tableName].Columns[columnName]; ok {
				nullable := col.Nullable || nullableTables[tableName] || nullableAliases[tableAlias]
				goType := gotype.ApplyNullability(dialect.GoTypeForSQL(d, col.SQLType), nullable)
				return model.ResultField{
					Name:       parse.ToPascal(columnName),
					DBName:     columnName,
					GoType:     goType,
					Nullable:   nullable,
					FromTable:  tableName,
					FromColumn: columnName,
					TableAlias: tableAlias,
				}
			}
		}

		var (
			sourceField model.ResultField
			found       bool
		)

		for _, columns := range derivedSources {
			field, ok := columns[columnName]
			if !ok {
				continue
			}

			if found {
				found = false
				break
			}

			sourceField = field
			found = true
		}

		if found {
			return model.ResultField{
				Name:       parse.ToPascal(columnName),
				DBName:     columnName,
				GoType:     sourceField.GoType,
				Nullable:   sourceField.Nullable,
				FromTable:  sourceField.FromTable,
				FromColumn: sourceField.FromColumn,
				TableAlias: sourceField.TableAlias,
			}
		}
	}

	if match := castFuncRe.FindStringSubmatch(expr); match != nil {
		inner := parseExpression(schema, strings.TrimSpace(match[1]), strings.TrimSpace(match[1]), queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
		sqlType := strings.ToUpper(strings.TrimSpace(match[2]))
		return model.ResultField{
			Name:         parse.ToPascal(fieldNameForExpr(rawExpr, match[1])),
			DBName:       dbNameForExpr(rawExpr, match[1]),
			GoType:       gotype.ApplyNullability(typeMap[sqlType], inner.Nullable),
			Nullable:     inner.Nullable,
			IsExpression: true,
		}
	}

	if match := pgCastRe.FindStringSubmatch(expr); match != nil {
		inner := parseExpression(schema, strings.TrimSpace(match[1]), strings.TrimSpace(match[1]), queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
		return model.ResultField{
			Name:         parse.ToPascal(fieldNameForExpr(rawExpr, match[1])),
			DBName:       dbNameForExpr(rawExpr, match[1]),
			GoType:       gotype.ApplyNullability(typeMap[strings.ToUpper(match[2])], inner.Nullable),
			Nullable:     inner.Nullable,
			IsExpression: true,
		}
	}

	if numberLiteralRe.MatchString(expr) {
		goType := "int64"
		if strings.Contains(expr, ".") {
			goType = "float64"
		}
		return literalResultField(rawExpr, expr, goType)
	}
	if stringLiteralRe.MatchString(expr) {
		return literalResultField(rawExpr, expr, "string")
	}
	if strings.EqualFold(expr, "TRUE") || strings.EqualFold(expr, "FALSE") {
		return literalResultField(rawExpr, expr, "bool")
	}

	if match := funcCallRe.FindStringSubmatch(expr); match != nil {
		name := strings.ToUpper(match[1])
		goType := "any"
		nullable := true

		switch name {
		case "COUNT", "ROW_NUMBER", "RANK", "DENSE_RANK":
			goType = "int64"
			nullable = false
		case "SUM":
			goType = "int64"
			nullable = true
			if args, ok := extractFunctionArgs(expr, []string{"sum"}); ok {
				inner := strings.TrimSpace(args)
				inner = stripLeadingDistinct(inner)
				field := parseExpression(schema, inner, inner, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
				nullable = !grouped || field.Nullable
			}
		case "AVG":
			goType = "float64"
			nullable = true
			if args, ok := extractFunctionArgs(expr, []string{"avg"}); ok {
				inner := strings.TrimSpace(args)
				inner = stripLeadingDistinct(inner)
				field := parseExpression(schema, inner, inner, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
				nullable = !grouped || field.Nullable
			}
		case "MAX", "MIN":
			goType = "any"
			nullable = true
			if args, ok := extractFunctionArgs(expr, []string{strings.ToLower(name)}); ok {
				inner := strings.TrimSpace(args)
				inner = stripLeadingDistinct(inner)
				field := parseExpression(schema, inner, inner, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
				goType = strings.TrimPrefix(field.GoType, "*")
				if goType == "" {
					goType = "any"
				}
				nullable = !grouped || field.Nullable
			}
		case "DATE", "DATETIME", "TIME", "TIMESTAMP":
			goType = "time.Time"
			nullable = true
			if args, ok := extractFunctionArgs(expr, []string{strings.ToLower(name)}); ok {
				inner := strings.TrimSpace(args)
				field := parseExpression(schema, inner, inner, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
				nullable = !grouped || field.Nullable
			}
		case "STRING_AGG", "GROUP_CONCAT":
			goType = "string"
			nullable = true
		case "COALESCE":
			goType, nullable = inferCoalesceType(schema, expr, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
		case "JSON_AGG", "JSONB_AGG", "JSON_GROUP_ARRAY", "JSON_BUILD_OBJECT", "JSON_OBJECT":
			goType = "[]byte"
			nullable = name == "JSON_AGG" || name == "JSONB_AGG" || name == "JSON_GROUP_ARRAY"
		}

		if matchAlias := asAliasRe.FindStringSubmatch(rawExpr); matchAlias != nil {
			return model.ResultField{Name: parse.ToPascal(matchAlias[1]), DBName: matchAlias[1], GoType: gotype.ApplyNullability(goType, nullable), Nullable: nullable, IsExpression: true}
		}

		return model.ResultField{Name: parse.ToPascal(name), DBName: strings.ToLower(name), GoType: gotype.ApplyNullability(goType, nullable), Nullable: nullable, IsExpression: true}
	}

	name := expr
	if idx := strings.IndexAny(name, " \t("); idx >= 0 {
		name = name[:idx]
	}

	return model.ResultField{
		Name:         parse.ToPascal(strings.ReplaceAll(name, ".", "_")),
		DBName:       name,
		GoType:       gotype.ApplyNullability("any", !isDefinitelyNonNullLiteral(expr)),
		Nullable:     !isDefinitelyNonNullLiteral(expr),
		IsExpression: true,
	}
}

func literalResultField(rawExpr, expr, goType string) model.ResultField {
	return model.ResultField{
		Name:         parse.ToPascal(fieldNameForExpr(rawExpr, expr)),
		DBName:       dbNameForExpr(rawExpr, expr),
		GoType:       goType,
		IsExpression: true,
	}
}

func inferCoalesceType(schema model.Schema, expr string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool, derivedSources map[string]map[string]model.ResultField, grouped bool, d model.Dialect) (string, bool) {
	args, ok := extractFunctionArgs(expr, []string{"coalesce"})
	if !ok {
		return "any", true
	}

	parts := utils.SplitByDelimiter(args, ',')
	goType := "any"
	nullable := true

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || isNullLiteral(part) {
			continue
		}

		field := parseExpression(schema, part, part, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)
		if goType == "any" && field.GoType != "" && field.GoType != "any" && field.GoType != "*any" {
			goType = strings.TrimPrefix(field.GoType, "*")
		}

		if isDefinitelyNonNullLiteral(part) || !field.Nullable {
			nullable = false
		}
	}

	return goType, nullable
}

func isNullLiteral(expr string) bool {
	return strings.EqualFold(strings.TrimSpace(expr), "NULL")
}

func isDefinitelyNonNullLiteral(expr string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" || isNullLiteral(expr) {
		return false
	}
	if numberLiteralRe.MatchString(expr) {
		return true
	}
	if stringLiteralRe.MatchString(expr) {
		return true
	}
	switch strings.ToUpper(expr) {
	case "TRUE", "FALSE", "CURRENT_TIMESTAMP", "NOW()":
		return true
	}
	return false
}

func lookupQueryAlias(schema model.Schema, queryTables [][2]string, alias string) (string, bool) {
	for _, pair := range queryTables {
		if pair[1] == alias {
			return pair[0], true
		}
	}
	return parse.FindTableByAlias(schema, alias)
}

func convertExplicitType(explicit string, d model.Dialect) string {
	if v, ok := dialect.TypeMap(d)[strings.ToUpper(explicit)]; ok {
		return v
	}
	return explicit
}

func extractFunctionArgs(expr string, names []string) (string, bool) {
	upper := strings.ToUpper(expr)

	for _, name := range names {
		token := strings.ToUpper(name) + "("
		start := strings.Index(upper, token)
		if start == -1 {
			continue
		}

		i := start + len(token)
		depth := 1
		begin := i
		for i < len(expr) {
			switch expr[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					return expr[begin:i], true
				}
			}
			i++
		}
	}
	return "", false
}
