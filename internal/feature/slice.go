package feature

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/sqlscan"
)

// trailingAliasRe matches a trailing " AS identifier" at the end of a SELECT expression.
var trailingAliasRe = regexp.MustCompile(`(?i)\s+AS\s+[a-zA-Z_][a-zA-Z0-9_]*$`)

const jsonMarker = "/*queryc_json*/"

func sliceHandler(content string, dialect model.Dialect) (string, error) {
	expr, distinct := extractDistinct(content)
	subquery, ok, err := extractSubquery(expr)
	if err != nil {
		return "", err
	}

	if ok {
		rewritten, err := rewriteSubquery(subquery)
		if err != nil {
			return "", err
		}
		if dialect == model.DialectSQLite {
			return fmt.Sprintf("%sCOALESCE((SELECT json_group_array(queryc_value) FROM (%s) AS queryc_slice WHERE queryc_value IS NOT NULL), '[]')", jsonMarker, rewritten), nil
		}
		return fmt.Sprintf("%sCOALESCE((SELECT json_agg(queryc_value) FROM (%s) AS queryc_slice WHERE queryc_value IS NOT NULL), '[]'::json)", jsonMarker, rewritten), nil
	}

	distinctPrefix := ""
	if distinct {
		distinctPrefix = "DISTINCT "
	}
	if dialect == model.DialectSQLite {
		return fmt.Sprintf("%sCOALESCE(json_group_array(%s%s), '[]')", jsonMarker, distinctPrefix, expr), nil
	}
	if distinct {
		return fmt.Sprintf("%sCOALESCE(jsonb_agg(%s%s) FILTER (WHERE %s IS NOT NULL), '[]'::jsonb)", jsonMarker, distinctPrefix, expr, expr), nil
	}
	return fmt.Sprintf("%sCOALESCE(json_agg(%s%s) FILTER (WHERE %s IS NOT NULL), '[]'::json)", jsonMarker, distinctPrefix, expr, expr), nil
}

func extractDistinct(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(strings.ToUpper(trimmed), "DISTINCT ") {
		return strings.TrimSpace(trimmed[len("DISTINCT "):]), true
	}
	return trimmed, false
}

func extractSubquery(content string) (string, bool, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
		inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		if strings.HasPrefix(strings.ToUpper(inner), "SELECT") {
			return inner, true, nil
		}
	}
	if strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
		return trimmed, true, nil
	}
	return "", false, nil
}

func rewriteSubquery(subquery string) (string, error) {
	selectExpr, fromClause, orderClause, err := splitSubquery(subquery)
	if err != nil {
		return "", err
	}
	query := fmt.Sprintf("SELECT %s AS queryc_value FROM %s", selectExpr, fromClause)
	if orderClause != "" {
		query += " ORDER BY " + orderClause
	}
	return query, nil
}

func splitSubquery(subquery string) (string, string, string, error) {
	stripped := strings.TrimSpace(strings.TrimSuffix(subquery, ";"))
	selectPos := sqlscan.FindTopLevelKeyword(stripped, "SELECT")
	if selectPos == -1 {
		return "", "", "", fmt.Errorf("SLICE subquery requires SELECT clause")
	}
	fromPos := sqlscan.FindTopLevelKeyword(stripped, "FROM")
	if fromPos == -1 {
		return "", "", "", fmt.Errorf("SLICE subquery requires FROM clause")
	}
	orderPos := sqlscan.FindTopLevelKeyword(stripped, "ORDER BY")
	selectExpr := strings.TrimSpace(stripped[selectPos+len("SELECT") : fromPos])
	fromClause := strings.TrimSpace(stripped[fromPos+len("FROM"):])
	orderClause := ""
	if orderPos != -1 {
		if orderPos < fromPos {
			selectExpr = strings.TrimSpace(stripped[selectPos+len("SELECT") : orderPos])
			orderClause = strings.TrimSpace(stripped[orderPos+len("ORDER BY") : fromPos])
			fromClause = strings.TrimSpace(stripped[fromPos+len("FROM"):])
		} else {
			selectExpr = strings.TrimSpace(stripped[selectPos+len("SELECT") : fromPos])
			fromClause = strings.TrimSpace(stripped[fromPos+len("FROM") : orderPos])
			orderClause = strings.TrimSpace(stripped[orderPos+len("ORDER BY"):])
		}
	}
	if loc := trailingAliasRe.FindStringIndex(selectExpr); loc != nil {
		selectExpr = strings.TrimSpace(selectExpr[:loc[0]])
	}
	return selectExpr, fromClause, orderClause, nil
}
