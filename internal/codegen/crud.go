package codegen

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

func writeCRUD(buf *bytes.Buffer, schema model.Schema, d model.Dialect, schemasPkg string) {
	for _, tableName := range sortedTables(schema) {
		table := schema.Tables[tableName]
		structName := parse.ToPascal(tableName)
		singular := parse.ToSingular(structName)
		writeAdd(buf, tableName, singular, table, d, schemasPkg)
		writeAddMany(buf, tableName, structName, singular, table, d, schemasPkg)
		if len(table.PrimaryKeys) == 0 {
			continue
		}
		writeGetAll(buf, tableName, structName, table, d, schemasPkg)
		writeGet(buf, tableName, singular, table, d, schemasPkg)
		writeGetMany(buf, tableName, structName, singular, table, d, schemasPkg)
		writeDelete(buf, tableName, singular, table, d)
		writeUpdate(buf, tableName, singular, table, d, schemasPkg)
		writeSet(buf, tableName, singular, table, d, schemasPkg)
	}
}

func writeAdd(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, schemasPkg string) {
	resultType := schemasPkg + "." + singular
	defCols := defaultableColumns(table)

	// Cannot be static if defaultable columns exist
	if len(defCols) == 0 {
		writeStaticAdd(buf, tableName, singular, table, d, resultType, schemasPkg)
	} else {
		writeDynamicAdd(buf, tableName, singular, table, d, resultType, schemasPkg, defCols)
	}
}

func writeStaticAdd(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, resultType, schemasPkg string) {
	insertCols := insertColumns(table)
	statement := buildCRUDStatements(tableName, table, d).Add
	fmt.Fprintf(buf, "func Add%s(row *%s.%s) %s {\n", singular, schemasPkg, singular, runtimeQueryType(resultType))
	writeRequiredRow(buf, resultType)
	writePKValidation(buf, table, "row", resultType)
	writeAutoValidation(buf, table, "row", resultType)
	args := make([]string, 0, len(insertCols))
	for _, col := range insertCols {
		args = append(args, "row."+parse.ToPascal(col))
	}
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n", resultType)
	fmt.Fprintf(buf, "\t\tSQL: `%s`,\n", statement)
	fmt.Fprintf(buf, "\t\tStmtName: %q,\n", "Add"+singular)
	buf.WriteString("\t\tArgs: []any{\n")
	for _, arg := range args {
		fmt.Fprintf(buf, "\t\t\t%s,\n", arg)
	}
	buf.WriteString("\t\t},\n\t}\n}\n\n")
}

func writeDynamicAdd(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, resultType, schemasPkg string, defCols []string) {
	defType := defaultableFieldType(tableName)
	fmt.Fprintf(buf, "func Add%s(row *%s.%s, useDefault ...%s) %s {\n", singular, schemasPkg, singular, defType, runtimeQueryType(resultType))
	writeRequiredRow(buf, resultType)
	writeUseDefaultValidation(buf, tableName, resultType, defType, defCols)
	writePKValidationWithDefaults(buf, table, "row", resultType, defType, defCols)
	writeAutoValidation(buf, table, "row", resultType)
	writeDynamicInsertBody(buf, table, tableName, resultType, d, defType, defCols, "row")
}

func writeAddMany(buf *bytes.Buffer, tableName, structName, singular string, table model.Table, d model.Dialect, schemasPkg string) {
	resultType := schemasPkg + "." + singular
	defCols := defaultableColumns(table)
	if len(defCols) == 0 {
		writeStaticAddMany(buf, tableName, structName, singular, table, d, resultType, schemasPkg)
		return
	}
	writeDynamicAddMany(buf, tableName, structName, table, d, resultType, schemasPkg, defCols)
}

