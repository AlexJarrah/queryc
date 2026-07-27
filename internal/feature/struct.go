package feature

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/utils"
)

var trailingAsAliasRe = regexp.MustCompile(`(?is)^(.+?)\s+AS\s+[a-zA-Z0-9_]+$`)

// structHandler rewrites @struct calls into dialect-specific JSON object builder.
func structHandler(content string, dialect model.Dialect) (string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") || !strings.HasSuffix(content, "}") {
		return "", fmt.Errorf("@struct body must be wrapped in braces")
	}
	content = strings.TrimSpace(content[1 : len(content)-1])

	parts := utils.SplitByDelimiter(content, ',')
	out := make([]string, 0, len(parts)*2)

	for _, part := range parts {
		key, value, ok := splitStructField(part)
		if !ok {
			return "", fmt.Errorf("invalid @struct field %q", part)
		}
		value = strings.TrimSpace(value)
		if match := trailingAsAliasRe.FindStringSubmatch(value); match != nil {
			value = strings.TrimSpace(match[1])
		}
		out = append(out, fmt.Sprintf("'%s'", strings.TrimSpace(key)))
		out = append(out, value)
	}

	if dialect == model.DialectSQLite {
		return jsonMarker + "json_object(" + strings.Join(out, ", ") + ")", nil
	}
	return jsonMarker + "jsonb_build_object(" + strings.Join(out, ", ") + ")", nil
}

// splitStructField splits a field entry on the first colon, returning the
// JSON key and the SQL expression.
func splitStructField(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	before, after, ok := strings.Cut(s, ":")
	if !ok {
		return "", "", false
	}
	key := strings.TrimSpace(before)
	value := strings.TrimSpace(after)
	return key, value, true
}
