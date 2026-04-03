package review

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/DreamCats/coco-ext/internal/git"
	"github.com/DreamCats/coco-ext/internal/lint"
)

var exportedFuncPattern = regexp.MustCompile(`^func\s+(\([^)]+\)\s*)?([A-Z][A-Za-z0-9_]*)\s*\(`)
var sensitiveLinePattern = regexp.MustCompile(`(?i)(password|secret|token|apikey|api_key)\s*[:=]`)

func CollectFacts(repoRoot string, diffInfo *git.DiffInfo) Facts {
	files := parseFileChanges(repoRoot, diffInfo.Diff)
	facts := Facts{
		DiffInfo:          diffInfo,
		Files:             files,
		ConfigFiles:       make([]string, 0),
		IDLFiles:          make([]string, 0),
		DDLFindings:       make([]string, 0),
		SensitiveFindings: make([]string, 0),
		MissingTests:      make([]string, 0),
		LargeFiles:        make([]string, 0),
	}

	for _, file := range files {
		facts.TotalChangedLines += file.Additions + file.Deletions
		if file.IsGoLike {
			facts.GoFileCount++
		}
		if file.IsConfigLike {
			facts.ConfigFiles = appendIfMissing(facts.ConfigFiles, file.Path)
		}
		if file.IsIDLLike {
			facts.IDLFiles = appendIfMissing(facts.IDLFiles, file.Path)
		}
		if file.IsGoLike && !file.IsTestLike && !file.HasTestSibling {
			facts.MissingTests = append(facts.MissingTests, file.Path)
		}
		if file.Additions+file.Deletions >= 200 {
			facts.LargeFiles = append(facts.LargeFiles, file.Path)
		}
		for _, line := range file.AddedTodoLines {
			upper := strings.ToUpper(line)
			switch {
			case strings.Contains(upper, "FIXME"):
				facts.FIXMECount++
			case strings.Contains(upper, "HACK"):
				facts.HACKCount++
			default:
				facts.TODOCount++
			}
		}
		for _, item := range file.AddedSensitive {
			facts.SensitiveFindings = append(facts.SensitiveFindings, fmt.Sprintf("%s: %s", file.Path, item))
		}
		for _, item := range file.AddedDDL {
			facts.DDLFindings = append(facts.DDLFindings, fmt.Sprintf("%s: %s", file.Path, item))
		}
		for _, sig := range file.PublicSigAdded {
			facts.PublicSignatureChange = appendIfMissing(
				facts.PublicSignatureChange,
				fmt.Sprintf("%s: %s", file.Path, sig),
			)
		}
		for _, sig := range file.PublicSigRemoved {
			facts.PublicSignatureChange = appendIfMissing(
				facts.PublicSignatureChange,
				fmt.Sprintf("%s: %s", file.Path, sig),
			)
		}
	}

	// 加载最近一次 lint 结果作为 review 输入
	if lintResult := lint.LoadLatestResult(repoRoot); lintResult != nil && lintResult.Total > 0 {
		facts.LintIssues = lintResult.Issues
	}

	return facts
}

func parseFileChanges(repoRoot, diff string) []FileChange {
	lines := strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n")
	files := make([]FileChange, 0)
	indexByPath := make(map[string]int)
	currentPath := ""

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			currentPath = diffPath(line)
			if currentPath == "" {
				continue
			}
			if _, ok := indexByPath[currentPath]; !ok {
				indexByPath[currentPath] = len(files)
				files = append(files, newFileChange(repoRoot, currentPath))
			}
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			continue
		default:
			if currentPath == "" {
				continue
			}
			file := &files[indexByPath[currentPath]]
			switch {
			case strings.HasPrefix(line, "+"):
				file.Additions++
				processAddedLine(file, line[1:])
			case strings.HasPrefix(line, "-"):
				file.Deletions++
				processRemovedLine(file, line[1:])
			}
		}
	}

	return files
}

