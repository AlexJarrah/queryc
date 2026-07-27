package codegen

import "testing"

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"break", "break_val"},
		{"case", "case_val"},
		{"chan", "chan_val"},
		{"const", "const_val"},
		{"continue", "continue_val"},
		{"default", "default_val"},
		{"defer", "defer_val"},
		{"else", "else_val"},
		{"fallthrough", "fallthrough_val"},
		{"for", "for_val"},
		{"func", "func_val"},
		{"go", "go_val"},
		{"goto", "goto_val"},
		{"if", "if_val"},
		{"import", "import_val"},
		{"interface", "interface_val"},
		{"map", "map_val"},
		{"package", "package_val"},
		{"range", "range_val"},
		{"return", "return_val"},
		{"select", "select_val"},
		{"struct", "struct_val"},
		{"switch", "switch_val"},
		{"type", "type_val"},
		{"var", "var_val"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeName(tt.input); got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
