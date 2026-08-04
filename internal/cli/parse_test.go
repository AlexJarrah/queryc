package cli

import (
	"testing"

	"github.com/AlexJarrah/queryc/internal/model"
)

func TestParseAcceptsFlagsAfterPositionals(t *testing.T) {
	opts, err := Parse([]string{
		"schema.sql",
		"queries.sql",
		"out.go",
		"--dialect",
		"postgres",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if opts.SchemaPath != "schema.sql" || opts.QueriesPath != "queries.sql" || opts.OutputPath != "out.go" {
		t.Fatalf("unexpected paths: %#v", opts)
	}
	if opts.Dialect != model.DialectPostgres {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestParseAcceptsInlineFlagValues(t *testing.T) {
	opts, err := Parse([]string{
		"schema.sql",
		"queries.sql",
		"out.go",
		"--dialect=postgresql",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if opts.Dialect != model.DialectPostgres {
		t.Fatalf("Dialect = %q, want %q", opts.Dialect, model.DialectPostgres)
	}
}

func TestParseDefaults(t *testing.T) {
	opts, err := Parse([]string{"schema.sql", "queries.sql", "out.go"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if opts.Dialect != model.DialectPostgres {
		t.Fatalf("unexpected defaults: %#v", opts)
	}
	if opts.PackageName != "" {
		t.Fatalf("PackageName = %q, want empty default", opts.PackageName)
	}
}

func TestParseAcceptsPackageFlag(t *testing.T) {
	opts, err := Parse([]string{
		"schema.sql",
		"queries.sql",
		"out.go",
		"--package",
		"db",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if opts.PackageName != "db" {
		t.Fatalf("PackageName = %q, want db", opts.PackageName)
	}
}

func TestParseAcceptsInlinePackageFlag(t *testing.T) {
	opts, err := Parse([]string{
		"schema.sql",
		"queries.sql",
		"out.go",
		"--package=store",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if opts.PackageName != "store" {
		t.Fatalf("PackageName = %q, want store", opts.PackageName)
	}
}

func TestParseRejectsRemovedSchemaFlags(t *testing.T) {
	for _, flag := range []string{"--schemas=models", "--schemas-import=example.com/app/models"} {
		if _, err := Parse([]string{"schema.sql", "queries.sql", "out.go", flag}); err == nil {
			t.Fatalf("expected removed flag %q to be rejected", flag)
		}
	}
}

func TestParseReportsUnknownFlagAfterPositionals(t *testing.T) {
	_, err := Parse([]string{
		"schema.sql",
		"queries.sql",
		"out.go",
		"--unknown",
	})
	if err == nil || err.Error() != "flag provided but not defined: -unknown" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseReturnsHelpSentinel(t *testing.T) {
	_, err := Parse([]string{"--help"})
	if !IsHelp(err) {
		t.Fatalf("Parse(--help) error = %v, want help", err)
	}
}

func TestParseDialectAliases(t *testing.T) {
	cases := map[string]model.Dialect{
		"":           model.DialectPostgres,
		"postgres":   model.DialectPostgres,
		"postgresql": model.DialectPostgres,
		"PG":         model.DialectPostgres,
		"sqlite":     model.DialectSQLite,
		" SQLite ":   model.DialectSQLite,
	}
	for input, want := range cases {
		got, err := parseDialect(input)
		if err != nil {
			t.Fatalf("parseDialect(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("parseDialect(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseRejectsUnsupportedDialect(t *testing.T) {
	if _, err := Parse([]string{
		"schema.sql",
		"queries.sql",
		"out.go",
		"--dialect=mysql",
	}); err == nil {
		t.Fatal("expected unsupported dialect to be rejected")
	}
}
