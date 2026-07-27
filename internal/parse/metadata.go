package parse

import (
	"fmt"
	"strings"
	"unicode"
)

// metadata holds the parsed contents of a @query metadata object.
type metadata struct {
	Name        string
	Description string
	Deprecated  string
	Warnings    []string
}

// parseQueryMetadata parses the inner portion of @query({ ... }).
func parseQueryMetadata(s string) (metadata, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return metadata{}, fmt.Errorf("metadata must be wrapped in braces")
	}
	s = strings.TrimSpace(s[1 : len(s)-1])

	var result metadata
	for len(s) > 0 {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}

		colonIdx := strings.Index(s, ":")
		if colonIdx == -1 {
			return metadata{}, fmt.Errorf("expected ':' in metadata pair")
		}
		key := strings.TrimSpace(s[:colonIdx])
		s = strings.TrimSpace(s[colonIdx+1:])

		var value string
		if strings.HasPrefix(s, "\"") {
			end := 1
			for end < len(s) {
				if s[end] == '\\' {
					end += 2
					continue
				}
				if s[end] == '"' {
					break
				}
				end++
			}
			if end >= len(s) || s[end] != '"' {
				return metadata{}, fmt.Errorf("unterminated string in metadata")
			}
			raw := s[1:end]
			unescaped, err := unescapeQuotedString(raw)
			if err != nil {
				return metadata{}, fmt.Errorf("invalid string in metadata: %w", err)
			}
			value = unescaped
			s = strings.TrimSpace(s[end+1:])
		} else if strings.HasPrefix(s, "[") {
			depth := 1
			end := 1
			for end < len(s) && depth > 0 {
				switch s[end] {
				case '[':
					depth++
				case ']':
					depth--
				case '"':
					end++
					for end < len(s) && s[end] != '"' {
						if s[end] == '\\' {
							end++
						}
						end++
					}
				}
				end++
			}
			if depth != 0 {
				return metadata{}, fmt.Errorf("unterminated array in metadata")
			}
			value = s[:end]
			s = strings.TrimSpace(s[end:])
		} else {
			end := 0
			for end < len(s) && s[end] != ',' && !unicode.IsSpace(rune(s[end])) {
				end++
			}
			value = strings.TrimSpace(s[:end])
			s = strings.TrimSpace(s[end:])
		}

		switch strings.ToLower(key) {
		case "name":
			result.Name = value
		case "description":
			result.Description = value
		case "deprecated":
			result.Deprecated = value
		default:
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("unknown metadata key %q", key))
		}

		if strings.HasPrefix(s, ",") {
			s = strings.TrimSpace(s[1:])
		}
	}

	if result.Name == "" {
		return metadata{}, fmt.Errorf("metadata must contain 'name'")
	}

	return result, nil
}