func newFileChange(repoRoot, path string) FileChange {
	base := filepath.Base(path)
	topDir := topDirectory(path)
	isGo := strings.HasSuffix(path, ".go")
	isTest := strings.HasSuffix(path, "_test.go")

	return FileChange{
		Path:           path,
		Category:       detectFileCategory(path),
		TopDir:         topDir,
		HasTestSibling: hasTestSibling(repoRoot, path),
		IsConfigLike:   isConfigLike(path),
		IsIDLLike:      isIDLLike(path),
		IsDocLike:      isDocLike(path),
		IsGoLike:       isGo,
		IsTestLike:     isTest || strings.Contains(base, "test"),
	}
}

func processAddedLine(file *FileChange, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	upper := strings.ToUpper(trimmed)
	if strings.Contains(upper, "TODO") || strings.Contains(upper, "FIXME") || strings.Contains(upper, "HACK") {
		file.AddedTodoLines = append(file.AddedTodoLines, trimmed)
	}
	if sensitiveLinePattern.MatchString(trimmed) || isSensitiveFilePath(file.Path) {
		file.AddedSensitive = append(file.AddedSensitive, trimmed)
	}
	if looksLikeDDL(trimmed) {
		file.AddedDDL = append(file.AddedDDL, trimmed)
	}
	if file.IsGoLike && exportedFuncPattern.MatchString(trimmed) {
		file.PublicSigAdded = append(file.PublicSigAdded, trimmed)
	}
}

func processRemovedLine(file *FileChange, line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	if looksLikeDDL(trimmed) {
		file.AddedDDL = append(file.AddedDDL, "removed: "+trimmed)
	}
	if file.IsGoLike && exportedFuncPattern.MatchString(trimmed) {
		file.PublicSigRemoved = append(file.PublicSigRemoved, trimmed)
	}
	if file.IsIDLLike && strings.Contains(trimmed, "required") {
		file.AddedDDL = append(file.AddedDDL, "removed required field: "+trimmed)
	}
}

func diffPath(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return ""
	}
	bPath := strings.TrimPrefix(parts[3], "b/")
	if bPath == "/dev/null" {
		return strings.TrimPrefix(parts[2], "a/")
	}
	return bPath
}

func detectFileCategory(path string) string {
	switch {
	case isDocLike(path):
		return "docs"
	case isConfigLike(path):
		return "config"
	case isIDLLike(path):
		return "idl"
	case strings.HasSuffix(path, ".sql") || strings.Contains(strings.ToLower(path), "migration"):
		return "db"
	case strings.HasSuffix(path, ".go"):
		return "go"
	default:
		return "other"
	}
}

func topDirectory(path string) string {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func isConfigLike(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, ".yaml") ||
		strings.HasSuffix(path, ".yml") ||
		strings.HasSuffix(path, ".toml") ||
		strings.HasSuffix(path, ".json") ||
		strings.Contains(path, "/config/") ||
		strings.Contains(path, "/conf/") ||
		strings.HasPrefix(path, "config/") ||
		strings.HasPrefix(path, "conf/")
}

func isIDLLike(path string) bool {
	return strings.HasSuffix(path, ".thrift") || strings.HasSuffix(path, ".proto")
}

func isDocLike(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".rst") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasPrefix(lower, "docs/") ||
		filepath.Base(lower) == "agents.md"
}

func hasTestSibling(repoRoot, path string) bool {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false
	}
	testPath := strings.TrimSuffix(path, ".go") + "_test.go"
	_, err := os.Stat(filepath.Join(repoRoot, testPath))
	return err == nil
}

func isSensitiveFilePath(path string) bool {
	lower := strings.ToLower(filepath.Base(path))
	return lower == "id_rsa" || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key")
}

func looksLikeDDL(line string) bool {
	upper := strings.ToUpper(line)
	for _, keyword := range []string{"ALTER TABLE", "DROP TABLE", "DROP COLUMN", "CREATE TABLE", "ADD COLUMN", "MODIFY COLUMN"} {
		if strings.Contains(upper, keyword) {
			return true
		}
	}
	return false
}

func appendIfMissing(items []string, item string) []string {
	if item == "" || slices.Contains(items, item) {
		return items
	}
	return append(items, item)
}
