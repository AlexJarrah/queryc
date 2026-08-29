package codegen

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/model"
)

func writeImports(buf *bytes.Buffer, userImports []model.Import, d model.Dialect, schema model.Schema, queries []model.AnalyzedQuery) {
	imports := map[string]model.Import{}
	runtimeNeeded := len(schema.Tables) > 0 || len(queries) > 0 || d == model.DialectPostgres
	sharedRuntimeNeeded := len(schema.Tables) > 0

	if d == model.DialectPostgres {
		imports["context"] = model.Import{Path: "context"}
	}

	if len(schema.Tables) > 0 || d == model.DialectPostgres && hasPreparedQueries(queries) {
		imports["fmt"] = model.Import{Path: "fmt"}
	}

	if len(schema.Tables) > 0 || hasDynamicQueries(queries) {
		imports["strings"] = model.Import{Path: "strings"}
	}

	for _, imp := range userImports {
		imports[imp.Path] = imp
	}
	if len(imports) == 0 && !runtimeNeeded {
		return
	}

	var stdlib, thirdParty []model.Import
	for _, imp := range imports {
		if isStdlibImport(imp.Path) {
			stdlib = append(stdlib, imp)
		} else {
			thirdParty = append(thirdParty, imp)
		}
	}
	sortImports(stdlib)
	sortImports(thirdParty)

	buf.WriteString("import (\n")
	for _, imp := range stdlib {
		writeImport(buf, imp)
	}
	buf.WriteString("\n")

	if runtimeNeeded {
		fmt.Fprintf(buf, "\tquerycruntime %q\n", dialect.RuntimeImportPath(d))
	}
	if sharedRuntimeNeeded {
		fmt.Fprintf(buf, "\tqueryc %q\n", dialect.SharedRuntimeImportPath())
	}
	for _, imp := range thirdParty {
		writeImport(buf, imp)
	}

	buf.WriteString(")\n\n")
}

func sortImports(imports []model.Import) {
	sort.Slice(imports, func(i, j int) bool {
		return imports[i].Path < imports[j].Path
	})
}

func writeImport(buf *bytes.Buffer, imp model.Import) {
	if imp.Alias == "" {
		fmt.Fprintf(buf, "\t%q\n", imp.Path)
		return
	}
	fmt.Fprintf(buf, "\t%s %q\n", imp.Alias, imp.Path)
}

func hasPreparedQueries(queries []model.AnalyzedQuery) bool {
	for _, analyzed := range queries {
		if shouldPrepareQuery(analyzed.Query) {
			return true
		}
	}
	return false
}

func hasDynamicQueries(queries []model.AnalyzedQuery) bool {
	for _, analyzed := range queries {
		if len(analyzed.Query.Hashtags) > 0 {
			return true
		}
	}
	return false
}

func isStdlibImport(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
