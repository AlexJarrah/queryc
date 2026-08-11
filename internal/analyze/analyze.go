package analyze

import (
	"fmt"
	"regexp"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

var (
	asAliasRe         = regexp.MustCompile(`(?is)\s+AS\s+(\w+)$`)
	aliasExprRe       = regexp.MustCompile(`(?is)(.+?)\s+AS\s+(\w+)$`)
	numberLiteralRe   = regexp.MustCompile(`^[-+]?\d+(\.\d+)?$`)
	stringLiteralRe   = regexp.MustCompile(`^'.*'$`)
	qualifiedColRe    = regexp.MustCompile(`^([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+|\*)$`)
	simpleIdentRe     = regexp.MustCompile(`^([a-zA-Z0-9_]+)$`)
	castFuncRe        = regexp.MustCompile(`(?is)^CAST\((.+)\s+AS\s+([a-zA-Z0-9_ ]+)\)$`)
	pgCastRe          = regexp.MustCompile(`(?is)^(.+)::([a-zA-Z]+)$`)
	funcCallRe        = regexp.MustCompile(`(?is)^([a-zA-Z0-9_]+)\s*\(`)
	qualifiedSimpleRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)$`)
	jsonKeyLiteralRe  = regexp.MustCompile(`^['"]([^'"]+)['"]$`)
	starSelectRe      = regexp.MustCompile(`^([a-zA-Z0-9_]+)\.\*$`)
	fromJoinTableRe   = regexp.MustCompile(`(?is)\b(?:FROM|JOIN)\s+([a-zA-Z0-9_]+)(?:\s+(?:AS\s+)?([a-zA-Z0-9_]+))?`)
	fromTableRe       = regexp.MustCompile(`(?is)\bFROM\s+([a-zA-Z0-9_]+)(?:\s+(?:AS\s+)?([a-zA-Z0-9_]+))?`)
	joinKindTableRe   = regexp.MustCompile(`(?is)\b((?:LEFT|RIGHT|FULL)(?:\s+OUTER)?|INNER|CROSS)?\s*JOIN(?:\s+LATERAL)?\s+([a-zA-Z0-9_]+|\(\))(?:\s+(?:AS\s+)?([a-zA-Z0-9_]+))?\s+ON`)
	sliceValueAsRe    = regexp.MustCompile(`(?is)\bSELECT\s+(.+?)\s+AS\s+queryc_value\s+FROM\b`)
)

// Queries parses a collection of queries.
func Queries(schema model.Schema, queries []model.Query, d model.Dialect) ([]model.AnalyzedQuery, error) {
	result := make([]model.AnalyzedQuery, 0, len(queries))
	for _, q := range queries {
		analyzed, err := Query(schema, q, d)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", q.Name, err)
		}
		result = append(result, analyzed)
	}
	return result, nil
}

// Query parses a query and returns an analysis.
func Query(schema model.Schema, query model.Query, d model.Dialect) (model.AnalyzedQuery, error) {
	cleanSQL := stripMarkers(query.RawSQL)
	queryTables := getQueryTables(cleanSQL)
	nullableTables, nullableAliases := getTablesWithNullableJoin(cleanSQL)
	derivedSources := buildDerivedSources(schema, query.RawSQL, d)
	fields, err := parseSelectColumns(schema, query.RawSQL, queryTables, nullableTables, nullableAliases, derivedSources, d)
	if err != nil {
		return model.AnalyzedQuery{}, err
	}
	shouldEmit, structName := shouldGenerateStruct(query.Name, fields, query.SourceSQL)
	embedded := detectEmbeddedTables(schema, query.RawSQL, queryTables, nullableTables, nullableAliases)
	if len(embedded) == 0 && len(fields) == 1 && fields[0].Skip && len(queryTables) > 0 {
		mainTable := queryTables[0][0]
		embedded = append(embedded, model.EmbeddedTable{
			TableName:  mainTable,
			StructName: parse.ToSingular(parse.ToPascal(mainTable)),
			IsNullable: false,
			TableAlias: queryTables[0][1],
		})
	}

	// Do not generate result struct if result is a single schemas-defined type.
	if shouldEmit && len(embedded) == 1 && !embedded[0].IsNullable {
		allSkipped := true
		for _, f := range fields {
			if !f.Skip {
				allSkipped = false
				break
			}
		}
		if allSkipped {
			shouldEmit = false
			structName = ""
		}
	}

	for i := range fields {
		field := &fields[i]
		if field.GeneratedStructKind != "" && field.GeneratedStructName == "" {
			field.GeneratedStructName = generatedJSONStructName(query.Name, field.DBName, field.GeneratedStructKind)
		}

		if field.GeneratedStructName == "" {
			continue
		}

		for j := range field.GeneratedFields {
			gf := &field.GeneratedFields[j]
			gf.GoType = resolveFieldType(schema, *gf, nullableTables, nullableAliases, d)
		}
	}

	return model.AnalyzedQuery{
		Query:             query,
		ResultStructName:  structName,
		ShouldEmitResult:  shouldEmit,
		Fields:            fields,
		EmbeddedTables:    embedded,
		NullableTables:    nullableTables,
		NullableAliases:   nullableAliases,
		GeneratedJSONSeen: map[string]bool{},
	}, nil
}
