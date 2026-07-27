package codegen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/model"
)

type crudStatements struct {
	Add    string
	Get    string
	GetAll string
	Delete string
	Update string
	Set    string
}

func buildCRUDStatements(tableName string, table model.Table, d model.Dialect) crudStatements {
	insertCols := insertColumns(table)
	insertPlaceholders := make([]string, len(insertCols))
	for i := range insertCols {
		insertPlaceholders[i] = dialect.Placeholder(d, i+1)
	}

	statements := crudStatements{
		Add: fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s) RETURNING *",
			tableName,
			strings.Join(insertCols, ", "),
			strings.Join(insertPlaceholders, ", "),
		),
		GetAll: "SELECT * FROM " + tableName,
	}
	if len(table.PrimaryKeys) == 0 {
		return statements
	}

	_, where, _ := pkSignature(table, d)
	statements.Get = fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT 1", tableName, where)
	statements.Delete = fmt.Sprintf("DELETE FROM %s WHERE %s", tableName, where)
	statements.Update = buildUpdateStatement(tableName, table, where, d)
	statements.Set = buildSetStatement(tableName, table, insertCols, insertPlaceholders, d)
	return statements
}

func buildUpdateStatement(tableName string, table model.Table, where string, d model.Dialect) string {
	updateCols := nonPKColumns(table)
	if len(updateCols) == 0 {
		return ""
	}

	sets := make([]string, 0, len(updateCols))
	paramIndex := len(table.PrimaryKeys) + 1
	for _, col := range updateCols {
		if col == "updated_at" && table.HasUpdatedAt {
			sets = append(sets, col+" = "+dialect.CurrentTimestamp(d))
			continue
		}
		sets = append(sets, fmt.Sprintf("%s = %s", col, dialect.Placeholder(d, paramIndex)))
		paramIndex++
	}
	return fmt.Sprintf("UPDATE %s SET %s WHERE %s", tableName, strings.Join(sets, ", "), where)
}

func buildSetStatement(tableName string, table model.Table, insertCols, placeholders []string, d model.Dialect) string {
	updates := make([]string, 0, len(table.Columns))
	for _, col := range sortedColumns(table) {
		if slices.Contains(table.PrimaryKeys, col) || col == "created_at" {
			continue
		}

		if col == "updated_at" && table.HasUpdatedAt {
			updates = append(updates, col+" = "+dialect.CurrentTimestamp(d))
			continue
		}

		updates = append(updates, col+" = excluded."+col)
	}

	statement := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		tableName,
		strings.Join(insertCols, ", "),
		strings.Join(placeholders, ", "),
	)
	if len(updates) > 0 {
		statement += fmt.Sprintf(
			" ON CONFLICT(%s) DO UPDATE SET %s",
			strings.Join(table.PrimaryKeys, ", "),
			strings.Join(updates, ", "),
		)
	}
	return statement + " RETURNING *"
}
