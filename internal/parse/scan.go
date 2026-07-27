package parse

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AlexJarrah/queryc/internal/sqlscan"
)

// codeMask marks bytes in executable SQL (outside string literals,
// dollar-quoted strings, and comments).
func codeMask(sql string) []bool {
	mask := make([]bool, len(sql))
	for i := 0; i < len(sql); {
		next, skipped := sqlscan.SkipLiteralOrComment(sql, i)
		if skipped {
			i = next
			continue
		}
		mask[i] = true
		i++
	}
	return mask
}

// replaceOutsideLiterals applies repl to every regexp match that is not inside
// a string literal or comment.
func replaceOutsideLiterals(sql string, re *regexp.Regexp, repl func(full string, groups []string) string) string {
	if sql == "" {
		return sql
	}

	mask := codeMask(sql)
	var out strings.Builder
	out.Grow(len(sql))
	i := 0
	for i < len(sql) {
		loc := re.FindStringSubmatchIndex(sql[i:])
		if loc == nil {
			out.WriteString(sql[i:])
			break
		}
		start := i + loc[0]
		end := i + loc[1]
		if !mask[start] {
			out.WriteString(sql[i : start+1])
			i = start + 1
			continue
		}
		out.WriteString(sql[i:start])
		full := sql[start:end]
		groups := make([]string, (len(loc)/2)-1)
		for g := 1; g < len(loc)/2; g++ {
			if loc[2*g] >= 0 {
				groups[g-1] = sql[i+loc[2*g] : i+loc[2*g+1]]
			}
		}
		out.WriteString(repl(full, groups))
		i = end
	}
	return out.String()
}

// unescapeQuotedString unquotes a double quoted string body that may contain
// backslash escapes. The input should not include the surrounding quotes.
func unescapeQuotedString(s string) (string, error) {
	var out strings.Builder
	out.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			out.WriteByte(s[i])
			continue
		}
		i++
		if i >= len(s) {
			return "", fmt.Errorf("unterminated escape sequence")
		}
		switch s[i] {
		case '"', '\\', '/':
			out.WriteByte(s[i])
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		default:
			out.WriteByte(s[i])
		}
	}
	return out.String(), nil
}

// isSQLFile returns whether path looks is a SQL file.
func isSQLFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".sql")
}
