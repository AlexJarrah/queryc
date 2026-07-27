package parse

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNaming(t *testing.T) {
	pascal := map[string]string{
		"user_id":    "UserID",
		"api_tokens": "APITokens",
		"uuid":       "UUID",
	}
	for input, want := range pascal {
		if got := ToPascal(input); got != want {
			t.Fatalf("ToPascal(%q) = %q, want %q", input, got, want)
		}
	}
	if got := ToCamel("user_id"); got != "userID" {
		t.Fatalf("ToCamel(user_id) = %q, want userID", got)
	}

	singular := map[string]string{
		"Users":      "User",
		"Categories": "Category",
		"Statuses":   "Status",
	}
	for input, want := range singular {
		if got := ToSingular(input); got != want {
			t.Fatalf("ToSingular(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReadFileOrDir_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.sql")
	content := "CREATE TABLE users (id INTEGER PRIMARY KEY);"

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := ReadFileOrDir(testFile)
	if err != nil {
		t.Fatalf("ReadFileOrDir() error = %v", err)
	}

	if string(result) != content {
		t.Errorf("ReadFileOrDir() = %q, want %q", string(result), content)
	}
}

func TestReadFileOrDir_WithDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in non-alphabetical order to test sorting
	files := map[string]string{
		"002_posts.sql":    "CREATE TABLE posts (id INTEGER PRIMARY KEY);",
		"001_users.sql":    "CREATE TABLE users (id INTEGER PRIMARY KEY);",
		"003_comments.sql": "CREATE TABLE comments (id INTEGER PRIMARY KEY);",
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	result, err := ReadFileOrDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadFileOrDir() error = %v", err)
	}

	expected := "CREATE TABLE users (id INTEGER PRIMARY KEY);\nCREATE TABLE posts (id INTEGER PRIMARY KEY);\nCREATE TABLE comments (id INTEGER PRIMARY KEY);"

	if string(result) != expected {
		t.Errorf("ReadFileOrDir() = %q, want %q", string(result), expected)
	}
}

func TestReadFileOrDir_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := ReadFileOrDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadFileOrDir() error = %v", err)
	}

	if string(result) != "" {
		t.Errorf("ReadFileOrDir() = %q, want empty string", string(result))
	}
}

func TestReadFileOrDir_NonExistentPath(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does_not_exist.sql")

	_, err := ReadFileOrDir(nonExistent)
	if err == nil {
		t.Error("ReadFileOrDir() expected error for non-existent path")
	}
}

func TestReadFileOrDir_IncludesSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create file in subdirectory (should be included)
	subFile := filepath.Join(subDir, "002_sub.sql")
	if err := os.WriteFile(subFile, []byte("CREATE TABLE sub (id INTEGER);"), 0o644); err != nil {
		t.Fatalf("failed to create test file in subdir: %v", err)
	}

	// Create file in main directory
	mainFile := filepath.Join(tmpDir, "001_main.sql")
	if err := os.WriteFile(mainFile, []byte("CREATE TABLE main (id INTEGER);"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := ReadFileOrDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadFileOrDir() error = %v", err)
	}

	// Files should be ordered by full path: 001_main.sql, subdir/002_sub.sql
	expected := "CREATE TABLE main (id INTEGER);\nCREATE TABLE sub (id INTEGER);"
	if string(result) != expected {
		t.Errorf("ReadFileOrDir() = %q, want %q", string(result), expected)
	}
}

func TestReadFileOrDir_MultilineFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with multiple lines
	file1 := filepath.Join(tmpDir, "001_first.sql")
	content1 := "-- Table: users\nCREATE TABLE users (\n    id INTEGER PRIMARY KEY,\n    name TEXT\n);"
	if err := os.WriteFile(file1, []byte(content1), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	file2 := filepath.Join(tmpDir, "002_second.sql")
	content2 := "-- Table: posts\nCREATE TABLE posts (\n    id INTEGER PRIMARY KEY\n);"
	if err := os.WriteFile(file2, []byte(content2), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := ReadFileOrDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadFileOrDir() error = %v", err)
	}

	expected := content1 + "\n" + content2
	if string(result) != expected {
		t.Errorf("ReadFileOrDir() = %q, want %q", string(result), expected)
	}
}

func TestReadFileOrDir_DeeplyNestedSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure: a/b/c/
	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatalf("failed to create nested directories: %v", err)
	}

	// Create files at various depths
	files := map[string]string{
		"root.sql":       "CREATE TABLE root (id INTEGER);",
		"a/a.sql":        "CREATE TABLE a (id INTEGER);",
		"a/b/b.sql":      "CREATE TABLE b (id INTEGER);",
		"a/b/c/c.sql":    "CREATE TABLE c (id INTEGER);",
		"a/b/c/deep.sql": "CREATE TABLE deep (id INTEGER);",
	}

	for relPath, content := range files {
		fullPath := filepath.Join(tmpDir, relPath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create test file %s: %v", relPath, err)
		}
	}

	result, err := ReadFileOrDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadFileOrDir() error = %v", err)
	}

	// Files should be ordered by full path alphabetically
	expected := "CREATE TABLE a (id INTEGER);\nCREATE TABLE b (id INTEGER);\nCREATE TABLE c (id INTEGER);\nCREATE TABLE deep (id INTEGER);\nCREATE TABLE root (id INTEGER);"
	if string(result) != expected {
		t.Errorf("ReadFileOrDir() = %q, want %q", string(result), expected)
	}
}
