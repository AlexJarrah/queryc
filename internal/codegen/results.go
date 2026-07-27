package codegen

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/AlexJarrah/queryc/internal/gotype"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

func writeResultStructs(buf *bytes.Buffer, queries []model.AnalyzedQuery, schemasPkg string) {
	seenGenerated := map[string]bool{}
	for _, analyzed := range queries {
		if !analyzed.ShouldEmitResult {
			continue
		}

		for _, field := range analyzed.Fields {
			if field.GeneratedStructKind == "" || field.GeneratedStructName == "" || seenGenerated[field.GeneratedStructName] {
				continue
			}
			seenGenerated[field.GeneratedStructName] = true
			fmt.Fprintf(buf, "// %s is generated from the JSON-backed field %s.\n", field.GeneratedStructName, field.Name)
			fmt.Fprintf(buf, "type %s struct {\n", field.GeneratedStructName)
			for _, gf := range field.GeneratedFields {
				fmt.Fprintf(buf, "\t%s %s `json:\"%s\"`\n", gf.FieldName, gf.GoType, gf.JSONName)
			}
			buf.WriteString("}\n\n")
		}

		fmt.Fprintf(buf, "// %s represents a result from the query %s\n", analyzed.ResultStructName, analyzed.Query.Name)
		fmt.Fprintf(buf, "type %s struct {\n", analyzed.ResultStructName)
		for _, embedded := range analyzed.EmbeddedTables {
			tag := fmt.Sprintf("`db:\"%s\" table:\"%s\"`", embedded.StructName, embedded.TableName)
			if embedded.IsNullable {
				fmt.Fprintf(buf, "\t%s *%s.%s %s\n", embedded.StructName, schemasPkg, embedded.StructName, tag)
			} else {
				fmt.Fprintf(buf, "\t%s %s.%s %s\n", embedded.StructName, schemasPkg, embedded.StructName, tag)
			}
		}

		for _, field := range analyzed.Fields {
			if field.Skip {
				continue
			}

			goType := field.GoType
			if field.GeneratedStructName != "" {
				if field.GeneratedStructKind == "slice" {
					goType = "[]" + field.GeneratedStructName
				} else {
					goType = field.GeneratedStructName
				}
				goType = gotype.ApplyNullability(goType, field.Nullable)
			}

			tag := fmt.Sprintf("`db:\"%s\"`", field.DBName)
			if field.Serialization == "json" {
				tag = fmt.Sprintf("`db:\"%s\" queryc:\"json\"`", field.DBName)
			}
			fmt.Fprintf(buf, "\t%s %s %s\n", field.Name, goType, tag)
		}
		buf.WriteString("}\n\n")
	}
}

func sanitizeName(name string) string {
	name = parse.ToCamel(name)
	switch name {
	case "break", "case", "chan", "const", "continue",
		"default", "defer", "else", "fallthrough", "for",
		"func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return",
		"select", "struct", "switch", "type", "var":
		return name + "_val"
	default:
		return name
	}
}

func shouldPrepareQuery(q model.Query) bool {
	if len(q.Hashtags) > 0 {
		return false
	}
	sql := strings.TrimSpace(strings.ToUpper(q.SQL))
	return !strings.HasPrefix(sql, "CREATE ") && !strings.HasPrefix(sql, "DROP ")
}

func runtimeQueryType(resultType string) string {
	return "*querycruntime.Query[" + resultType + "]"
}

func analyzedQueryResultType(analyzed model.AnalyzedQuery, schemasPkg string) string {
	if len(analyzed.EmbeddedTables) == 1 && !analyzed.EmbeddedTables[0].IsNullable && hasOnlySkippedFields(analyzed.Fields) {
		return schemasPkg + "." + analyzed.EmbeddedTables[0].StructName
	}
	if analyzed.ShouldEmitResult && analyzed.ResultStructName != "" {
		return analyzed.ResultStructName
	}
	return "struct{}"
}

func hasOnlySkippedFields(fields []model.ResultField) bool {
	if len(fields) == 0 {
		return true
	}

	for _, field := range fields {
		if !field.Skip {
			return false
		}
	}
	return true
}
