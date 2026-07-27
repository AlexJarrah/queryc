package parse

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/AlexJarrah/queryc/internal/dialect"
	"github.com/AlexJarrah/queryc/internal/feature"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/sqlscan"
)

// buildQuery runs body through feature conversion, hashtag and param
// extraction, and final sanitisation.
func buildQuery(meta metadata, body string, schema model.Schema, d model.Dialect) (model.Query, error) {
	registry := feature.NewRegistry()

	// Compact SQL body by removing comments
	body = stripSQLComments(body)

	expandedSQL, err := registry.Convert(body, d)
	if err != nil {
		return model.Query{}, fmt.Errorf("%s: feature conversion failed: %w", meta.Name, err)
	}

	sqlWithTags, hashtags, hashtagSequence, err := parseHashtags(expandedSQL)
	if err != nil {
		return model.Query{}, fmt.Errorf("%s: %w", meta.Name, err)
	}
	finalSQL, params, err := parseParams(sqlWithTags, body, schema, d)
	if err != nil {
		return model.Query{}, fmt.Errorf("%s: %w", meta.Name, err)
	}
	finalSQL = StripQueryCMarkers(stripTypeAnnotations(finalSQL))

	return model.Query{
		Name:            meta.Name,
		Description:     meta.Description,
		Deprecated:      meta.Deprecated,
		Warnings:        meta.Warnings,
		RawSQL:          expandedSQL,
		SQL:             finalSQL,
		Params:          params,
		Hashtags:        hashtags,
		HashtagSequence: hashtagSequence,
		SourceSQL:       body,
	}, nil
}

// stripSQLComments removes -- line comments and /* */ block comments from sql,
// respecting single and double quoted literals so that comment delimiters
// inside strings are preserved.
func stripSQLComments(sql string) string {
	var out strings.Builder
	for i := 0; i < len(sql); i++ {
		if next, skipped := sqlscan.SkipLiteralOrComment(sql, i); skipped {
			isLineComment := sql[i] == '-' && i+1 < len(sql) && sql[i+1] == '-'
			isBlockComment := sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*'
			if isLineComment {
				if next < len(sql) && sql[next] == '\n' {
					out.WriteByte('\n')
					i = next
				} else {
					i = next - 1
				}
				continue
			}
			if isBlockComment {
				i = next - 1
				continue
			}
			out.WriteString(sql[i:next])
			i = next - 1
			continue
		}
		out.WriteByte(sql[i])
	}
	return out.String()
}

// parseHashtags replaces dynamic SQL fragments with a sentinel delimiter and
// stores them in declaration order. String literals and comments are ignored.
func parseHashtags(sql string) (string, []model.Hashtag, []string, error) {
	hashtags := map[string]model.Hashtag{}
	var sequence []string
	counter := 0
	var parseErr error

	out := replaceOutsideLiterals(sql, hashtagRe, func(full string, groups []string) string {
		if parseErr != nil {
			return full
		}
		name := groups[0]
		isSlice := groups[1] == "[]"
		explicitType := groups[2]
		sequence = append(sequence, name)

		if existing, ok := hashtags[name]; ok {
			if existing.Explicit && explicitType != "" && existing.Type != explicitType {
				parseErr = fmt.Errorf("SQL fragment #%s has conflicting types %q and %q", name, existing.Type, explicitType)
				return full
			}
			if existing.IsSlice != isSlice {
				parseErr = fmt.Errorf("SQL fragment #%s is used with both scalar and slice syntax", name)
				return full
			}
			if !existing.Explicit && explicitType != "" {
				existing.Type = explicitType
				existing.Explicit = true
				hashtags[name] = existing
			}
			return HashtagDelimiter
		}

		hashtags[name] = model.Hashtag{
			Name:     name,
			Type:     explicitType,
			IsSlice:  isSlice,
			Explicit: explicitType != "",
			Index:    counter,
		}
		counter++
		return HashtagDelimiter
	})
	if parseErr != nil {
		return "", nil, nil, parseErr
	}

	keys := make([]string, 0, len(hashtags))
	for key := range hashtags {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return hashtags[keys[i]].Index < hashtags[keys[j]].Index
	})

	result := make([]model.Hashtag, 0, len(keys))
	for _, key := range keys {
		h := hashtags[key]
		if h.Type == "" {
			if h.IsSlice {
				h.Type = "[]string"
			} else {
				h.Type = "string"
			}
		}
		result = append(result, h)
	}

	return out, result, sequence, nil
}