func writeStaticAddMany(buf *bytes.Buffer, tableName, structName, singular string, table model.Table, d model.Dialect, resultType, schemasPkg string) {
	insertCols := insertColumns(table)
	fmt.Fprintf(buf, "func AddMany%s(rows []%s.%s) *querycruntime.Query[%s] {\n", structName, schemasPkg, singular, resultType)
	fmt.Fprintf(buf, "\tif len(rows) == 0 {\n\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"rows slice cannot be empty\")}\n\t}\n", resultType)
	buf.WriteString("\tfor _, r := range rows {\n")
	writePKValidation(buf, table, "r", resultType)
	writeAutoValidation(buf, table, "r", resultType)
	buf.WriteString("\t}\n")
	buf.WriteString("\tvar valueSets []string\n\tvar args []any\n")
	if d == model.DialectPostgres {
		buf.WriteString("\tcounter := 1\n")
	}
	buf.WriteString("\tfor _, row := range rows {\n")
	for _, col := range insertCols {
		fmt.Fprintf(buf, "\t\targs = append(args, row.%s)\n", parse.ToPascal(col))
	}
	buf.WriteString("\t\tvar parts []string\n")
	if d == model.DialectPostgres {
		fmt.Fprintf(buf, "\t\tfor i := range %d {\n", len(insertCols))
		fmt.Fprintf(buf, "\t\t\tparts = append(parts, %q)\n", dialect.Placeholder(d, 1))
		buf.WriteString("\t\t\tparts[len(parts)-1] = fmt.Sprintf(\"$%d\", counter+i)\n")
	} else {
		fmt.Fprintf(buf, "\t\tfor range %d {\n", len(insertCols))
		fmt.Fprintf(buf, "\t\t\tparts = append(parts, %q)\n", dialect.Placeholder(d, 1))
	}
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tvalueSets = append(valueSets, \"(\"+strings.Join(parts, \", \")+\")\")\n")
	if d == model.DialectPostgres {
		fmt.Fprintf(buf, "\t\tcounter += %d\n", len(insertCols))
	}
	buf.WriteString("\t}\n")
	fmt.Fprintf(buf, "\tquery := `INSERT INTO %s (%s) VALUES ` + strings.Join(valueSets, \", \") + \" RETURNING *\"\n", tableName, strings.Join(insertCols, ", "))
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n\t\tSQL: query,\n\t\tArgs: args,\n\t}\n}\n\n", resultType)
}

func writeDynamicAddMany(buf *bytes.Buffer, tableName, structName string, table model.Table, d model.Dialect, resultType, schemasPkg string, defCols []string) {
	singular := parse.ToSingular(structName)
	defType := defaultableFieldType(tableName)
	fmt.Fprintf(buf, "func AddMany%s(rows []%s.%s, useDefault ...%s) *querycruntime.Query[%s] {\n", structName, schemasPkg, singular, defType, resultType)
	fmt.Fprintf(buf, "\tif len(rows) == 0 {\n\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"rows slice cannot be empty\")}\n\t}\n", resultType)
	writeUseDefaultValidation(buf, tableName, resultType, defType, defCols)
	buf.WriteString("\tfor _, r := range rows {\n")
	writePKValidationWithDefaults(buf, table, "r", resultType, defType, defCols)
	writeAutoValidation(buf, table, "r", resultType)
	buf.WriteString("\t}\n")
	writeDynamicInsertManyBody(buf, table, tableName, resultType, d, defType, defCols)
}

func writeGetAll(buf *bytes.Buffer, tableName, structName string, table model.Table, d model.Dialect, schemasPkg string) {
	resultType := schemasPkg + "." + parse.ToSingular(structName)
	fmt.Fprintf(buf, "func GetAll%s() *querycruntime.Query[%s] {\n", structName, resultType)
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n", resultType)
	fmt.Fprintf(buf, "\t\tSQL: `%s`,\n", buildCRUDStatements(tableName, table, d).GetAll)
	fmt.Fprintf(buf, "\t\tStmtName: %q,\n", "GetAll"+structName)
	buf.WriteString("\t\tArgs: nil,\n\t}\n}\n\n")
}

func writeGet(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, schemasPkg string) {
	sig, _, names := pkSignature(table, d)
	resultType := schemasPkg + "." + singular
	fmt.Fprintf(buf, "func Get%s(%s) %s {\n", singular, sig, runtimeQueryType(resultType))
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n", resultType)
	fmt.Fprintf(buf, "\t\tSQL: `%s`,\n", buildCRUDStatements(tableName, table, d).Get)
	fmt.Fprintf(buf, "\t\tStmtName: %q,\n", "Get"+singular)
	buf.WriteString("\t\tArgs: []any{\n")
	for _, name := range names {
		fmt.Fprintf(buf, "\t\t\t%s,\n", name)
	}
	buf.WriteString("\t\t},\n\t}\n}\n\n")
}

