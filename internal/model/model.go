package model

// Dialect identifies a supported SQL dialect.
type Dialect uint8

// Supported SQL dialects.
const (
	DialectSQLite Dialect = iota
	DialectPostgres
)

// Column describes one schema column.
type Column struct {
	Name       string
	SQLType    string
	Nullable   bool
	HasDefault bool
}

// Table describes a parsed schema table.
type Table struct {
	Name         string
	Columns      map[string]Column
	PrimaryKeys  []string
	AutoFields   []string
	HasUpdatedAt bool
	Aliases      []string
}

// Schema contains tables keyed by SQL name.
type Schema struct {
	Tables map[string]Table
}

// Import describes a Go import.
type Import struct {
	Path   string
	Alias  string
	Schema bool
}

// Param describes one parameterized SQL value.
type Param struct {
	Name     string
	Type     string
	Explicit bool
	Index    int
}

// Hashtag describes one dynamic SQL fragment.
type Hashtag struct {
	Name     string
	Type     string
	IsSlice  bool
	Explicit bool
	Index    int
}

// Query is a parsed named query.
type Query struct {
	Name            string
	Description     string
	Deprecated      string
	Warnings        []string
	RawSQL          string
	SQL             string
	Params          []Param
	Hashtags        []Hashtag
	HashtagSequence []string
	SourceSQL       string
}

// QueryFile contains the imports and named queries in a queryc file.
type QueryFile struct {
	Imports []Import
	Queries []Query
}

// ResultField describes one analyzed result column.
type ResultField struct {
	Name                string
	DBName              string
	GoType              string
	Nullable            bool
	Serialization       string
	ExplicitType        string
	IsExpression        bool
	Skip                bool
	FromTable           string
	FromColumn          string
	TableAlias          string
	GeneratedStructName string
	GeneratedStructKind string
	GeneratedFields     []GeneratedJSONField
}

// GeneratedJSONField describes a field nested in generated JSON result types.
type GeneratedJSONField struct {
	JSONName   string
	FieldName  string
	GoType     string
	Nullable   bool
	FromTable  string
	FromColumn string
	TableAlias string
}

// EmbeddedTable describes an application row type embedded in a result.
type EmbeddedTable struct {
	TableName  string
	StructName string
	IsNullable bool
	TableAlias string
}

// AnalyzedQuery contains the type information required for code generation.
type AnalyzedQuery struct {
	Query             Query
	ResultStructName  string
	ShouldEmitResult  bool
	Fields            []ResultField
	EmbeddedTables    []EmbeddedTable
	NullableTables    map[string]bool
	NullableAliases   map[string]bool
	GeneratedJSONSeen map[string]bool
}
