package utils

import (
	"reflect"
	"testing"
)

func TestSplitByDelimiter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter rune
		want      []string
	}{
		{
			name:      "simple comma",
			input:     "a, b, c",
			delimiter: ',',
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "respects nested parens",
			input:     "a, foo(b, c), d",
			delimiter: ',',
			want:      []string{"a", "foo(b, c)", "d"},
		},
		{
			name:      "respects nested braces",
			input:     "a, {b, c}, d",
			delimiter: ',',
			want:      []string{"a", "{b, c}", "d"},
		},
		{
			name:      "respects quoted commas",
			input:     `a, 'last, first', "quoted,column", d`,
			delimiter: ',',
			want:      []string{"a", "'last, first'", `"quoted,column"`, "d"},
		},
		{
			name:      "respects escaped quotes",
			input:     `a, 'it''s, valid', d`,
			delimiter: ',',
			want:      []string{"a", "'it''s, valid'", "d"},
		},
		{
			name:      "backslash does not escape SQL quote",
			input:     `name ILIKE $name ESCAPE '\', next`,
			delimiter: ',',
			want:      []string{`name ILIKE $name ESCAPE '\'`, "next"},
		},
		{
			name:      "postgres escape strings support backslash escapes",
			input:     `E'it\'s, valid', next`,
			delimiter: ',',
			want:      []string{`E'it\'s, valid'`, "next"},
		},
		{
			name:      "respects dollar quoted commas",
			input:     `a, $$one,two$$, $tag$three,four$tag$, d`,
			delimiter: ',',
			want:      []string{"a", "$$one,two$$", "$tag$three,four$tag$", "d"},
		},
		{
			name:      "respects brackets",
			input:     "a, ARRAY[1, 2], d",
			delimiter: ',',
			want:      []string{"a", "ARRAY[1, 2]", "d"},
		},
		{
			name:      "mixed nesting",
			input:     "a, foo({b, c}), d",
			delimiter: ',',
			want:      []string{"a", "foo({b, c})", "d"},
		},
		{
			name:      "empty parts omitted",
			input:     "a,,b",
			delimiter: ',',
			want:      []string{"a", "b"},
		},
		{
			name:      "trims spaces",
			input:     "  a  ,   b   ",
			delimiter: ',',
			want:      []string{"a", "b"},
		},
		{
			name:      "single element",
			input:     "only_one",
			delimiter: ',',
			want:      []string{"only_one"},
		},
		{
			name:      "empty string",
			input:     "",
			delimiter: ',',
			want:      nil,
		},
		{
			name:      "semicolon delimiter",
			input:     "a; b; c",
			delimiter: ';',
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "unbalanced closing paren ignored",
			input:     ")a, b(",
			delimiter: ',',
			want:      []string{")a", "b("},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitByDelimiter(tt.input, tt.delimiter)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitByDelimiter(%q, %q) = %v, want %v", tt.input, string(tt.delimiter), got, tt.want)
			}
		})
	}
}
