package analyze

import (
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/gotype"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
	"github.com/AlexJarrah/queryc/internal/sqlscan"
	"github.com/AlexJarrah/queryc/internal/utils"
)

func inferJSONField(schema model.Schema, expr string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool, derivedSources map[string]map[string]model.ResultField, grouped bool, d model.Dialect) (model.ResultField, bool) {
	clean := stripMarkers(expr)
	if args, ok := extractFunctionArgs(clean, []string{"coalesce"}); ok {
		parts := utils.SplitByDelimiter(args, ',')
		if len(parts) > 0 {
			if inner, ok := inferJSONField(schema, strings.TrimSpace(parts[0]), queryTables, nullableTables, nullableAliases, derivedSources, grouped, d); ok {
				inner.Nullable = false
				return inner, true
			}
		}
	}

	if args, ok := extractFunctionArgs(clean, []string{"json_agg", "jsonb_agg", "json_group_array"}); ok {
		inner := strings.TrimSpace(args)
		inner = stripLeadingDistinct(inner)
		if generated, ok := inferJSONObjectFields(schema, inner, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d); ok {
			return model.ResultField{
				GoType:              "[]any",
				ExplicitType:        "[]any",
				Nullable:            true,
				GeneratedStructKind: "slice",
				GeneratedFields:     generated,
			}, true
		}

		if sourceQuery, ok := extractSliceSourceQuery(clean); ok {
			localQueryTables := getQueryTables(stripMarkers(sourceQuery))
			localNullableTables, localNullableAliases := getTablesWithNullableJoin(stripMarkers(sourceQuery))
			localDerivedSources := buildDerivedSourcesWithBase(schema, sourceQuery, derivedSources, d)

			if subqueryExpr, ok := extractSliceQueryValueExpr(sourceQuery); ok {
				if generated, ok := inferJSONObjectFields(schema, subqueryExpr, localQueryTables, localNullableTables, localNullableAliases, localDerivedSources, grouped, d); ok {
					return model.ResultField{
						GoType:              "[]any",
						ExplicitType:        "[]any",
						Nullable:            true,
						GeneratedStructKind: "slice",
						GeneratedFields:     generated,
					}, true
				}
			}
		}

		if subqueryExpr, ok := extractSliceQueryValueExpr(clean); ok {
			if generated, ok := inferJSONObjectFields(schema, subqueryExpr, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d); ok {
				return model.ResultField{
					GoType:              "[]any",
					ExplicitType:        "[]any",
					Nullable:            true,
					GeneratedStructKind: "slice",
					GeneratedFields:     generated,
				}, true
			}
		}

		if match := qualifiedSimpleRe.FindStringSubmatch(inner); match != nil {
			if tableName, ok := lookupQueryAlias(schema, queryTables, match[1]); ok {
				if col, ok := schema.Tables[tableName].Columns[match[2]]; ok {
					goType := "[]" + dialect.GoTypeForSQL(d, col.SQLType)
					return model.ResultField{GoType: goType, ExplicitType: goType, Nullable: true}, true
				}
			}
		}

		return model.ResultField{GoType: "[]byte", ExplicitType: "[]byte", Nullable: true}, true
	}

	if generated, ok := inferJSONObjectFields(schema, clean, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d); ok {
		return model.ResultField{
			GoType:              "any",
			ExplicitType:        "any",
			Nullable:            false,
			GeneratedStructKind: "struct",
			GeneratedFields:     generated,
		}, true
	}

	return model.ResultField{}, false
}

func inferJSONObjectFields(schema model.Schema, expr string, queryTables [][2]string, nullableTables, nullableAliases map[string]bool, derivedSources map[string]map[string]model.ResultField, grouped bool, d model.Dialect) ([]model.GeneratedJSONField, bool) {
	args, ok := extractFunctionArgs(expr, []string{"json_build_object", "jsonb_build_object", "json_object"})
	if !ok {
		return nil, false
	}

	parts := utils.SplitByDelimiter(args, ',')
	if len(parts) < 2 || len(parts)%2 != 0 {
		return nil, false
	}

	fields := make([]model.GeneratedJSONField, 0, len(parts)/2)
	for i := 0; i < len(parts); i += 2 {
		keyMatch := jsonKeyLiteralRe.FindStringSubmatch(strings.TrimSpace(parts[i]))
		if keyMatch == nil {
			return nil, false
		}

		valueExpr := strings.TrimSpace(parts[i+1])
		parsedExpr, explicitType := extractTypeAnnotation(valueExpr)
		parsedField := parseExpression(schema, parsedExpr, parsedExpr, queryTables, nullableTables, nullableAliases, derivedSources, grouped, d)

		if explicitType != "" {
			parsedField.GoType = gotype.ApplyNullability(convertExplicitType(explicitType, d), parsedField.Nullable)
			parsedField.ExplicitType = parsedField.GoType
			parsedField.FromTable = ""
			parsedField.FromColumn = ""
			parsedField.TableAlias = ""
		}

		fields = append(fields, model.GeneratedJSONField{
			JSONName:   keyMatch[1],
			FieldName:  parse.ToPascal(keyMatch[1]),
			GoType:     parsedField.GoType,
			Nullable:   parsedField.Nullable,
			FromTable:  parsedField.FromTable,
			FromColumn: parsedField.FromColumn,
			TableAlias: parsedField.TableAlias,
		})
	}

	return fields, true
}

func resolveFieldType(schema model.Schema, field model.GeneratedJSONField, nullableTables, nullableAliases map[string]bool, d model.Dialect) string {
	goType := field.GoType
	if field.FromTable == "" || field.FromColumn == "" {
		return goType
	}

	col, ok := schema.Tables[field.FromTable].Columns[field.FromColumn]
	if !ok {
		return goType
	}

	goType = dialect.GoTypeForSQL(d, col.SQLType)
	return gotype.ApplyNullability(goType, field.Nullable || col.Nullable || nullableTables[field.FromTable] || nullableAliases[field.TableAlias])
}

func generatedJSONStructName(queryName, fieldDBName, kind string) string {
	base := queryName + parse.ToPascal(fieldDBName)
	if kind == "slice" {
		return base + "Item"
	}
	return base
}

func extractSliceQueryValueExpr(expr string) (string, bool) {
	match := sliceValueAsRe.FindStringSubmatch(expr)
	if match == nil {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}

func extractSliceSourceQuery(expr string) (string, bool) {
	upper := strings.ToUpper(expr)
	idx := strings.Index(upper, "FROM (")
	if idx == -1 {
		return "", false
	}

	open := strings.Index(expr[idx:], "(")
	if open == -1 {
		return "", false
	}

	open += idx
	body, _, err := sqlscan.ExtractBalanced(expr, open)
	if err != nil {
		return "", false
	}

	return strings.TrimSpace(body), true
}