func writeGetMany(buf *bytes.Buffer, tableName, structName, singular string, table model.Table, d model.Dialect, schemasPkg string) {
	resultType := schemasPkg + "." + singular
	multiPK := len(table.PrimaryKeys) > 1

	argName := "keys"
	if !multiPK {
		argName = sanitizeName(table.PrimaryKeys[0] + "s")
	}

	if multiPK {
		fmt.Fprintf(buf, "type %sPKs struct {\n", singular)
		for _, pk := range table.PrimaryKeys {
			fmt.Fprintf(buf, "\t%s %s\n", parse.ToPascal(pk), dialect.GoTypeForSQL(d, table.Columns[pk].SQLType))
		}
		buf.WriteString("}\n\n")
	}

	argType := "[]" + singular + "PKs"
	if !multiPK {
		argType = "[]" + dialect.GoTypeForSQL(d, table.Columns[table.PrimaryKeys[0]].SQLType)
	}

	fmt.Fprintf(buf, "func Get%s(%s %s) %s {\n", structName, argName, argType, runtimeQueryType(resultType))
	fmt.Fprintf(buf, "\tif len(%s) == 0 {\n", argName)
	fmt.Fprintf(buf, "\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(%q)}\n\t}\n", resultType, argName+" slice cannot be empty")
	buf.WriteString("\tvar parts []string\n\tvar args []any\n")
	if d == model.DialectPostgres {
		buf.WriteString("\tcounter := 1\n")
	}
	fmt.Fprintf(buf, "\tfor _, k := range %s {\n", argName)

	if multiPK {
		for _, pk := range table.PrimaryKeys {
			fmt.Fprintf(buf, "\t\targs = append(args, k.%s)\n", parse.ToPascal(pk))
		}
		buf.WriteString("\t\tvar conds []string\n")
		for i, pk := range table.PrimaryKeys {
			if d == model.DialectPostgres {
				fmt.Fprintf(buf, "\t\tconds = append(conds, fmt.Sprintf(%q, counter+%d))\n", pk+" = $%d", i)
			} else {
				fmt.Fprintf(buf, "\t\tconds = append(conds, %q)\n", pk+" = ?")
			}
		}
		buf.WriteString("\t\tparts = append(parts, \"(\"+strings.Join(conds, \" AND \")+\")\")\n")
		if d == model.DialectPostgres {
			fmt.Fprintf(buf, "\t\tcounter += %d\n", len(table.PrimaryKeys))
		}
	} else {
		buf.WriteString("\t\targs = append(args, k)\n")
		if d == model.DialectPostgres {
			buf.WriteString("\t\tparts = append(parts, fmt.Sprintf(\"$%d\", counter))\n")
			buf.WriteString("\t\tcounter++\n")
		} else {
			buf.WriteString("\t\tparts = append(parts, \"?\")\n")
		}
	}
	buf.WriteString("\t}\n")

	if multiPK {
		fmt.Fprintf(buf, "\tquery := `SELECT * FROM %s WHERE ` + strings.Join(parts, \" OR \")\n", tableName)
	} else {
		fmt.Fprintf(buf, "\tquery := `SELECT * FROM %s WHERE %s IN (` + strings.Join(parts, \", \") + `)`\n", tableName, table.PrimaryKeys[0])
	}

	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n\t\tSQL: query,\n\t\tArgs: args,\n\t}\n}\n\n", resultType)
}

func writeDelete(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect) {
	sig, _, names := pkSignature(table, d)
	resultType := "struct{}"
	fmt.Fprintf(buf, "func Delete%s(%s) %s {\n", singular, sig, runtimeQueryType(resultType))
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n", resultType)
	fmt.Fprintf(buf, "\t\tSQL: `%s`,\n", buildCRUDStatements(tableName, table, d).Delete)
	fmt.Fprintf(buf, "\t\tStmtName: %q,\n", "Delete"+singular)
	buf.WriteString("\t\tArgs: []any{\n")
	for _, name := range names {
		fmt.Fprintf(buf, "\t\t\t%s,\n", name)
	}
	buf.WriteString("\t\t},\n\t}\n}\n\n")
}

