package parse

import (
	"fmt"
	"go/token"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
)

func parseQuotedValue(input string, start int) (string, int, error) {
	if start >= len(input) || input[start] != '"' {
		return "", start, fmt.Errorf("expected string literal")
	}

	for end := start + 1; end < len(input); end++ {
		if input[end] == '\\' {
			end++
			continue
		}
		if input[end] != '"' {
			continue
		}

		value, err := unescapeQuotedString(input[start+1 : end])
		if err != nil {
			return "", start, fmt.Errorf("invalid string: %w", err)
		}
		return value, end + 1, nil
	}

	return "", start, fmt.Errorf("unterminated string")
}

func findObjectEnd(input string, start int) int {
	depth := 0
	for i := start; i < len(input); i++ {
		if input[i] == '"' {
			_, next, err := parseQuotedValue(input, i)
			if err != nil {
				return -1
			}
			i = next - 1
			continue
		}

		switch input[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseImportObject(input string) (model.Import, error) {
	input = strings.TrimSpace(input)
	if len(input) < 2 || input[0] != '{' || input[len(input)-1] != '}' {
		return model.Import{}, fmt.Errorf("import metadata must be wrapped in braces")
	}

	var imp model.Import
	remaining := strings.TrimSpace(input[1 : len(input)-1])
	for remaining != "" {
		keyEnd := strings.IndexByte(remaining, ':')
		if keyEnd < 0 {
			return model.Import{}, fmt.Errorf("expected ':' in import metadata")
		}
		key := strings.TrimSpace(remaining[:keyEnd])
		remaining = strings.TrimSpace(remaining[keyEnd+1:])

		var value string
		if strings.HasPrefix(remaining, "\"") {
			var next int
			var err error
			value, next, err = parseQuotedValue(remaining, 0)
			if err != nil {
				return model.Import{}, fmt.Errorf("invalid %s in @import: %w", key, err)
			}
			remaining = strings.TrimSpace(remaining[next:])
		} else {
			valueEnd := strings.IndexByte(remaining, ',')
			if valueEnd < 0 {
				valueEnd = len(remaining)
			}
			value = strings.TrimSpace(remaining[:valueEnd])
			remaining = strings.TrimSpace(remaining[valueEnd:])
		}

		switch strings.ToLower(key) {
		case "path":
			imp.Path = value
		case "alias":
			imp.Alias = value
		case "schema":
			switch value {
			case "true":
				imp.Schema = true
			case "false":
				imp.Schema = false
			default:
				return model.Import{}, fmt.Errorf("schema in @import must be true or false")
			}
		default:
			return model.Import{}, fmt.Errorf("unknown @import key %q", key)
		}

		if remaining == "" {
			break
		}
		if remaining[0] != ',' {
			return model.Import{}, fmt.Errorf("expected ',' in import metadata")
		}
		remaining = strings.TrimSpace(remaining[1:])
		if remaining == "" {
			return model.Import{}, fmt.Errorf("trailing comma in import metadata")
		}
	}

	if err := validateImport(imp); err != nil {
		return model.Import{}, err
	}
	return imp, nil
}

func appendImport(imports []model.Import, candidate model.Import) ([]model.Import, error) {
	for _, existing := range imports {
		if existing.Path != candidate.Path {
			continue
		}
		if existing == candidate {
			return imports, nil
		}
		return nil, fmt.Errorf("conflicting declarations for import %q", candidate.Path)
	}
	return append(imports, candidate), nil
}

func validateImports(imports []model.Import) error {
	aliases := make(map[string]string)
	schemaPath := ""
	for _, imp := range imports {
		if err := validateImport(imp); err != nil {
			return err
		}
		if imp.Alias != "" {
			if existing, ok := aliases[imp.Alias]; ok && existing != imp.Path {
				return fmt.Errorf("import alias %q is used by both %q and %q", imp.Alias, existing, imp.Path)
			}
			aliases[imp.Alias] = imp.Path
		}
		if imp.Schema {
			if schemaPath != "" && schemaPath != imp.Path {
				return fmt.Errorf("multiple schema imports: %q and %q", schemaPath, imp.Path)
			}
			schemaPath = imp.Path
		}
	}
	return nil
}

func validateImport(imp model.Import) error {
	if strings.TrimSpace(imp.Path) == "" {
		return fmt.Errorf("@import path cannot be empty")
	}
	if imp.Alias != "" && (!token.IsIdentifier(imp.Alias) || token.Lookup(imp.Alias).IsKeyword()) {
		return fmt.Errorf("@import alias %q is not a valid Go identifier", imp.Alias)
	}
	if imp.Schema && imp.Alias == "" {
		return fmt.Errorf("schema @import requires an alias")
	}
	if imp.Alias == "querycruntime" || imp.Alias == "queryc" {
		return fmt.Errorf("@import alias %q is reserved", imp.Alias)
	}
	return nil
}