// parseParams replaces named parameters with dialect placeholders and
// deduplicates repeated usages. String literals and comments are ignored.
func parseParams(sql, originalSQL string, schema model.Schema, d model.Dialect) (string, []model.Param, error) {
	params := map[string]model.Param{}
	counter := 1
	var parseErr error

	out := replaceOutsideLiterals(sql, paramRe, func(full string, groups []string) string {
		if parseErr != nil {
			return full
		}
		name := groups[0]
		explicitType := groups[1]

		if existing, ok := params[name]; ok {
			if existing.Explicit && explicitType != "" && existing.Type != explicitType {
				parseErr = fmt.Errorf("parameter $%s has conflicting types %q and %q", name, existing.Type, explicitType)
				return full
			}
			if explicitType != "" && !existing.Explicit {
				existing.Type = explicitType
				existing.Explicit = true
				params[name] = existing
			}
		} else {
			params[name] = model.Param{
				Name:     name,
				Type:     inferParamType(name, explicitType, originalSQL, schema, d),
				Explicit: explicitType != "",
				Index:    counter,
			}
			counter++
		}

		param := params[name]
		if explicitType == "string" {
			return fmt.Sprintf("CAST(%s AS TEXT)", dialect.Placeholder(d, param.Index))
		}
		return dialect.Placeholder(d, param.Index)
	})
	if parseErr != nil {
		return "", nil, parseErr
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return params[keys[i]].Index < params[keys[j]].Index
	})

	result := make([]model.Param, 0, len(keys))
	for _, key := range keys {
		result = append(result, params[key])
	}

	return out, result, nil
}

// inferParamType attempts to detect the Go type of a parameter.
func inferParamType(name, explicitType, sql string, schema model.Schema, d model.Dialect) string {
	if explicitType != "" {
		return explicitType
	}

	re := regexp.MustCompile(`([a-zA-Z0-9_]+)\s*(?:=|!=|<>|LIKE|ILIKE|>|<|>=|<=)\s*\$` + regexp.QuoteMeta(name))
	match := re.FindStringSubmatch(sql)
	if match == nil {
		return "any"
	}

	columnName := match[1]
	for _, table := range schema.Tables {
		if col, ok := table.Columns[columnName]; ok {
			return dialect.GoTypeForSQL(d, col.SQLType)
		}
	}
	return "any"
}

// stripTypeAnnotations removes type castings (e.g. :uuid.UUID) that are
// invalid SQL for the parser's benefit.
func stripTypeAnnotations(sql string) string {
	var out strings.Builder
	for i := 0; i < len(sql); i++ {
		if sql[i] != ':' {
			out.WriteByte(sql[i])
			continue
		}
		if i+1 < len(sql) && sql[i+1] == ':' {
			out.WriteString("::")
			i++
			continue
		}
		i++
		if i < len(sql) && sql[i] == '*' {
			i++
		}
		for i < len(sql) && (sql[i] == '.' || sql[i] == '_' || sql[i] == '[' || sql[i] == ']' || sql[i] >= '0' && sql[i] <= '9' || sql[i] >= 'A' && sql[i] <= 'Z' || sql[i] >= 'a' && sql[i] <= 'z') {
			i++
		}
		i--
	}
	return out.String()
}

// StripQueryCMarkers removes internal sentinel comments used by the feature
// layer to flag JSON serialised columns.
func StripQueryCMarkers(sql string) string {
	return queryCMarkerRe.ReplaceAllString(sql, "")
}