func writeUpdate(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, schemasPkg string) {
	sig, _, names := pkSignature(table, d)
	updateCols := nonPKColumns(table)
	if len(updateCols) == 0 {
		return
	}

	resultType := "struct{}"
	fmt.Fprintf(buf, "func Update%s(%s, row *%s.%s) %s {\n", singular, sig, schemasPkg, singular, runtimeQueryType(resultType))
	writeRequiredRow(buf, resultType)
	writeZeroPKValidation(buf, table, "row", resultType)
	var args []string
	for _, col := range updateCols {
		if col == "updated_at" {
			continue
		}
		args = append(args, "row."+parse.ToPascal(col))
	}
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n", resultType)
	fmt.Fprintf(buf, "\t\tSQL: `%s`,\n", buildCRUDStatements(tableName, table, d).Update)
	fmt.Fprintf(buf, "\t\tStmtName: %q,\n", "Update"+singular)
	buf.WriteString("\t\tArgs: []any{\n")
	for _, name := range names {
		fmt.Fprintf(buf, "\t\t\t%s,\n", name)
	}
	for _, arg := range args {
		fmt.Fprintf(buf, "\t\t\t%s,\n", arg)
	}
	buf.WriteString("\t\t},\n\t}\n}\n\n")
}

func writeSet(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, schemasPkg string) {
	resultType := schemasPkg + "." + singular
	defCols := defaultableColumns(table)
	if len(defCols) == 0 {
		writeStaticSet(buf, tableName, singular, table, d, resultType, schemasPkg)
		return
	}
	writeDynamicSet(buf, tableName, singular, table, d, resultType, schemasPkg, defCols)
}

func writeStaticSet(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, resultType, schemasPkg string) {
	insertCols := insertColumns(table)
	fmt.Fprintf(buf, "func Set%s(row *%s.%s) %s {\n", singular, schemasPkg, singular, runtimeQueryType(resultType))
	writeRequiredRow(buf, resultType)
	writePKValidation(buf, table, "row", resultType)
	args := make([]string, 0, len(insertCols))
	for _, col := range insertCols {
		args = append(args, "row."+parse.ToPascal(col))
	}
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n", resultType)
	fmt.Fprintf(buf, "\t\tSQL: `%s`,\n", buildCRUDStatements(tableName, table, d).Set)
	fmt.Fprintf(buf, "\t\tStmtName: %q,\n", "Set"+singular)
	buf.WriteString("\t\tArgs: []any{\n")
	for _, arg := range args {
		fmt.Fprintf(buf, "\t\t\t%s,\n", arg)
	}
	buf.WriteString("\t\t},\n\t}\n}\n\n")
}

func writeDynamicSet(buf *bytes.Buffer, tableName, singular string, table model.Table, d model.Dialect, resultType, schemasPkg string, defCols []string) {
	defType := defaultableFieldType(tableName)
	fmt.Fprintf(buf, "func Set%s(row *%s.%s, useDefault ...%s) %s {\n", singular, schemasPkg, singular, defType, runtimeQueryType(resultType))
	writeRequiredRow(buf, resultType)
	writeUseDefaultValidation(buf, tableName, resultType, defType, defCols)
	writePKValidationWithDefaults(buf, table, "row", resultType, defType, defCols)
	writeAutoValidation(buf, table, "row", resultType)
	for _, pk := range table.PrimaryKeys {
		if slices.Contains(defCols, pk) {
			field := defType + parse.ToPascal(pk)
			fmt.Fprintf(buf, "\tif useDefaultSet[%s] {\n", field)
			fmt.Fprintf(buf, "\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"cannot use default for primary key field %s in Set\")}\n", resultType, parse.ToPascal(pk))
			fmt.Fprintf(buf, "\t}\n")
		}
	}
	writeDynamicSetBody(buf, table, tableName, resultType, d, defType, defCols, "row")
}

func writePKValidation(buf *bytes.Buffer, table model.Table, rowVar, resultType string) {
	for _, pk := range table.PrimaryKeys {
		if slices.Contains(table.AutoFields, pk) {
			continue
		}
		field := parse.ToPascal(pk)
		fmt.Fprintf(buf, "\tif queryc.IsZeroValue(%s.%s) {\n", rowVar, field)
		fmt.Fprintf(buf, "\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"all primary key fields must be provided\")}\n\t}\n", resultType)
	}
}

