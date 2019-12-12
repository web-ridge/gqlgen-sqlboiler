package modelgen

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
func parseBoilerFile(dir string) (map[string]string, map[string]string) {
	fieldsMap := make(map[string]string, 0)
	structsMap := make(map[string]string, 0)
	// fmt.Println(dir)
	dir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Println("abs error", err)
		return fieldsMap, structsMap
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Println("readdir error", err)
		return fieldsMap, structsMap
	}

	fset := token.NewFileSet()
	for _, file := range files {
		// only pick .go files and ignore test files
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".go") ||
			strings.HasSuffix(strings.ToLower(file.Name()), "_test.go") {
			continue
		}

		// fmt.Println("file!!", file.Name())

		filename := filepath.Join(dir, file.Name())
		if src, err := parser.ParseFile(fset, filename, nil, parser.ParseComments); err == nil {

			// var structName string
			// for _, obj := range src.Scope.Objects {
			// 	if !ast.IsExported(obj.Name) {
			// 		continue
			// 	}
			// 	// ast
			// 	if obj.Kind == ast.Typ &&
			// 		!strings.HasSuffix(obj.Name, "Slice") {
			// 		structName = obj.Name
			// 		// break

			// 	}
			// }
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
							key := safeTypeSpec.Name.String() + "." + name

							fieldsMap[key] = t.X.(*ast.Ident).Name + "." + t.Sel.Name
						// case *ast.StarExpr:
						// 	// Used for pointers to external structs
						// case *ast.ArrayType:

						case *ast.Ident:
							t, _ := field.Type.(*ast.Ident) // The type as a string
							typeName := t.Name
							name := field.Names[0].Name //name as a string
							// if strings.HasPrefix(name, "_") || strings.HasPrefix(structName, "_") {
							// 	continue
							// }
							// fmt.Println(name + " : " + typeName)
							fieldsMap[safeTypeSpec.Name.String()+"."+name] = typeName
							// tag := ""
							// if field.Tag != nil {
							// 	tag = field.Tag.Value //the tag as a string
							// }

							// fmt.Println(stype)
							// fmt.Println(tag)

						default:
							// fmt.Println("ignoring....", field.Names)
						}
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

	return fieldsMap, structsMap
}
