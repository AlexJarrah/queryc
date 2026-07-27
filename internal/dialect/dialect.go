package dialect

import (
	"strconv"

	"github.com/AlexJarrah/queryc/internal/model"
)

var postgresToGoType = map[string]string{
	"UUID":                        "uuid.UUID",
	"TEXT":                        "string",
	"VARCHAR":                     "string",
	"CHAR":                        "string",
	"STRING":                      "string",
	"BOOLEAN":                     "bool",
	"BOOL":                        "bool",
	"SMALLINT":                    "int16",
	"INT2":                        "int16",
	"INTEGER":                     "int32",
	"INT":                         "int32",
	"INT4":                        "int32",
	"BIGINT":                      "int64",
	"INT8":                        "int64",
	"SMALLSERIAL":                 "int16",
	"SERIAL":                      "int32",
	"BIGSERIAL":                   "int64",
	"TIMESTAMP":                   "time.Time",
	"TIMESTAMPTZ":                 "time.Time",
	"TIMESTAMP WITH TIME ZONE":    "time.Time",
	"TIMESTAMP WITHOUT TIME ZONE": "time.Time",
	"TIME":                        "time.Time",
	"TIME WITH TIME ZONE":         "time.Time",
	"TIME WITHOUT TIME ZONE":      "time.Time",
	"DATE":                        "time.Time",
	"JSONB":                       "string",
	"JSON":                        "string",
	"BYTEA":                       "[]byte",
	"DOUBLE PRECISION":            "float64",
	"FLOAT8":                      "float64",
	"FLOAT4":                      "float32",
	"NUMERIC":                     "float64",
	"DECIMAL":                     "float64",
	"REAL":                        "float32",
	"BLOB":                        "[]byte",
	"DATETIME":                    "time.Time",
	"XML":                         "string",
	"UUID[]":                      "[]uuid.UUID",
	"TEXT[]":                      "[]string",
	"VARCHAR[]":                   "[]string",
	"BOOLEAN[]":                   "[]bool",
	"BOOL[]":                      "[]bool",
	"SMALLINT[]":                  "[]int16",
	"INTEGER[]":                   "[]int32",
	"INT[]":                       "[]int32",
	"BIGINT[]":                    "[]int64",
	"REAL[]":                      "[]float32",
	"DOUBLE PRECISION[]":          "[]float64",
}

var sqliteToGoType = map[string]string{
	"TEXT":             "string",
	"VARCHAR":          "string",
	"CHAR":             "string",
	"STRING":           "string",
	"BOOLEAN":          "bool",
	"BOOL":             "bool",
	"INTEGER":          "int64",
	"INT":              "int64",
	"BIGINT":           "int64",
	"SMALLINT":         "int64",
	"REAL":             "float64",
	"FLOAT":            "float64",
	"DOUBLE":           "float64",
	"DOUBLE PRECISION": "float64",
	"NUMERIC":          "float64",
	"DECIMAL":          "float64",
	"BLOB":             "[]byte",
	"UUID":             "uuid.UUID",
	"TIMESTAMP":        "time.Time",
	"DATETIME":         "time.Time",
	"DATE":             "time.Time",
	"JSON":             "string",
	"JSONB":            "string",
}

// TypeMap returns the Go type mapping for d.
func TypeMap(d model.Dialect) map[string]string {
	if d == model.DialectSQLite {
		return sqliteToGoType
	}
	return postgresToGoType
}

// GoTypeForSQL maps a normalized SQL type to its generated Go type.
func GoTypeForSQL(d model.Dialect, sqlType string) string {
	if v, ok := TypeMap(d)[sqlType]; ok {
		return v
	}
	return "any"
}

// CurrentTimestamp returns the dialect's current-time SQL expression.
func CurrentTimestamp(d model.Dialect) string {
	if d == model.DialectSQLite {
		return "datetime('now')"
	}
	return "NOW()"
}

// PackageName returns the generated package name for d.
func PackageName(d model.Dialect) string {
	if d == model.DialectSQLite {
		return "sqlite"
	}
	return "postgres"
}

// RuntimeImportPath returns the runtime package imported by generated code.
func RuntimeImportPath(d model.Dialect) string {
	if d == model.DialectSQLite {
		return "github.com/AlexJarrah/queryc/runtime/sqlite"
	}
	return "github.com/AlexJarrah/queryc/runtime/postgres"
}

// SharedRuntimeImportPath returns the dialect-independent runtime package.
func SharedRuntimeImportPath() string {
	return "github.com/AlexJarrah/queryc/runtime"
}

// Placeholder returns the dialect placeholder for a one-based argument index.
func Placeholder(d model.Dialect, index int) string {
	if d == model.DialectSQLite {
		return "?"
	}
	return "$" + strconv.Itoa(index)
}
