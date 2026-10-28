package customization

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

func GetFunctionNamesFromDir(dir string, ignore []string) ([]string, error) {
	var a []string
	set := token.NewFileSet()
	// Use filter function to skip ignored files DURING parsing, not after
	// This prevents parse errors from generated files that may be empty/malformed
	filterFunc := func(info os.FileInfo) bool {
		return !contains(ignore, info.Name())
	}
	packs, err := parser.ParseDir(set, dir, filterFunc, 0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to parse package: %v", err)
	}

	for _, pack := range packs {
		for _, file := range pack.Files {
			a = append(a, GetFunctionNamesFromAstFile(file)...)
		}
	}
	return a, nil
}

func GetFunctionNamesFromAstFile(node *ast.File) []string {
	var a []string

	ast.Inspect(node, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if ok {
			a = append(a, fn.Name.Name)
		}
		return true
	})
	return a
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// GetExportedFunctionNamesFromDir lists the exported package-level functions
// of the non-test files in dir.
func GetExportedFunctionNamesFromDir(dir string) ([]string, error) {
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, dir, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package: %v", err)
	}
	var a []string
	for _, pack := range packs {
		for _, file := range pack.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && fn.Name.IsExported() {
					a = append(a, fn.Name.Name)
				}
			}
		}
	}
	return a, nil
}
