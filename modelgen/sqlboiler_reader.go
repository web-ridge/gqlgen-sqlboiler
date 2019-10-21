package modelgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"
)

var ignoreFiles = []string{"boil_queries.go", "boil_table_names.go", "boil_types.go", "mysql_upsert.go"}

// will return StructName.key
// e.g.
// Address.ID: null.Integer
// Address.Longitude: null.String
// Address.Latitude : null.Decimal
// needed to generate the right convert code
func parseBoilerFile(dir string) map[string]string {
	m := make(map[string]string, 0)

	fmt.Println(dir)
	dir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Println("abs", err)
		return m
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Println("readdir", err)
		return m
	}

	fmt.Println("files found in dir!!", len(files))
	fset := token.NewFileSet()
	for _, file := range files {
		// only pick .go files and ignore test files
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".go") ||
			strings.HasSuffix(strings.ToLower(file.Name()), "_test.go") {
			continue
		}

		fmt.Println("file!!", file.Name())

		filename := filepath.Join(dir, file.Name())
		if src, err := parser.ParseFile(fset, filename, nil, parser.ParseComments); err == nil {

			var structName string
			for _, obj := range src.Scope.Objects {
				if !ast.IsExported(obj.Name) {
					continue
				}
				// ast
				if obj.Kind == ast.Typ &&
					!strings.HasSuffix(obj.Name, "Slice") {
					structName = obj.Name
					// break

				}
			}
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
					safeStructDecl, ok := safeTypeSpec.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, field := range safeStructDecl.Fields.List {
						// if t := reflect.TypeOf(field.Type); t.Kind() == reflect.Ptr {
						// 	fmt.Println("STRUCT?????", "*"+t.Elem().Name())
						// 	// return
						// } else {
						// 	fmt.Println("STRUCT?????", t.Name())
						// 	// return
						// }
						// fmt.Println("STRUCT?????", field.Type)
						// fmt.Println(field.Names[0], field.Type)
						switch field.Type.(type) {

						case *ast.SelectorExpr:
							t, _ := field.Type.(*ast.SelectorExpr)
							name := field.Names[0].Name
							// if strings.HasPrefix(name, "_") || strings.HasPrefix(structName, "_") {
							// 	continue
							// }
							key := structName + "." + name

							m[key] = t.X.(*ast.Ident).Name + "." + t.Sel.Name
						case *ast.StarExpr:
							// Used for pointers to external structs
						case *ast.ArrayType:
							fmt.Println("Array type....")
						case *ast.Ident:
							t, _ := field.Type.(*ast.Ident) // The type as a string
							typeName := t.Name
							name := field.Names[0].Name //name as a string
							// if strings.HasPrefix(name, "_") || strings.HasPrefix(structName, "_") {
							// 	continue
							// }
							// fmt.Println(name + " : " + typeName)
							m[structName+"."+name] = typeName
							// tag := ""
							// if field.Tag != nil {
							// 	tag = field.Tag.Value //the tag as a string
							// }

							// fmt.Println(stype)
							// fmt.Println(tag)

						default:

						}
					}
				}

			}
			// typeDecl := src.Decls[0].(*ast.GenDecl)

			// s
			// fields := structDecl.Fields.List

			// for _, field := range fields {
			// 	switch field.Type.(type) {
			// 	case *ast.Ident:
			// 		stype := field.Type.(*ast.Ident).Name // The type as a string
			// 		tag := ""
			// 		if field.Tag != nil {
			// 			tag = field.Tag.Value //the tag as a string
			// 		}
			// 		name := field.Names[0].Name //name as a string
			// 		fmt.Println(name, sType, tag)
			// 	default:
			// 	}
			// }

			// for _, obj := range src.Scope.Objects {
			// 	if !ast.IsExported(obj.Name) {
			// 		continue
			// 	}
			// 	// ast
			// 	if obj.Kind == ast.Typ {

			// 		fmt.Println(obj.Name)
			// 		// fmt.Println(obj.Kind)
			// 		// fmt.Println(obj.Data)
			// 		fmt.Println("Decleration")
			// 		fmt.Println(obj.Decl)
			// 		// fmt.Println(obj.Type)
			// 	}

			// }
			// return src.Name.Name
		}
	}
	fmt.Println(" ")
	fmt.Println(" ")
	fmt.Println(" ")
	fmt.Println("START OF MAP DUMP")
	fmt.Println("START OF MAP DUMP")
	fmt.Println("START OF MAP DUMP")
	fmt.Println(" ")
	fmt.Println(" ")
	for key, value := range m {
		fmt.Println(key, ":", value)
	}
	fmt.Println(" ")
	fmt.Println(" ")
	fmt.Println("END OF MAP DUMP")
	fmt.Println("END OF MAP DUMP")
	fmt.Println("END OF MAP DUMP")
	fmt.Println(" ")
	fmt.Println(" ")
	fmt.Println(" ")

	return m
}