func writePKValidationWithDefaults(buf *bytes.Buffer, table model.Table, rowVar, resultType, defType string, defCols []string) {
	for _, pk := range table.PrimaryKeys {
		if slices.Contains(table.AutoFields, pk) {
			continue
		}
		field := parse.ToPascal(pk)
		if slices.Contains(defCols, pk) {
			constName := defType + field
			fmt.Fprintf(buf, "\tif !useDefaultSet[%s] && queryc.IsZeroValue(%s.%s) {\n", constName, rowVar, field)
			fmt.Fprintf(buf, "\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"primary key field %s must be provided or listed in useDefault\")}\n\t}\n", resultType, field)
		} else {
			fmt.Fprintf(buf, "\tif queryc.IsZeroValue(%s.%s) {\n", rowVar, field)
			fmt.Fprintf(buf, "\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"all primary key fields must be provided\")}\n\t}\n", resultType)
		}
	}
}

func writeRequiredRow(buf *bytes.Buffer, resultType string) {
	fmt.Fprintf(buf, "\tif row == nil {\n\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"row cannot be nil\")}\n\t}\n", resultType)
}

func writeZeroPKValidation(buf *bytes.Buffer, table model.Table, rowVar, resultType string) {
	for _, pk := range table.PrimaryKeys {
		if slices.Contains(table.AutoFields, pk) {
			continue
		}
		field := parse.ToPascal(pk)
		fmt.Fprintf(buf, "\tif !queryc.IsZeroValue(%s.%s) {\n", rowVar, field)
		fmt.Fprintf(buf, "\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"primary key fields must be zero value in update\")}\n\t}\n", resultType)
	}
}

func writeAutoValidation(buf *bytes.Buffer, table model.Table, rowVar, resultType string) {
	for _, auto := range table.AutoFields {
		field := parse.ToPascal(auto)
		fmt.Fprintf(buf, "\tif !queryc.IsZeroValue(%s.%s) {\n", rowVar, field)
		fmt.Fprintf(buf, "\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"auto-generated field %s must be zero value\")}\n\t}\n", resultType, field)
	}
}

