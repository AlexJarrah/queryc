package codegen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/AlexJarrah/queryc/internal/analyze"
	"github.com/AlexJarrah/queryc/internal/model"
	"github.com/AlexJarrah/queryc/internal/parse"
)

func TestGenerateUserConfigurationGoldenSnippet(t *testing.T) {
	root := filepath.Join("..", "..")
	schemaBytes, err := os.ReadFile(filepath.Join(root, "test", "schema.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	queryBytes, err := os.ReadFile(filepath.Join(root, "test", "queries.sql"))
	if err != nil {
		t.Fatalf("read queries: %v", err)
	}

	schema, err := parse.Schema(string(schemaBytes), model.DialectPostgres)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	queryFile, err := parse.QueryFile(string(queryBytes), schema, model.DialectPostgres)
	if err != nil {
		t.Fatalf("parse queries: %v", err)
	}
	analyzed, err := analyze.Queries(schema, queryFile.Queries, model.DialectPostgres)
	if err != nil {
		t.Fatalf("analyze queries: %v", err)
	}

	out, err := Generate(schema, queryFile.Imports, analyzed, model.DialectPostgres, "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	text := string(out)
	functionSnippet := regexp.MustCompile(`(?s)func GetUserWithConfiguration\(.*?\n\}`).FindString(text)
	resultSnippet := regexp.MustCompile(`(?s)type GetUserWithConfigurationResult struct \{.*?\n\}`).FindString(text)
	if functionSnippet == "" || resultSnippet == "" {
		t.Fatalf("failed to extract golden snippet from output:\n%s", text)
	}
	matches := strings.TrimSpace(functionSnippet) + "\n\n" + strings.TrimSpace(resultSnippet)

	goldenPath := filepath.Join(root, "test", "user_configuration_snippet.golden.txt")
	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if strings.TrimSpace(matches) != strings.TrimSpace(string(goldenBytes)) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", matches, string(goldenBytes))
	}
}
