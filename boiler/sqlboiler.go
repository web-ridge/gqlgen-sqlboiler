package boiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
)

var ignoreFiles = []string{"boil_queries.go", "boil_table_names.go", "boil_types.go", "mysql_upsert.go"}

// will return StructName.key
// e.g.
// Address.ID: null.Integer
// Address.Longitude: null.String
// Address.Latitude : null.Decimal
// needed to generate the right convert code
func ParseBoilerFile(dir string) (map[string]string, map[string]string, map[string]int) {
	fieldsMap := make(map[string]string)
	structsMap := make(map[string]string)
	fieldsOrder := make(map[string]int)

	dir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Println("abs error", err)
		return fieldsMap, structsMap, fieldsOrder
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Println("readdir error", err)
		return fieldsMap, structsMap, fieldsOrder
	}

	fset := token.NewFileSet()
	for _, file := range files {
		// only pick .go files and ignore test files
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".go") ||
			strings.HasSuffix(strings.ToLower(file.Name()), "_test.go") {
			continue
		}

		filename := filepath.Join(dir, file.Name())
		if src, err := parser.ParseFile(fset, filename, nil, parser.ParseComments); err == nil {

			var i int
			for _, decl := range src.Decls {
				typeDecl, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}

				for _, spec := range typeDecl.Specs {

					safeTypeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					structsMap[safeTypeSpec.Name.String()] = safeTypeSpec.Name.String()

					safeStructDecl, ok := safeTypeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}

					for _, field := range safeStructDecl.Fields.List {
						switch xv := field.Type.(type) {
						case *ast.StarExpr:

							if len(field.Names) > 0 {
								name := field.Names[0].Name

								if si, ok := xv.X.(*ast.Ident); ok {

									k := safeTypeSpec.Name.String() + "." + name
									//https://stackoverflow.com/questions/28246970/how-to-parse-a-method-declaration
									fieldsMap[k] = si.Name
									fieldsOrder[k] = i
								}

							} else {
								// fmt.Println("len(field.Names) == 0", field)
							}

						case *ast.SelectorExpr:
							t, _ := field.Type.(*ast.SelectorExpr)
							name := field.Names[0].Name

							k := safeTypeSpec.Name.String() + "." + name

							fieldsMap[k] = t.X.(*ast.Ident).Name + "." + t.Sel.Name
							fieldsOrder[k] = i

						case *ast.Ident:

							t, _ := field.Type.(*ast.Ident) // The type as a string
							typeName := t.Name
							name := field.Names[0].Name //name as a string

							k := safeTypeSpec.Name.String() + "." + name
							fieldsMap[k] = typeName
							fieldsOrder[k] = i

						default:
							// fmt.Println("ignoring....", field.Names)
						}
						i++
					}
				}

			}

		}
	}

	// To store the keys in slice in sorted order
	var keys []string
	for k := range fieldsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// fmt.Println(" ")
	// fmt.Println(" ")
	// fmt.Println(" ")
	// fmt.Println("START OF MAP DUMP")
	// fmt.Println("START OF MAP DUMP")
	// fmt.Println("START OF MAP DUMP")
	// fmt.Println(" ")
	// fmt.Println(" ")
	// for _, key := range keys {
	// 	fmt.Println(key, ":", fieldsMap[key])
	// }
	// fmt.Println(" ")
	// fmt.Println(" ")
	// fmt.Println("END OF MAP DUMP")
	// fmt.Println("END OF MAP DUMP")
	// fmt.Println("END OF MAP DUMP")
	// fmt.Println(" ")
	// fmt.Println(" ")
	// fmt.Println(" ")

	return fieldsMap, structsMap, fieldsOrder
}