func writeUseDefaultValidation(buf *bytes.Buffer, tableName, resultType, defType string, defCols []string) {
	fmt.Fprintf(buf, "\tuseDefaultSet := make(map[%s]bool, len(useDefault))\n", defType)
	buf.WriteString("\tfor _, f := range useDefault {\n")
	buf.WriteString("\t\tswitch f {\n")
	for _, col := range defCols {
		fmt.Fprintf(buf, "\t\tcase %s%s:\n", defType, parse.ToPascal(col))
	}
	buf.WriteString("\t\tdefault:\n")
	fmt.Fprintf(buf, "\t\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"field %%q is not defaultable on table %%q\", f, %q)}\n", resultType, tableName)
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tif useDefaultSet[f] {\n")
	fmt.Fprintf(buf, "\t\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"duplicate default field %%q\", f)}\n", resultType)
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t\tuseDefaultSet[f] = true\n")
	buf.WriteString("\t}\n")
}

func writeDynamicColArgs(buf *bytes.Buffer, table model.Table, defCols []string, defType, rowVar string) {
	buf.WriteString("\tvar cols []string\n")
	buf.WriteString("\tvar args []any\n")
	for _, col := range sortedColumns(table) {
		if slices.Contains(table.AutoFields, col) {
			continue
		}
		field := parse.ToPascal(col)
		if slices.Contains(defCols, col) {
			constName := defType + field
			fmt.Fprintf(buf, "\tif !useDefaultSet[%s] {\n", constName)
			fmt.Fprintf(buf, "\t\tcols = append(cols, %q)\n", col)
			fmt.Fprintf(buf, "\t\targs = append(args, %s.%s)\n", rowVar, field)
			buf.WriteString("\t}\n")
		} else {
			fmt.Fprintf(buf, "\tcols = append(cols, %q)\n", col)
			fmt.Fprintf(buf, "\targs = append(args, %s.%s)\n", rowVar, field)
		}
	}
}

func writeDynamicColsOnly(buf *bytes.Buffer, table model.Table, defCols []string, defType string) {
	buf.WriteString("\tvar cols []string\n")
	for _, col := range sortedColumns(table) {
		if slices.Contains(table.AutoFields, col) {
			continue
		}
		field := parse.ToPascal(col)
		if slices.Contains(defCols, col) {
			constName := defType + field
			fmt.Fprintf(buf, "\tif !useDefaultSet[%s] {\n", constName)
			fmt.Fprintf(buf, "\t\tcols = append(cols, %q)\n", col)
			buf.WriteString("\t}\n")
		} else {
			fmt.Fprintf(buf, "\tcols = append(cols, %q)\n", col)
		}
	}
}

func writePlaceholders(buf *bytes.Buffer, d model.Dialect) {
	if d == model.DialectPostgres {
		buf.WriteString("\tplaceholders := make([]string, len(cols))\n")
		buf.WriteString("\tfor i := range cols {\n")
		buf.WriteString("\t\tplaceholders[i] = fmt.Sprintf(\"$%d\", i+1)\n")
		buf.WriteString("\t}\n")
	} else {
		buf.WriteString("\tplaceholders := make([]string, len(cols))\n")
		buf.WriteString("\tfor i := range cols {\n")
		buf.WriteString("\t\tplaceholders[i] = \"?\"\n")
		buf.WriteString("\t}\n")
	}
}

func writeUpdatesForSet(buf *bytes.Buffer, table model.Table, defCols []string, defType string, d model.Dialect) {
	buf.WriteString("\tvar updates []string\n")
	for _, col := range setUpdateColumns(table) {
		if col == "updated_at" {
			fmt.Fprintf(buf, "\tupdates = append(updates, %q)\n", col+" = "+dialect.CurrentTimestamp(d))
			continue
		}
		if slices.Contains(defCols, col) {
			constName := defType + parse.ToPascal(col)
			fmt.Fprintf(buf, "\tif !useDefaultSet[%s] {\n", constName)
			fmt.Fprintf(buf, "\t\tupdates = append(updates, %q)\n", col+" = excluded."+col)
			buf.WriteString("\t}\n")
		} else {
			fmt.Fprintf(buf, "\tupdates = append(updates, %q)\n", col+" = excluded."+col)
		}
	}
}

func writeDynamicInsertBody(buf *bytes.Buffer, table model.Table, tableName, resultType string, d model.Dialect, defType string, defCols []string, rowVar string) {
	insertCols := insertColumns(table)
	statement := buildCRUDStatements(tableName, table, d).Add
	singular := parse.ToSingular(parse.ToPascal(tableName))
	buf.WriteString("\tif len(useDefault) == 0 {\n")
	buf.WriteString("\t\treturn &querycruntime.Query[")
	buf.WriteString(resultType)
	buf.WriteString("]{\n")
	fmt.Fprintf(buf, "\t\t\tSQL: `%s`,\n", statement)
	fmt.Fprintf(buf, "\t\t\tStmtName: %q,\n", "Add"+singular)
	buf.WriteString("\t\t\tArgs: []any{\n")
	for _, col := range insertCols {
		fmt.Fprintf(buf, "\t\t\t\t%s.%s,\n", rowVar, parse.ToPascal(col))
	}
	buf.WriteString("\t\t\t},\n\t\t}\n\t}\n")
	writeDynamicColArgs(buf, table, defCols, defType, rowVar)
	fmt.Fprintf(buf, "\tif len(cols) == 0 {\n\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"no columns to insert: all columns were flagged as default\")}\n\t}\n", resultType)
	writePlaceholders(buf, d)
	fmt.Fprintf(buf, "\tquery := fmt.Sprintf(\"INSERT INTO %s (%%s) VALUES (%%s) RETURNING *\", strings.Join(cols, \", \"), strings.Join(placeholders, \", \"))\n", tableName)
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n\t\tSQL: query,\n\t\tArgs: args,\n\t}\n}\n\n", resultType)
}

