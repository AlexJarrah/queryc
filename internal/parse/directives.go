package parse

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/sqlscan"
)

// directive stores a queryc statement.
type directive struct {
	kind       string
	importSpec model.Import
	metadata   metadata
	body       string
}

// parseDirective parses a directive that starts immediately after the '@'.
func parseDirective(content string, start int) (directive, int, error) {
	nameEnd := start
	for nameEnd < len(content) && (unicode.IsLetter(rune(content[nameEnd])) || content[nameEnd] == '_') {
		nameEnd++
	}
	if nameEnd == start {
		return directive{}, 0, fmt.Errorf("expected directive name after @")
	}
	name := strings.ToLower(content[start:nameEnd])

	switch name {
	case "import":
		return parseImportDirective(content, nameEnd)
	case "query":
		return parseQueryDirective(content, nameEnd)
	default:
		return directive{}, 0, fmt.Errorf("unknown directive @%s", name)
	}
}

func parseImportDirective(content string, start int) (directive, int, error) {
	start = skipWhitespace(content, start)
	if start >= len(content) || content[start] != '(' {
		return directive{}, 0, fmt.Errorf("expected '(' after @import")
	}
	start++

	start = skipWhitespace(content, start)
	if start >= len(content) || content[start] != '{' {
		return directive{}, 0, fmt.Errorf("expected import object in @import")
	}

	objectEnd := findObjectEnd(content, start)
	if objectEnd == -1 {
		return directive{}, 0, fmt.Errorf("unterminated object in @import")
	}
	imp, err := parseImportObject(content[start : objectEnd+1])
	if err != nil {
		return directive{}, 0, err
	}

	end := skipWhitespace(content, objectEnd+1)
	if end >= len(content) || content[end] != ')' {
		return directive{}, 0, fmt.Errorf("expected ')' in @import")
	}

	return directive{kind: "import", importSpec: imp}, end + 1, nil
}

func parseQueryDirective(content string, start int) (directive, int, error) {
	start = skipWhitespace(content, start)
	if start >= len(content) || content[start] != '(' {
		return directive{}, 0, fmt.Errorf("expected '(' after @query")
	}
	start++

	// Find the matching ')' for the metadata wrapper, respecting nested
	// parentheses and string literals.
	parenDepth := 1
	inString := false
	var stringChar byte
	metaEnd := -1

	for i := start; i < len(content); i++ {
		c := content[i]
		if inString {
			if stringChar == '"' && c == '\\' {
				i++
				continue
			}
			if c == stringChar {
				inString = false
			}
			continue
		}

		// Skip SQL comments that may appear inside the metadata wrapper.
		if c == '-' && i+1 < len(content) && content[i+1] == '-' {
			for i < len(content) && content[i] != '\n' {
				i++
			}
			continue
		}
		if c == '/' && i+1 < len(content) && content[i+1] == '*' {
			i += 2
			for i+1 < len(content) {
				if content[i] == '*' && content[i+1] == '/' {
					i++
					break
				}
				i++
			}
			continue
		}
		if c == '"' || c == '\'' {
			inString = true
			stringChar = c
			continue
		}
		if c == '(' {
			parenDepth++
		} else if c == ')' {
			parenDepth--
			if parenDepth == 0 {
				metaEnd = i
				break
			}
		}
	}

	if metaEnd == -1 {
		return directive{}, 0, fmt.Errorf("unclosed metadata in @query")
	}

	meta, err := parseQueryMetadata(content[start:metaEnd])
	if err != nil {
		return directive{}, 0, fmt.Errorf("parse metadata: %w", err)
	}

	// Expect '{' to begin the SQL body.
	bodyStart := metaEnd + 1
	bodyStart = skipWhitespaceAndComments(content, bodyStart)
	if bodyStart >= len(content) || content[bodyStart] != '{' {
		return directive{}, 0, fmt.Errorf("expected '{' after @query metadata")
	}
	bodyStart++

	// Find the matching '}' for the SQL body, respecting nested braces
	// and string literals so @struct({ ... }) inside the body works.
	braceDepth := 1
	bodyEnd := -1

	for i := bodyStart; i < len(content); i++ {
		if next, skipped := sqlscan.SkipLiteralOrComment(content, i); skipped {
			i = next - 1
			continue
		}
		c := content[i]
		if c == '{' {
			braceDepth++
		} else if c == '}' {
			braceDepth--
			if braceDepth == 0 {
				bodyEnd = i
				break
			}
		}
	}

	if bodyEnd == -1 {
		return directive{}, 0, fmt.Errorf("unclosed body in @query %q", meta.Name)
	}

	body := strings.TrimSpace(content[bodyStart:bodyEnd])

	return directive{kind: "query", metadata: meta, body: body}, bodyEnd + 1, nil
}

func skipWhitespace(s string, i int) int {
	for i < len(s) && unicode.IsSpace(rune(s[i])) {
		i++
	}
	return i
}

// skipWhitespaceAndComments advances past whitespace and SQL comments (-- and /* */).
func skipWhitespaceAndComments(s string, i int) int {
	for i < len(s) {
		if unicode.IsSpace(rune(s[i])) {
			i++
			continue
		}
		if i+1 < len(s) && s[i] == '-' && s[i+1] == '-' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) {
				if s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		break
	}
	return i
}
