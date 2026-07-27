package codegen

import (
	"bytes"
	"fmt"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

func writePrepareStatements(buf *bytes.Buffer, schema model.Schema, queries []model.AnalyzedQuery, d model.Dialect) {
	buf.WriteString("func PrepareStatements(ctx context.Context, conn querycruntime.Preparer) error {\n")
	for _, analyzed := range queries {
		q := analyzed.Query
		if !shouldPrepareQuery(q) {
			continue
		}
		fmt.Fprintf(buf, "\tif _, err := conn.Prepare(ctx, %q, `%s`); err != nil {\n", q.Name, escapeBackticks(q.SQL))
		fmt.Fprintf(buf, "\t\treturn fmt.Errorf(\"prepare %s: %%w\", err)\n", q.Name)
		buf.WriteString("\t}\n")
	}
	for _, tableName := range sortedTables(schema) {
		table := schema.Tables[tableName]
		structName := parse.ToPascal(tableName)
		singular := parse.ToSingular(structName)
		statements := buildCRUDStatements(tableName, table, d)
		writePreparedStatement(buf, "Add"+singular, statements.Add)
		if len(table.PrimaryKeys) == 0 {
			continue
		}
		writePreparedStatement(buf, "Get"+singular, statements.Get)
		writePreparedStatement(buf, "GetAll"+structName, statements.GetAll)
		writePreparedStatement(buf, "Delete"+singular, statements.Delete)
		writePreparedStatement(buf, "Update"+singular, statements.Update)
		writePreparedStatement(buf, "Set"+singular, statements.Set)
	}
	buf.WriteString("\treturn nil\n}\n\n")
}

func writePreparedStatement(buf *bytes.Buffer, name, sql string) {
	if sql == "" {
		return
	}
	fmt.Fprintf(buf, "\tif _, err := conn.Prepare(ctx, %q, `%s`); err != nil {\n", name, escapeBackticks(sql))
	fmt.Fprintf(buf, "\t\treturn fmt.Errorf(\"prepare %s: %%w\", err)\n", name)
	buf.WriteString("\t}\n")
}
