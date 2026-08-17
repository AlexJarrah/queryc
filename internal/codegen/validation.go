package codegen

import (
	"fmt"
	goparser "go/parser"
	"go/token"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

type tableDecl struct {
	name string
	kind string
}

func validateGenerationInputs(schema model.Schema, queries []model.AnalyzedQuery, d model.Dialect) error {
	declared := make(map[string]string)
	declare := func(name, kind string) error {
		if !token.IsIdentifier(name) {
			return fmt.Errorf("%s generates invalid Go identifier %q", kind, name)
		}

		if previous, exists := declared[name]; exists {
			return fmt.Errorf("%s generates duplicate Go identifier %q (already used by %s)", kind, name, previous)
		}

		declared[name] = kind
		return nil
	}

	builtins := []string{"Table", "SortDirection", "ASC", "DESC"}
	if d == model.DialectPostgres {
		builtins = append(builtins, "PrepareStatements")
	}

	for _, name := range builtins {
		if err := declare(name, "generated runtime API"); err != nil {
			return err
		}
	}

	for _, tableName := range sortedTables(schema) {
		table := schema.Tables[tableName]
		structName := parse.ToPascal(tableName)
		singular := parse.ToSingular(structName)
		fieldType := structName + "Field"

		tableDecls := []tableDecl{
			{name: structName, kind: fmt.Sprintf("table %q constant", tableName)},
			{name: fieldType, kind: fmt.Sprintf("table %q field type", tableName)},
			{name: "Add" + singular, kind: fmt.Sprintf("table %q CRUD function", tableName)},
			{name: "AddMany" + structName, kind: fmt.Sprintf("table %q CRUD function", tableName)},
		}

		if len(table.PrimaryKeys) > 0 {
			tableDecls = append(
				tableDecls,
				tableDecl{name: "GetAll" + structName, kind: fmt.Sprintf("table %q CRUD function", tableName)},
				tableDecl{name: "Get" + singular, kind: fmt.Sprintf("table %q CRUD function", tableName)},
				tableDecl{name: "Get" + structName, kind: fmt.Sprintf("table %q CRUD function", tableName)},
				tableDecl{name: "Delete" + singular, kind: fmt.Sprintf("table %q CRUD function", tableName)},
				tableDecl{name: "Set" + singular, kind: fmt.Sprintf("table %q CRUD function", tableName)},
			)
			if len(table.PrimaryKeys) > 1 {
				tableDecls = append(tableDecls, tableDecl{name: singular + "PKs", kind: fmt.Sprintf("table %q composite key type", tableName)})
			}
			if len(nonPKColumns(table)) > 0 {
				tableDecls = append(tableDecls, tableDecl{name: "Update" + singular, kind: fmt.Sprintf("table %q CRUD function", tableName)})
			}
		}

		for _, declaration := range tableDecls {
			if err := declare(declaration.name, declaration.kind); err != nil {
				return err
			}
		}

		columnFields := make(map[string]string)
		for _, column := range sortedColumns(table) {
			fieldName := parse.ToPascal(column)
			if !token.IsIdentifier(fieldName) {
				return fmt.Errorf("column %q on table %q generates invalid Go field %q", column, tableName, fieldName)
			}

			if previous, exists := columnFields[fieldName]; exists {
				return fmt.Errorf("columns %q and %q on table %q both generate Go field %q", previous, column, tableName, fieldName)
			}

			columnFields[fieldName] = column
			if err := declare(fieldType+fieldName, fmt.Sprintf("column %q on table %q constant", column, tableName)); err != nil {
				return err
			}
		}
	}

	for _, analyzed := range queries {
		q := analyzed.Query
		if err := declare(q.Name, fmt.Sprintf("query %q function", q.Name)); err != nil {
			return err
		}

		arguments := make(map[string]string)
		if len(q.Params) > 1 {
			if err := declare(q.Name+"Params", fmt.Sprintf("query %q params type", q.Name)); err != nil {
				return err
			}

			fields := make(map[string]string)
			for _, param := range q.Params {
				fieldName := parse.ToPascal(param.Name)
				if previous, exists := fields[fieldName]; exists {
					return fmt.Errorf("parameters %q and %q on query %q both generate Go field %q", previous, param.Name, q.Name, fieldName)
				}
				fields[fieldName] = param.Name
			}

			arguments["params"] = "parameter struct"
		} else if len(q.Params) == 1 {
			arguments[sanitizeName(q.Params[0].Name)] = "parameter"
		}

		for _, param := range q.Params {
			if _, err := goparser.ParseExpr(param.Type); err != nil {
				return fmt.Errorf("parameter %q on query %q has invalid Go type %q: %w", param.Name, q.Name, param.Type, err)
			}
		}

		for _, hashtag := range q.Hashtags {
			name := sanitizeName(hashtag.Name)
			if previous, exists := arguments[name]; exists {
				return fmt.Errorf("SQL fragment %q on query %q conflicts with %s argument %q", hashtag.Name, q.Name, previous, name)
			}

			arguments[name] = "SQL fragment"
			if _, err := goparser.ParseExpr(hashtag.Type); err != nil {
				return fmt.Errorf("SQL fragment %q on query %q has invalid Go type %q: %w", hashtag.Name, q.Name, hashtag.Type, err)
			}
		}

		if len(q.Hashtags) > 0 {
			if previous, exists := arguments["sql"]; exists {
				return fmt.Errorf("query %q cannot use %s argument name %q with dynamic SQL fragments", q.Name, previous, "sql")
			}
		}

		if analyzed.ShouldEmitResult && analyzed.ResultStructName != "" {
			if err := declare(analyzed.ResultStructName, fmt.Sprintf("query %q result type", q.Name)); err != nil {
				return err
			}

			resultFields := make(map[string]string)
			for _, embedded := range analyzed.EmbeddedTables {
				if !token.IsIdentifier(embedded.StructName) {
					return fmt.Errorf("query %q generates invalid embedded Go field %q", q.Name, embedded.StructName)
				}

				if previous, exists := resultFields[embedded.StructName]; exists {
					return fmt.Errorf("query %q generates duplicate result field %q from %s and table %q", q.Name, embedded.StructName, previous, embedded.TableName)
				}

				resultFields[embedded.StructName] = "table " + embedded.TableName
			}

			for _, field := range analyzed.Fields {
				if field.Skip {
					continue
				}

				if !token.IsIdentifier(field.Name) {
					return fmt.Errorf("query %q column %q generates invalid Go field %q; add a valid SQL alias with AS", q.Name, field.DBName, field.Name)
				}

				if previous, exists := resultFields[field.Name]; exists {
					return fmt.Errorf("query %q generates duplicate result field %q from %s and column %q", q.Name, field.Name, previous, field.DBName)
				}

				resultFields[field.Name] = "column " + field.DBName
			}

			for _, field := range analyzed.Fields {
				if field.GeneratedStructKind == "" || field.GeneratedStructName == "" {
					continue
				}

				if err := declare(field.GeneratedStructName, fmt.Sprintf("query %q JSON result type", q.Name)); err != nil {
					return err
				}

				jsonFields := make(map[string]string)
				for _, generated := range field.GeneratedFields {
					if !token.IsIdentifier(generated.FieldName) {
						return fmt.Errorf("JSON key %q on query %q generates invalid Go field %q", generated.JSONName, q.Name, generated.FieldName)
					}

					if previous, exists := jsonFields[generated.FieldName]; exists {
						return fmt.Errorf("JSON keys %q and %q on query %q both generate Go field %q", previous, generated.JSONName, q.Name, generated.FieldName)
					}

					jsonFields[generated.FieldName] = generated.JSONName
				}
			}
		}
	}

	return nil
}

func resolveSchemaPackage(schema model.Schema, imports []model.Import) (string, error) {
	if len(schema.Tables) == 0 {
		return "", nil
	}

	var schemaImport *model.Import
	for _, imp := range imports {
		if !imp.Schema {
			continue
		}

		if schemaImport != nil {
			return "", fmt.Errorf("multiple schema imports: %q and %q", schemaImport.Path, imp.Path)
		}

		candidate := imp
		schemaImport = &candidate
	}

	if schemaImport == nil {
		return "", fmt.Errorf("schema definitions require @import({ path: \"...\", alias: \"...\", schema: true })")
	}

	if !token.IsIdentifier(schemaImport.Alias) {
		return "", fmt.Errorf("schema import %q requires a valid alias", schemaImport.Path)
	}

	return schemaImport.Alias, nil
}
