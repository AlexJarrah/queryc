package feature

import (
	"strings"
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestSliceSubqueryConversion(t *testing.T) {
	registry := NewRegistry()
	sql, err := registry.Convert(`SELECT @slice((SELECT ul.login_at ORDER BY ul.login_at DESC FROM user_logins ul WHERE ul.user_id = users.user_id)) AS recent_logins`, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if !strings.Contains(sql, "json_agg(queryc_value)") {
		t.Fatalf("expected json_agg output, got %q", sql)
	}
	if !strings.Contains(sql, "SELECT ul.login_at AS queryc_value FROM user_logins ul") {
		t.Fatalf("expected rewritten subquery, got %q", sql)
	}
}

func TestStructNestedInSliceConversion(t *testing.T) {
	registry := NewRegistry()
	sql, err := registry.Convert(`SELECT @slice(distinct @struct({tag: ut.tag, source: ut.source})) AS tags`, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if !strings.Contains(sql, "jsonb_build_object('tag', ut.tag, 'source', ut.source)") {
		t.Fatalf("expected jsonb_build_object output, got %q", sql)
	}
	if !strings.Contains(sql, "json_agg(DISTINCT") {
		t.Fatalf("expected distinct json_agg output, got %q", sql)
	}
}

func TestStructFieldAllowsTrailingAlias(t *testing.T) {
	registry := NewRegistry()
	sql, err := registry.Convert(`SELECT @struct({enabled: uc.email_notifications:bool AS email_notifications, timezone: uc.timezone:string}) AS configuration`, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if !strings.Contains(sql, "jsonb_build_object('enabled', uc.email_notifications:bool, 'timezone', uc.timezone:string)") {
		t.Fatalf("expected trailing alias to be ignored inside struct, got %q", sql)
	}
}

func TestConvertIgnoresFeaturesInsideSQLLiterals(t *testing.T) {
	registry := NewRegistry()
	input := `SELECT '@slice(not_a_feature)', "@struct({not: real})", $$@slice(nope)$$, $tag$@struct({nope: true})$tag$`

	got, err := registry.Convert(input, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if got != input {
		t.Fatalf("Convert() changed literals:\n got: %s\nwant: %s", got, input)
	}
}

func TestConvertBalancesParenthesesInsideFeatureLiterals(t *testing.T) {
	registry := NewRegistry()
	got, err := registry.Convert(`SELECT @struct({label: 'value ) text', count: COUNT(*)})`, model.DialectPostgres)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if !strings.Contains(got, `jsonb_build_object('label', 'value ) text', 'count', COUNT(*))`) {
		t.Fatalf("unexpected conversion: %s", got)
	}
}