func writeDynamicInsertManyBody(buf *bytes.Buffer, table model.Table, tableName, resultType string, d model.Dialect, defType string, defCols []string) {
	writeDynamicColsOnly(buf, table, defCols, defType)
	fmt.Fprintf(buf, "\tif len(cols) == 0 {\n\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"no columns to insert: all columns were flagged as default\")}\n\t}\n", resultType)
	buf.WriteString("\tvar valueSets []string\n")
	buf.WriteString("\tvar args []any\n")
	if d == model.DialectPostgres {
		buf.WriteString("\tcounter := 1\n")
	}
	buf.WriteString("\tfor _, row := range rows {\n")
	for _, col := range sortedColumns(table) {
		if slices.Contains(table.AutoFields, col) {
			continue
		}
		field := parse.ToPascal(col)
		if slices.Contains(defCols, col) {
			constName := defType + field
			fmt.Fprintf(buf, "\t\tif !useDefaultSet[%s] {\n", constName)
			fmt.Fprintf(buf, "\t\t\targs = append(args, row.%s)\n", field)
			buf.WriteString("\t\t}\n")
		} else {
			fmt.Fprintf(buf, "\t\targs = append(args, row.%s)\n", field)
		}
	}
	buf.WriteString("\t\tvar parts []string\n")
	if d == model.DialectPostgres {
		buf.WriteString("\t\tfor i := range cols {\n")
		buf.WriteString("\t\t\tparts = append(parts, fmt.Sprintf(\"$%d\", counter+i))\n")
		buf.WriteString("\t\t}\n")
		buf.WriteString("\t\tcounter += len(cols)\n")
	} else {
		buf.WriteString("\t\tfor range cols {\n")
		buf.WriteString("\t\t\tparts = append(parts, \"?\")\n")
		buf.WriteString("\t\t}\n")
	}
	buf.WriteString("\t\tvalueSets = append(valueSets, \"(\"+strings.Join(parts, \", \")+\")\")\n")
	buf.WriteString("\t}\n")
	fmt.Fprintf(buf, "\tquery := fmt.Sprintf(\"INSERT INTO %s (%%s) VALUES %%s RETURNING *\", strings.Join(cols, \", \"), strings.Join(valueSets, \", \"))\n", tableName)
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n\t\tSQL: query,\n\t\tArgs: args,\n\t}\n}\n\n", resultType)
}

func writeDynamicSetBody(buf *bytes.Buffer, table model.Table, tableName, resultType string, d model.Dialect, defType string, defCols []string, rowVar string) {
	insertCols := insertColumns(table)
	statement := buildCRUDStatements(tableName, table, d).Set
	singular := parse.ToSingular(parse.ToPascal(tableName))
	buf.WriteString("\tif len(useDefault) == 0 {\n")
	buf.WriteString("\t\treturn &querycruntime.Query[")
	buf.WriteString(resultType)
	buf.WriteString("]{\n")
	fmt.Fprintf(buf, "\t\t\tSQL: `%s`,\n", statement)
	fmt.Fprintf(buf, "\t\t\tStmtName: %q,\n", "Set"+singular)
	buf.WriteString("\t\t\tArgs: []any{\n")
	for _, col := range insertCols {
		fmt.Fprintf(buf, "\t\t\t\t%s.%s,\n", rowVar, parse.ToPascal(col))
	}
	buf.WriteString("\t\t\t},\n\t\t}\n\t}\n")
	writeDynamicColArgs(buf, table, defCols, defType, rowVar)
	fmt.Fprintf(buf, "\tif len(cols) == 0 {\n\t\treturn &querycruntime.Query[%s]{Error: fmt.Errorf(\"no columns to insert: all columns were flagged as default\")}\n\t}\n", resultType)
	writePlaceholders(buf, d)
	writeUpdatesForSet(buf, table, defCols, defType, d)
	fmt.Fprintf(buf, "\tquery := fmt.Sprintf(\"INSERT INTO %s (%%s) VALUES (%%s)\", strings.Join(cols, \", \"), strings.Join(placeholders, \", \"))\n", tableName)
	buf.WriteString("\tif len(updates) > 0 {\n")
	fmt.Fprintf(buf, "\t\tquery += fmt.Sprintf(\" ON CONFLICT(%s) DO UPDATE SET %%s\", strings.Join(updates, \", \"))\n", strings.Join(table.PrimaryKeys, ", "))
	buf.WriteString("\t}\n")
	buf.WriteString("\tquery += \" RETURNING *\"\n")
	fmt.Fprintf(buf, "\treturn &querycruntime.Query[%s]{\n\t\tSQL: query,\n\t\tArgs: args,\n\t}\n}\n\n", resultType)
}

func pkSignature(table model.Table, d model.Dialect) (string, string, []string) {
	var sigParts []string
	var where []string
	var names []string
	for i, pk := range table.PrimaryKeys {
		name := sanitizeName(pk)
		col := table.Columns[pk]
		sigParts = append(sigParts, name+" "+dialect.GoTypeForSQL(d, col.SQLType))
		where = append(where, fmt.Sprintf("%s = %s", pk, dialect.Placeholder(d, i+1)))
		names = append(names, name)
	}
	return strings.Join(sigParts, ", "), strings.Join(where, " AND "), names
}
