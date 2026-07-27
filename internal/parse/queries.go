package parse

import (
	"fmt"
	"regexp"

	"github.com/AlexJarrah/queryc/internal/model"
)

const (
	// HashtagDelimiter replaces #fragments in rewritten SQL before generation
	// splices fragment arguments back in.
	HashtagDelimiter = "__HASHTAG_DELIMITER__"
)

var (
	hashtagRe      = regexp.MustCompile(`#([a-zA-Z0-9_]+)(\[\])?(?::([a-zA-Z0-9_.\[\]]+))?`)
	paramRe        = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)(?::(\*?[a-zA-Z0-9_.\[\]]+))?`)
	queryCMarkerRe = regexp.MustCompile(`/\*queryc_[^*]+\*/`)
	errorPosRe     = regexp.MustCompile(`at position (\d+)`)
)

// QueryFile parses a queryc file into imports and named queries.
func QueryFile(content string, schema model.Schema, d model.Dialect) (model.QueryFile, error) {
	var imports []model.Import
	var queries []model.Query

	i := 0
	for i < len(content) {
		i = skipWhitespaceAndComments(content, i)
		if i >= len(content) {
			break
		}

		if content[i] != '@' {
			return model.QueryFile{}, fmt.Errorf("unexpected character %q at position %d; expected @import or @query", content[i], i)
		}

		directive, next, err := parseDirective(content, i+1)
		if err != nil {
			return model.QueryFile{}, err
		}
		i = next

		switch directive.kind {
		case "import":
			imports, err = appendImport(imports, directive.importSpec)
			if err != nil {
				return model.QueryFile{}, err
			}
		case "query":
			query, err := buildQuery(directive.metadata, directive.body, schema, d)
			if err != nil {
				return model.QueryFile{}, err
			}
			queries = append(queries, query)
		default:
			return model.QueryFile{}, fmt.Errorf("unknown directive @%s", directive.kind)
		}
	}

	if err := ensureUniqueQueryNames(queries); err != nil {
		return model.QueryFile{}, err
	}
	if err := validateImports(imports); err != nil {
		return model.QueryFile{}, err
	}

	return model.QueryFile{Imports: imports, Queries: queries}, nil
}

func ensureUniqueQueryNames(queries []model.Query) error {
	seen := map[string]bool{}
	for _, q := range queries {
		if seen[q.Name] {
			return fmt.Errorf("duplicate query name %q", q.Name)
		}
		seen[q.Name] = true
	}
	return nil
}
