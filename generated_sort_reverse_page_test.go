package gbgen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/web-ridge/gqlgen-sqlboiler/v3/structs"
	"github.com/web-ridge/gqlgen-sqlboiler/v3/templates"
)

// hasPreviousPage on a forward page used to be a COUNT(*) over every row
// before the cursor, and without the connection's own mods: on a
// multi-tenant table that read the whole table on every page after the first.
// The generated <Model>ReversePageInformation must ask for one row within
// the original mods.
func TestGeneratedReversePageAsksOneRowWithinTheOriginalMods(t *testing.T) {
	file := renderGeneratedSort(t)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	reverse := findFunc(parsed, "UserReversePageInformation")
	if reverse == nil {
		t.Fatal("UserReversePageInformation not generated")
	}
	params := reverse.Type.Params.List
	last := params[len(params)-1]
	if _, variadic := last.Type.(*ast.Ellipsis); !variadic || last.Names[0].Name != "originalMods" {
		t.Fatalf("UserReversePageInformation must take originalMods ...qm.QueryMod as its last parameter")
	}
	calls := map[string]bool{}
	usesOriginalMods := false
	ast.Inspect(reverse.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			calls[node.Sel.Name] = true
		case *ast.Ident:
			if node.Name == "originalMods" {
				usesOriginalMods = true
			}
		}
		return true
	})
	if calls["Count"] {
		t.Fatal("UserReversePageInformation still counts the reversed range")
	}
	if !calls["QueryRowContext"] || !usesOriginalMods {
		t.Fatal("UserReversePageInformation must ask for one row within originalMods")
	}

	connection := findFunc(parsed, "UserConnection")
	passesMods := false
	ast.Inspect(connection.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || call.Ellipsis == token.NoPos {
			return true
		}
		if fn, ok := call.Fun.(*ast.Ident); ok && fn.Name == "UserReversePageInformation" {
			if arg, ok := call.Args[len(call.Args)-1].(*ast.Ident); ok && arg.Name == "originalMods" {
				passesMods = true
			}
		}
		return true
	})
	if !passesMods {
		t.Fatal("UserConnection must pass its originalMods to UserReversePageInformation")
	}
}

func renderGeneratedSort(t *testing.T) string {
	t.Helper()
	boilerModel := &structs.BoilerModel{Name: "User", PluralName: "Users", TableName: "User"}
	sortEnum := &structs.Enum{Name: "UserSort", Values: []*structs.EnumValue{{Name: "ID"}, {Name: "NAME"}}}
	data := &ConvertTemplateData{
		PackageName: "helpers",
		Backend:     structs.Config{Directory: "example.com/app/dm", PackageName: "dm"},
		Frontend:    structs.Config{Directory: "example.com/app/fm", PackageName: "fm"},
		Models: []*structs.Model{{
			Name:                  "UserOrdering",
			BoilerModel:           boilerModel,
			HasBoilerModel:        true,
			IsOrdering:            true,
			TableNameResolverName: "TableNames",
			Fields:                []*structs.Field{{Name: "Sort", Enum: sortEnum}},
		}},
	}
	content, err := getTemplateContent("generated_sort.gotpl")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "generated_sort.go")
	if err := templates.WriteTemplateFile(file, templates.Options{Template: content, PackageName: "helpers", Data: data}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
	return file
}

func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}
