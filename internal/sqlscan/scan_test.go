package sqlscan

import "testing"

func TestSkipLiteralOrComment(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		start int
		want  int
		ok    bool
	}{
		{name: "ordinary SQL", sql: "SELECT", want: 0, ok: false},
		{name: "standard string with backslash", sql: `'\' trailing`, want: 3, ok: true},
		{name: "doubled quote", sql: `'it''s valid' trailing`, want: 13, ok: true},
		{name: "Postgres escape string", sql: `E'it\'s valid' trailing`, start: 1, want: 14, ok: true},
		{name: "quoted identifier", sql: `"a""b" trailing`, want: 6, ok: true},
		{name: "dollar quote", sql: `$tag$content$tag$ trailing`, want: 17, ok: true},
		{name: "line comment", sql: "-- comment\nSELECT", want: 10, ok: true},
		{name: "block comment", sql: "/* comment */SELECT", want: 13, ok: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := SkipLiteralOrComment(test.sql, test.start)
			if got != test.want || ok != test.ok {
				t.Fatalf("SkipLiteralOrComment(%q) = (%d, %v), want (%d, %v)", test.sql, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestExtractBalancedSkipsLiteralsAndComments(t *testing.T) {
	sql := `(first, 'ignored )', nested(second), /* ) */ third) trailing`
	body, closeIndex, err := ExtractBalanced(sql, 0)
	if err != nil {
		t.Fatalf("ExtractBalanced() error = %v", err)
	}
	if want := `first, 'ignored )', nested(second), /* ) */ third`; body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
	if closeIndex != 50 {
		t.Fatalf("close = %d, want 50", closeIndex)
	}
}

func TestFindTopLevelKeyword(t *testing.T) {
	sql := `SELECT (SELECT 'FROM ignored') AS nested FROM users ORDER BY id`
	if got := FindTopLevelKeyword(sql, "FROM"); got != 41 {
		t.Fatalf("FindTopLevelKeyword(FROM) = %d, want 41", got)
	}
	if got := FindTopLevelKeyword(sql, "ORDER BY"); got != 52 {
		t.Fatalf("FindTopLevelKeyword(ORDER BY) = %d, want 52", got)
	}
}
