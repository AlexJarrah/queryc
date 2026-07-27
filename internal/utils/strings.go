package utils

import (
	"strings"

	"github.com/AlexJarrah/queryc/internal/sqlscan"
)

// SplitByDelimiter splits s on delimiter while respecting nested SQL
// expressions, quoted values, dollar quoted values, and comments.
func SplitByDelimiter(s string, delimiter rune) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	delimiterByte := byte(delimiter)

	for i := 0; i < len(s); i++ {
		if next, skipped := sqlscan.SkipLiteralOrComment(s, i); skipped {
			current.WriteString(s[i:next])
			i = next - 1
			continue
		}

		c := s[i]
		switch c {
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			if depth > 0 {
				depth--
			}
		}

		if c == delimiterByte && depth == 0 {
			if part := strings.TrimSpace(current.String()); part != "" {
				parts = append(parts, part)
			}
			current.Reset()
			continue
		}

		current.WriteByte(c)
	}

	if tail := strings.TrimSpace(current.String()); tail != "" {
		parts = append(parts, tail)
	}

	return parts
}
