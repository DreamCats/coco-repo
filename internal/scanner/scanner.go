package scanner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ScanResult 仓库扫描结果
type ScanResult struct {
	RepoRoot   string      // 仓库根目录
	DirTree    []string    // 目录结构（相对路径）
	GoPackages []PkgInfo   // Go 包信息
	GoTypes    []TypeInfo  // 导出的 Go 类型（struct/interface）
	RPCFiles   []string    // IDL 文件（proto/thrift）
}

// PkgInfo Go 包信息
type PkgInfo struct {
	Path    string // 相对路径
	Name    string // 包名
	Files   int    // 文件数量
}

// TypeInfo 导出的类型信息
type TypeInfo struct {
	Package  string // 所在包路径
	Name     string // 类型名
	Kind     string // struct / interface
	File     string // 所在文件
}

// Scan 扫描仓库，返回结构化结果
func Scan(repoRoot string) (*ScanResult, error) {
	result := &ScanResult{RepoRoot: repoRoot}

	if err := scanDirTree(repoRoot, result); err != nil {
		return nil, fmt.Errorf("扫描目录结构失败: %w", err)
	}

	if err := scanGoPackages(repoRoot, result); err != nil {
		return nil, fmt.Errorf("扫描 Go 包失败: %w", err)
	}

	scanIDLFiles(repoRoot, result)

	return result, nil
}

// Summary 返回扫描结果的文本摘要，作为 prompt 上下文
func (r *ScanResult) Summary() string {
	var sb strings.Builder

	sb.WriteString("## 目录结构\n\n")
	for _, dir := range r.DirTree {
		sb.WriteString(dir + "\n")
	}

	sb.WriteString("\n## Go 包\n\n")
	for _, pkg := range r.GoPackages {
		sb.WriteString(fmt.Sprintf("- %s (package %s, %d 文件)\n", pkg.Path, pkg.Name, pkg.Files))
	}

	sb.WriteString("\n## 导出类型\n\n")
	for _, t := range r.GoTypes {
		sb.WriteString(fmt.Sprintf("- %s.%s (%s) — %s\n", t.Package, t.Name, t.Kind, t.File))
	}

	if len(r.RPCFiles) > 0 {
		sb.WriteString("\n## IDL 文件\n\n")
		for _, f := range r.RPCFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return sb.String()
}

// scanDirTree 扫描目录树（排除隐藏目录、vendor 等）
func scanDirTree(root string, result *ScanResult) error {
	skipDirs := map[string]bool{
		".git": true, ".livecoding": true, "vendor": true, "node_modules": true,
		"kitex_gen": true, "dist": true, ".idea": true, ".vscode": true,
	}

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}
		if !info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}

		base := filepath.Base(path)
		if skipDirs[base] || strings.HasPrefix(base, ".") {
			return filepath.SkipDir
		}

		result.DirTree = append(result.DirTree, rel)
		return nil
	})
}

// scanGoPackages 扫描 Go 包和导出类型
func scanGoPackages(root string, result *ScanResult) error {
	fset := token.NewFileSet()
	pkgMap := make(map[string]*PkgInfo)

	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 跳过不需要的目录
		if info.IsDir() {
			base := filepath.Base(path)
			skip := map[string]bool{
				".git": true, "vendor": true, "node_modules": true,
				"kitex_gen": true, "dist": true,
			}
			if skip[base] || (strings.HasPrefix(base, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		dir := filepath.Dir(path)
		relDir, _ := filepath.Rel(root, dir)
		relFile, _ := filepath.Rel(root, path)

		f, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return nil // 跳过解析失败的文件
		}

		// 记录包信息
		if _, ok := pkgMap[relDir]; !ok {
			pkgInfo := &PkgInfo{Path: relDir, Name: f.Name.Name}
			pkgMap[relDir] = pkgInfo
			result.GoPackages = append(result.GoPackages, *pkgInfo)
		}
		// 更新文件计数（找到对应索引）
		for i := range result.GoPackages {
			if result.GoPackages[i].Path == relDir {
				result.GoPackages[i].Files++
				break
			}
		}

		// 提取导出类型
		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if !typeSpec.Name.IsExported() {
					continue
				}

				kind := "other"
				switch typeSpec.Type.(type) {
				case *ast.StructType:
					kind = "struct"
				case *ast.InterfaceType:
					kind = "interface"
				}

				result.GoTypes = append(result.GoTypes, TypeInfo{
					Package: relDir,
					Name:    typeSpec.Name.Name,
					Kind:    kind,
					File:    relFile,
				})
			}
		}

		return nil
	})
}

// scanIDLFiles 扫描 proto/thrift IDL 文件
func scanIDLFiles(root string, result *ScanResult) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".proto" || ext == ".thrift" {
			rel, _ := filepath.Rel(root, path)
			result.RPCFiles = append(result.RPCFiles, rel)
		}
		return nil
	})
}
