package parse

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/AlexJarrah/queryc/internal/model"
)

// QueryFilePath reads a queryc file or directory from disk and parses it.
func QueryFilePath(path string, schema model.Schema, d model.Dialect) (model.QueryFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return model.QueryFile{}, err
	}
	if info.IsDir() {
		return parseQueryDirectory(path, schema, d)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return model.QueryFile{}, err
	}
	text := string(content)
	qf, err := QueryFile(text, schema, d)
	if err != nil {
		return model.QueryFile{}, formatFileError(err, text, path)
	}
	return qf, nil
}

// parseQueryDirectory walks and parses dir recursively. Files are parsed
// concurrently and merged alphabetically for a deterministic output.
func parseQueryDirectory(dir string, schema model.Schema, d model.Dialect) (model.QueryFile, error) {
	files, err := sqlFiles(dir)
	if err != nil {
		return model.QueryFile{}, fmt.Errorf("walk directory %s: %w", dir, err)
	}

	type fileResult struct {
		path    string
		imports []model.Import
		queries []model.Query
		err     error
	}

	ch := make(chan fileResult, len(files))
	jobs := make(chan string, len(files))
	workerCount := min(len(files), max(1, runtime.GOMAXPROCS(0)))
	for range workerCount {
		go func() {
			for filePath := range jobs {
				b, err := os.ReadFile(filePath)
				if err != nil {
					ch <- fileResult{path: filePath, err: fmt.Errorf("read %s: %w", filePath, err)}
					continue
				}
				text := string(b)
				qf, err := QueryFile(text, schema, d)
				if err != nil {
					ch <- fileResult{path: filePath, err: formatFileError(err, text, filePath)}
					continue
				}
				ch <- fileResult{path: filePath, imports: qf.Imports, queries: qf.Queries}
			}
		}()
	}
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	results := make([]fileResult, 0, len(files))
	for range files {
		results = append(results, <-ch)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].path < results[j].path })
	for _, result := range results {
		if result.err != nil {
			return model.QueryFile{}, result.err
		}
	}

	var allImports []model.Import
	var allQueries []model.Query
	seenQueryNames := map[string]string{}

	for _, r := range results {
		for _, imp := range r.imports {
			var err error
			allImports, err = appendImport(allImports, imp)
			if err != nil {
				return model.QueryFile{}, fmt.Errorf("%s: %w", r.path, err)
			}
		}
		for _, q := range r.queries {
			if prev, ok := seenQueryNames[q.Name]; ok {
				return model.QueryFile{}, fmt.Errorf("duplicate query name %q in %s (also defined in %s)", q.Name, r.path, prev)
			}
			seenQueryNames[q.Name] = r.path
			allQueries = append(allQueries, q)
		}
	}

	if err := validateImports(allImports); err != nil {
		return model.QueryFile{}, err
	}

	return model.QueryFile{Imports: allImports, Queries: allQueries}, nil
}

// lineCol returns the 1-based line and column corresponding to the
// byte-offset in content.
func lineCol(content string, offset int) (int, int) {
	line, col := 1, 1
	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// formatFileError annotates err with file path, line and column information.
// If the error message contains the phrase "at position N", that part is
// replaced with the corresponding line:col pair.
func formatFileError(err error, content string, filePath string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()

	// Replace byte-offset with human-readable line:col.
	if loc := errorPosRe.FindStringSubmatch(msg); loc != nil {
		offset, _ := strconv.Atoi(loc[1])
		line, col := lineCol(content, offset)
		msg = strings.Replace(
			msg,
			fmt.Sprintf("at position %d", offset),
			fmt.Sprintf("at line %d, col %d", line, col),
			1,
		)
	}

	return fmt.Errorf("%s: %s", filePath, msg)
}
