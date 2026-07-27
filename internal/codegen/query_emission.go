package codegen

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

func writeConstants(buf *bytes.Buffer, schema model.Schema) {
	tableNames := sortedTables(schema)
	buf.WriteString("type Table string\n\n")
	buf.WriteString("const (\n")
	for _, name := range tableNames {
		fmt.Fprintf(buf, "\t%s Table = %q\n", parse.ToPascal(name), name)
	}
	buf.WriteString(")\n\n")

	buf.WriteString("type SortDirection string\n\n")
	buf.WriteString("const (\n\tASC SortDirection = \"ASC\"\n\tDESC SortDirection = \"DESC\"\n)\n\n")

	for _, name := range tableNames {
		table := schema.Tables[name]
		fieldType := parse.ToPascal(name) + "Field"
		fmt.Fprintf(buf, "type %s string\n\n", fieldType)
		buf.WriteString("const (\n")
		cols := sortedColumns(table)
		for _, col := range cols {
			fmt.Fprintf(buf, "\t%s%s %s = %q\n", fieldType, parse.ToPascal(col), fieldType, col)
		}
		buf.WriteString(")\n\n")
	}
}

func writeCustomQueries(buf *bytes.Buffer, queries []model.AnalyzedQuery, schemasPkg string) {
	for _, analyzed := range queries {
		q := analyzed.Query
		resultType := analyzedQueryResultType(analyzed, schemasPkg)
		queryType := runtimeQueryType(resultType)
		paramsStruct := q.Name + "Params"
		useStruct := len(q.Params) > 1

		if useStruct {
			fmt.Fprintf(buf, "type %s struct {\n", paramsStruct)
			for _, p := range q.Params {
				fmt.Fprintf(buf, "\t%s %s\n", parse.ToPascal(p.Name), p.Type)
			}
			buf.WriteString("}\n\n")
		}

		var sigParts []string
		if useStruct {
			sigParts = append(sigParts, "params "+paramsStruct)
		} else if len(q.Params) == 1 {
			sigParts = append(sigParts, sanitizeName(q.Params[0].Name)+" "+q.Params[0].Type)
		}
		for _, h := range q.Hashtags {
			sigParts = append(sigParts, sanitizeName(h.Name)+" "+h.Type)
		}

		writeDocComment(buf, q.Description, q.Deprecated)

		if resultType == "struct{}" {
			trimmed := strings.TrimSpace(strings.ToUpper(q.SourceSQL))
			if strings.HasPrefix(trimmed, "SELECT") || strings.HasPrefix(trimmed, "WITH") || strings.Contains(trimmed, "RETURNING") {
				buf.WriteString("// WARNING: this query looks like it returns rows but no result struct was generated.\n")
				buf.WriteString("// Check that columns are properly aliased with AS and that CTE references resolve.\n")
			}
		}

		fmt.Fprintf(buf, "func %s(%s) %s {\n", q.Name, strings.Join(sigParts, ", "), queryType)
		if len(q.Hashtags) > 0 {
			hashtagByName := map[string]model.Hashtag{}
			for _, h := range q.Hashtags {
				hashtagByName[h.Name] = h
			}

			parts := strings.Split(q.SQL, parse.HashtagDelimiter)
			buf.WriteString("\tvar sql strings.Builder\n")
			fmt.Fprintf(buf, "\tsql.WriteString(`%s`)\n", escapeBackticks(parts[0]))
			for i, name := range q.HashtagSequence {
				h := hashtagByName[name]
				name := sanitizeName(h.Name)

				if h.IsSlice {
					fmt.Fprintf(buf, "\tsql.WriteString(strings.Join(%s, \", \"))\n", name)
				} else {
					fmt.Fprintf(buf, "\tsql.WriteString(string(%s))\n", name)
				}

				if i+1 < len(parts) {
					fmt.Fprintf(buf, "\tsql.WriteString(`%s`)\n", escapeBackticks(parts[i+1]))
				}
			}
		}

		fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n", resultType)
		if len(q.Hashtags) > 0 {
			buf.WriteString("\t\tSQL: sql.String(),\n")
		} else {
			fmt.Fprintf(buf, "\t\tSQL: `%s`,\n", escapeBackticks(q.SQL))
		}

		if shouldPrepareQuery(q) {
			fmt.Fprintf(buf, "\t\tStmtName: %q,\n", q.Name)
		}

		if len(q.Params) == 0 {
			buf.WriteString("\t\tArgs: nil,\n")
		} else {
			buf.WriteString("\t\tArgs: []any{\n")
			for _, p := range q.Params {
				if useStruct {
					fmt.Fprintf(buf, "\t\t\tparams.%s,\n", parse.ToPascal(p.Name))
				} else {
					fmt.Fprintf(buf, "\t\t\t%s,\n", sanitizeName(p.Name))
				}
			}

			buf.WriteString("\t\t},\n")
		}

		buf.WriteString("\t}\n")
		buf.WriteString("}\n\n")
	}
}

func writeDocComment(buf *bytes.Buffer, description, deprecated string) {
	if description != "" {
		for line := range strings.SplitSeq(description, "\n") {
			fmt.Fprintf(buf, "// %s\n", line)
		}
	}

	if deprecated != "" {
		for i, line := range strings.Split(deprecated, "\n") {
			if i == 0 {
				fmt.Fprintf(buf, "// Deprecated: %s\n", line)
			} else {
				fmt.Fprintf(buf, "// %s\n", line)
			}
		}
	}
}
