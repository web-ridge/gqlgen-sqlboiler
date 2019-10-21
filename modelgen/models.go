package modelgen

import (
	"fmt"
	"go/types"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	"github.com/vektah/gqlparser/ast"
)

var pathRegex *regexp.Regexp

func init() {
	pathRegex, _ = regexp.Compile(`src\/(.*)`)
}

type ModelBuild struct {
	BackendModelsPath  string
	FrontendModelsPath string

	PackageName string
	Interfaces  []*Interface
	Models      []*Object
	Enums       []*Enum
	Scalars     []string
}

type Interface struct {
	Description string
	Name        string
}

type Object struct {
	Description string
	Name        string
	Fields      []*Field
	Implements  []string
}

type Field struct {
	Description       string
	Name              string
	RelationName      string
	Type              types.Type
	Tag               string
	IsPointerToString bool
	IsId              bool
	IsRelation        bool
}

type Enum struct {
	Description string
	Name        string
	Values      []*EnumValue
}

type EnumValue struct {
	Description string
	Name        string
}

func New(filename, backendModelsPath, frontendModelsPath string) plugin.Plugin {
	return &Plugin{filename: filename, backendModelsPath: backendModelsPath, frontendModelsPath: frontendModelsPath}
}

type Plugin struct {
	filename           string
	backendModelsPath  string
	frontendModelsPath string
}

var _ plugin.ConfigMutator = &Plugin{}

func (m *Plugin) Name() string {
	return "convert-generator"
}

func copyConfig(cfg config.Config) *config.Config {
	return &cfg
}

func getGoImportFromFile(dir string) string {
	// graphql_models

	longPath, err := filepath.Abs(dir)
	if err != nil {
		fmt.Println("abs 1", err)
	}
	// src/Users/richardlindhout/go/src/gitlab.com/eyeontarget/app/backend/graphql_models
	return strings.TrimPrefix(pathRegex.FindString(longPath), "src/")

}

