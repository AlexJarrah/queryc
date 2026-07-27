package parse

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// ToPascal converts snake_case identifiers to PascalCase.
func ToPascal(s string) string {
	if s == "" {
		return ""
	}

	special := map[string]string{
		"api":   "API",
		"http":  "HTTP",
		"https": "HTTPS",
		"uuid":  "UUID",
		"uuids": "UUIDs",
		"id":    "ID",
		"ids":   "IDs",
		"sql":   "SQL",
		"url":   "URL",
		"urls":  "URLs",
	}

	parts := strings.Split(s, "_")
	var out strings.Builder
	for _, part := range parts {
		lower := strings.ToLower(part)
		if v, ok := special[lower]; ok {
			out.WriteString(v)
			continue
		}
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		out.WriteRune(unicode.ToUpper(runes[0]))
		for _, r := range runes[1:] {
			out.WriteRune(unicode.ToLower(r))
		}
	}
	return out.String()
}

// ToCamel converts snake_case identifiers to lower camel case.
func ToCamel(s string) string {
	if s == "" {
		return ""
	}
	if !strings.ContainsRune(s, '_') {
		runes := []rune(s)
		runes[0] = unicode.ToLower(runes[0])
		return string(runes)
	}

	parts := strings.Split(s, "_")
	var out strings.Builder
	out.WriteString(strings.ToLower(parts[0]))
	for _, part := range parts[1:] {
		out.WriteString(ToPascal(part))
	}
	return out.String()
}

// ToSingular applies common English plural to singular heuristics for type names.
func ToSingular(s string) string {
	switch {
	case strings.HasSuffix(s, "ies"):
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ses"):
		return s[:len(s)-2]
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return s[:len(s)-1]
	default:
		return s
	}
}

func generateShortAlias(tableName string) string {
	parts := strings.Split(tableName, "_")
	if len(parts) == 1 {
		if len(parts[0]) <= 3 {
			return strings.ToLower(parts[0])
		}
		return strings.ToLower(parts[0][:3])
	}

	var out strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		out.WriteByte(byte(unicode.ToLower(rune(part[0]))))
	}

	return out.String()
}

// ReadFileOrDir reads a file or directory recursively (only .sql files).
func ReadFileOrDir(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return os.ReadFile(path)
	}

	files, err := sqlFiles(path)
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	var combined strings.Builder
	for i, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read file %s: %w", file, err)
		}
		if i > 0 {
			combined.WriteString("\n")
		}
		combined.Write(content)
	}

	return []byte(combined.String()), nil
}

func sqlFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !entry.IsDir() && isSQLFile(path) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}
