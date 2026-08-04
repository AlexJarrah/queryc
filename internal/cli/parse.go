package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/AlexJarrah/queryc/internal/app"
	"github.com/AlexJarrah/queryc/internal/model"
)

const Usage = "usage: queryc <schema> <queries> <output> [--dialect postgres|sqlite] [--package name]"

// IsHelp returns whether parsing stopped for a help request.
func IsHelp(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}

// Parse validates and returns command arguments as structured data.
func Parse(args []string) (app.Options, error) {
	var dialect, pkg string

	fs := flag.NewFlagSet("queryc", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&dialect, "dialect", "postgres", "Database dialect (postgres, sqlite)")
	fs.StringVar(&pkg, "package", "", "Generated Go package name (defaults to dialect name)")

	if err := fs.Parse(normalizeFlagOrder(args)); err != nil {
		return app.Options{}, err
	}

	if fs.NArg() != 3 {
		return app.Options{}, errors.New("expected schema, queries, and output paths")
	}

	parsedDialect, err := parseDialect(dialect)
	if err != nil {
		return app.Options{}, err
	}

	return app.Options{
		SchemaPath:  fs.Arg(0),
		QueriesPath: fs.Arg(1),
		OutputPath:  fs.Arg(2),
		Dialect:     parsedDialect,
		PackageName: strings.TrimSpace(pkg),
	}, nil
}

func parseDialect(value string) (model.Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "postgres", "postgresql", "pg":
		return model.DialectPostgres, nil
	case "sqlite":
		return model.DialectSQLite, nil
	default:
		return 0, fmt.Errorf("unsupported dialect %q", value)
	}
}

func normalizeFlagOrder(args []string) []string {
	var flags []string
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}

		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		flags = append(flags, arg)
		if strings.Contains(arg, "=") || !isKnownFlag(arg) {
			continue
		}

		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags = append(flags, args[i+1])
			i++
		}
	}

	return append(flags, positionals...)
}

func isKnownFlag(arg string) bool {
	name := strings.TrimPrefix(arg, "--")
	if name == arg {
		name = strings.TrimPrefix(arg, "-")
	}

	name, _, _ = strings.Cut(name, "=")
	switch name {
	case "dialect", "package":
		return true
	default:
		return false
	}
}