func (m *Plugin) MutateConfig(ignoredConfig *config.Config) error {
	// fmt.Println("cfg.Check()")
	// if err := cfg.Check(); err != nil {
	// 	return err
	// }
	cfg := copyConfig(*ignoredConfig)

	fmt.Println("cfg.LoadSchema()")
	schema, _, err := cfg.LoadSchema()
	if err != nil {
		return err
	}
	fmt.Println("cfg.Autobind(schema)")
	err = cfg.Autobind(schema)
	if err != nil {
		return err
	}

	cfg.InjectBuiltins(schema)

	fmt.Println("cfg.InjectBuiltins(schema)")
	binder, err := cfg.NewBinder(schema)
	if err != nil {
		return err
	}

	b := &ModelBuild{
		FrontendModelsPath: getGoImportFromFile(m.frontendModelsPath),
		BackendModelsPath:  getGoImportFromFile(m.backendModelsPath),
		PackageName:        cfg.Model.Package,
	}

	parseBoilerFile(m.backendModelsPath)

	for _, schemaType := range schema.Types {

		// if cfg.Models.UserDefined(schemaType.Name) {
		// 	fmt.Println("continue")
		// 	continue
		// }

		switch schemaType.Kind {
		case ast.Interface, ast.Union:
			it := &Interface{
				Description: schemaType.Description,
				Name:        schemaType.Name,
			}

			b.Interfaces = append(b.Interfaces, it)
		case ast.Object, ast.InputObject:
			if schemaType == schema.Query || schemaType == schema.Mutation || schemaType == schema.Subscription {
				continue
			}
			it := &Object{
				Description: schemaType.Description,
				Name:        schemaType.Name,
			}

			for _, implementor := range schema.GetImplements(schemaType) {
				it.Implements = append(it.Implements, implementor.Name)
			}

			for _, field := range schemaType.Fields {
				var typ types.Type
				fieldDef := schema.Types[field.Type.Name()]

				if cfg.Models.UserDefined(field.Type.Name()) {
					pkg, typeName := PkgAndType(cfg.Models[field.Type.Name()].Model[0])
					typ, err = binder.FindType(pkg, typeName)
					if err != nil {
						return err
					}
				} else {

					switch fieldDef.Kind {
					case ast.Scalar:
						// no user defined model, referencing a default scalar
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), "string", nil),
							nil,
							nil,
						)

					case ast.Interface, ast.Union:
						// no user defined model, referencing a generated interface type
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), templates.ToGo(field.Type.Name()), nil),
							types.NewInterfaceType([]*types.Func{}, []types.Type{}),
							nil,
						)

					case ast.Enum:
						// no user defined model, must reference a generated enum
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), templates.ToGo(field.Type.Name()), nil),
							nil,
							nil,
						)

					case ast.Object, ast.InputObject:
						// no user defined model, must reference a generated struct
						typ = types.NewNamed(
							types.NewTypeName(0, cfg.Model.Pkg(), templates.ToGo(field.Type.Name()), nil),
							types.NewStruct(nil, nil),
							nil,
						)

					default:
						panic(fmt.Errorf("unknown ast type %s", fieldDef.Kind))
					}
				}

				name := field.Name
				if nameOveride := cfg.Models[schemaType.Name].Fields[field.Name].FieldName; nameOveride != "" {
					name = nameOveride
				}

				// fmt.Println(fieldDef)
				// fmt.Println("-----       -----")
				typ = binder.CopyModifiersFromAst(field.Type, typ)
				isRelation := fieldDef.Kind == ast.Object
				// fmt.Println(isRelation, "isRelation")
				// fmt.Println(ast.Object, "ast.Object")
				// fmt.Println(typ, "typ")

				if isStruct(typ) && (fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject) {
					typ = types.NewPointer(typ)
				}

				isId := name == "id"
				isPointerToString := typ.String() == "*string"
				// if isId {
				// 	fmt.Println(isId)
				// } else {
				// 	fmt.Println(name)
				// }

				it.Fields = append(it.Fields, &Field{
					IsId:              isId,
					IsRelation:        isRelation,
					IsPointerToString: isPointerToString,
					Name:              name,
					RelationName:      fieldDef.Name,
					Type:              typ,
					Description:       field.Description,
					Tag:               `json:"` + field.Name + `"`,
				})
			}

			if strings.HasPrefix(it.Name, "_") {
				continue
			}

			b.Models = append(b.Models, it)
		case ast.Enum:
			it := &Enum{
				Name:        schemaType.Name,
				Description: schemaType.Description,
			}

			for _, v := range schemaType.EnumValues {
				it.Values = append(it.Values, &EnumValue{
					Name:        v.Name,
					Description: v.Description,
				})
			}

			b.Enums = append(b.Enums, it)
		case ast.Scalar:
			b.Scalars = append(b.Scalars, schemaType.Name)
		}
	}

	sort.Slice(b.Enums, func(i, j int) bool { return b.Enums[i].Name < b.Enums[j].Name })
	sort.Slice(b.Models, func(i, j int) bool { return b.Models[i].Name < b.Models[j].Name })
	sort.Slice(b.Interfaces, func(i, j int) bool { return b.Interfaces[i].Name < b.Interfaces[j].Name })

	for _, it := range b.Enums {
		cfg.Models.Add(it.Name, cfg.Model.ImportPath()+"."+templates.ToGo(it.Name))
	}
	for _, it := range b.Models {
		cfg.Models.Add(it.Name, cfg.Model.ImportPath()+"."+templates.ToGo(it.Name))
	}
	for _, it := range b.Interfaces {
		cfg.Models.Add(it.Name, cfg.Model.ImportPath()+"."+templates.ToGo(it.Name))
	}
	for _, it := range b.Scalars {
		cfg.Models.Add(it, "github.com/99designs/gqlgen/graphql.String")
	}

	if len(b.Models) == 0 && len(b.Enums) == 0 {
		fmt.Println("return nil")
		return nil
	}

	renderError := templates.Render(templates.Options{
		PackageName:     cfg.Model.Package,
		Filename:        m.filename,
		Data:            b,
		GeneratedHeader: true,
	})

	if renderError != nil {
		fmt.Println("renderError", renderError)
	}
	return nil
}

func isStruct(t types.Type) bool {
	_, is := t.Underlying().(*types.Struct)
	return is
}
