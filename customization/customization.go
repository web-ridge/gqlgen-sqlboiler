package customization

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

func GetFunctionNamesFromDir(dir string, ignore []string) []string {
	var a []string
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, dir, nil, 0)
	if err != nil {
		fmt.Println("Failed to parse package:", err)
		os.Exit(1)
	}

	for _, pack := range packs {
		for fileName, file := range pack.Files {
			simpleName := strings.TrimPrefix(fileName, dir+"/")
			if !contains(ignore, simpleName) {
				a = append(a, GetFunctionNamesFromAstFile(file)...)
			}

		}
	}
	return a
}

func GetAstFileFromString(fileContent string) (*ast.File, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "src.go", fileContent, 0)
	if err != nil {
		return nil, fmt.Errorf("can not get node: %v", err)
	}
	return node, nil
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
