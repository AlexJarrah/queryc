package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexJarrah/queryc/internal/analyze"
	"github.com/AlexJarrah/queryc/internal/codegen"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

// Options stores generation options.
type Options struct {
	SchemaPath  string
	QueriesPath string
	OutputPath  string
	Dialect     model.Dialect
	PackageName string
}

// Run generates bindings based on the options provided.
func Run(opts Options) error {
	if opts.Dialect != model.DialectPostgres && opts.Dialect != model.DialectSQLite {
		return fmt.Errorf("unsupported dialect %q", opts.Dialect)
	}

	schema, err := parse.SchemaFile(opts.SchemaPath, opts.Dialect)
	if err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}

	queryFile, err := parse.QueryFilePath(opts.QueriesPath, schema, opts.Dialect)
	if err != nil {
		return fmt.Errorf("parse queries: %w", err)
	}

	for _, q := range queryFile.Queries {
		for _, w := range q.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s: %s\n", q.Name, w)
		}
	}

	analyzed, err := analyze.Queries(schema, queryFile.Queries, opts.Dialect)
	if err != nil {
		return fmt.Errorf("analyze queries: %w", err)
	}

	out, err := codegen.Generate(schema, queryFile.Imports, analyzed, opts.Dialect, opts.PackageName)
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	if err := writeFileAtomically(opts.OutputPath, out, 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func writeFileAtomically(path string, content []byte, mode os.FileMode) (err error) {
	if existing, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(existing, content) {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err = temp.Chmod(mode); err != nil {
		return err
	}
	if _, err = temp.Write(content); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}
