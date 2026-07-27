package sqlscan

import (
	"fmt"
	"strings"
)

// SkipLiteralOrComment skips past a quoted string or identifier,
// dollar quoted string, or SQL comment beginning at start. It returns false
// when start points to regular SQL.
func SkipLiteralOrComment(sql string, start int) (next int, ok bool) {
	if start >= len(sql) {
		return start, false
	}

	switch sql[start] {
	case '\'', '"':
		quote := sql[start]
		backslashEscapes := quote == '\'' && hasPostgresEscapePrefix(sql, start)
		for i := start + 1; i < len(sql); i++ {
			if backslashEscapes && sql[i] == '\\' {
				i++
				continue
			}
			if sql[i] != quote {
				continue
			}
			if i+1 < len(sql) && sql[i+1] == quote {
				i++
				continue
			}
			return i + 1, true
		}
		return len(sql), true
	case '$':
		delimiterEnd := start + 1
		for delimiterEnd < len(sql) && IsIdentifierByte(sql[delimiterEnd]) {
			delimiterEnd++
		}
		if delimiterEnd < len(sql) && sql[delimiterEnd] == '$' {
			delimiter := sql[start : delimiterEnd+1]
			if closeOffset := strings.Index(sql[delimiterEnd+1:], delimiter); closeOffset >= 0 {
				return delimiterEnd + 1 + closeOffset + len(delimiter), true
			}
			return len(sql), true
		}
	case '-':
		if start+1 < len(sql) && sql[start+1] == '-' {
			next := start + 2
			for next < len(sql) && sql[next] != '\n' {
				next++
			}
			return next, true
		}
	case '/':
		if start+1 < len(sql) && sql[start+1] == '*' {
			next := start + 2
			for next+1 < len(sql) {
				if sql[next] == '*' && sql[next+1] == '/' {
					return next + 2, true
				}
				next++
			}
			return len(sql), true
		}
	}

	return start, false
}

func hasPostgresEscapePrefix(sql string, quoteIndex int) bool {
	if quoteIndex == 0 || sql[quoteIndex-1] != 'E' && sql[quoteIndex-1] != 'e' {
		return false
	}
	return quoteIndex == 1 || !IsIdentifierByte(sql[quoteIndex-2])
}

// IsIdentifierByte returns whether b can occur in an unquoted SQL identifier.
func IsIdentifierByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// ExtractBalanced returns the content and closing index for the parenthesized
// expression beginning at open. String literals and comments are ignored.
func ExtractBalanced(sql string, open int) (string, int, error) {
	if open < 0 || open >= len(sql) || sql[open] != '(' {
		return "", open, fmt.Errorf("expected opening parenthesis")
	}

	depth := 1
	for i := open + 1; i < len(sql); i++ {
		if next, ok := SkipLiteralOrComment(sql, i); ok {
			i = next - 1
			continue
		}
		switch sql[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return sql[open+1 : i], i, nil
			}
		}
	}
	return "", open, fmt.Errorf("unbalanced parenthesized expression")
}

// FindTopLevelKeyword finds keyword (case-insensitive) outside parenthesized expressions,
// literals, and comments.
func FindTopLevelKeyword(sql, keyword string) int {
	if keyword == "" {
		return -1
	}

	upperSQL := strings.ToUpper(sql)
	upperKeyword := strings.ToUpper(keyword)
	depth := 0
	for i := 0; i <= len(sql)-len(keyword); i++ {
		if next, ok := SkipLiteralOrComment(sql, i); ok {
			i = next - 1
			continue
		}
		switch sql[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && upperSQL[i:i+len(keyword)] == upperKeyword {
			return i
		}
	}
	return -1
}
