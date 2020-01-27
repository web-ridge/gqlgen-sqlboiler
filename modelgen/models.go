package modelgen

import (
	"fmt"
	"go/types"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	pluralize "github.com/gertd/go-pluralize"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	"github.com/iancoleman/strcase"
	"github.com/vektah/gqlparser/ast"
)

var pathRegex *regexp.Regexp
var pluralizer *pluralize.Client

func init() {
	pluralizer = pluralize.NewClient()
	pathRegex, _ = regexp.Compile(`src\/(.*)`)
}

type ModelBuild struct {
	BackendModelsPath  string
	FrontendModelsPath string

	PackageName string
	Interfaces  []*Interface
	Models      []*Model
	Enums       []*Enum
	Scalars     []string
}

type Interface struct {
	Description string
	Name        string
}

type Model struct {
	Description string
	Name        string
	BoilerName  string
	PluralName  string
	Fields      []*Field
	Implements  []string
	IsInput     bool
	IsPayload   bool
	PreloadMap  map[string]ColumnSetting
}

type ColumnSetting struct {
	Name        string
	IDAvailable bool
}

type Field struct {
	Description            string
	Name                   string
	CamelCaseName          string
	PluralName             string
	BoilerName             string
	PluralBoilerName       string
	BoilerType             string
	GraphType              string
	Type                   types.Type
	Tag                    string
	IsCustomFunction       bool
	CustomFromFunction     string
	CustomToFunction       string
	CustomBoilerIDFunction string
	CustomGraphIDFunction  string
	IsId                   bool
	IsNullableID           bool
	IsRelation             bool
	IsPlural               bool
	CustomGraphType        string
	CustomBoilerType       string
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
		fmt.Println("error while trying to convert folder to gopath", err)
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

	// fmt.Println("cfg.LoadSchema()")
	schema, _, err := cfg.LoadSchema()
	if err != nil {
		return err
	}
	// fmt.Println("cfg.Autobind(schema)")
	err = cfg.Autobind(schema)
	if err != nil {
		return err
	}

	cfg.InjectBuiltins(schema)

	// fmt.Println("cfg.InjectBuiltins(schema)")
	binder, err := cfg.NewBinder(schema)
	if err != nil {
		return err
	}

	b := &ModelBuild{
		FrontendModelsPath: getGoImportFromFile(m.frontendModelsPath),
		BackendModelsPath:  getGoImportFromFile(m.backendModelsPath),
		PackageName:        "convert", // TODO convert?
	}

	boilerTypeMap, boilerStructMap := parseBoilerFile(m.backendModelsPath)

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
			if schemaType == schema.Query ||
				schemaType == schema.Mutation ||
				schemaType == schema.Subscription {
				continue
			}
			modelName := schemaType.Name
			boilerName, hasBoilerName := boilerStructMap[modelName]
			if !hasBoilerName {
				boilerName, hasBoilerName = boilerStructMap[strings.TrimSuffix(modelName, "Input")]
			}
			if !hasBoilerName {
				boilerName, hasBoilerName = boilerStructMap[strings.TrimSuffix(modelName, "Payload")]
			}
			// fmt.Println("GRAPHQL MODEL ::::", it.Name)
			if strings.HasPrefix(modelName, "_") {
				continue
			}

			// We will try to handle Input mutations to graphql objects
			if !hasBoilerName {
				fmt.Println(fmt.Sprintf("    [WARN] Skip  %v because it there is no database model found", modelName))
				continue
			}

			it := &Model{
				Description: schemaType.Description,
				Name:        modelName,
				PluralName:  pluralizer.Plural(modelName),
				BoilerName:  boilerName,
				IsInput:     strings.HasSuffix(modelName, "Input"),
				IsPayload:   strings.HasSuffix(modelName, "Payload"),
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

				if strings.HasPrefix(it.Name, "_") {
					continue
				}

				structKey := strcase.ToCamel(name)
				structKey = strings.Replace(structKey, "Id", "ID", -1)
				structKey = strings.Replace(structKey, "Url", "URL", -1)
				isId := strings.Contains(structKey, "ID")
				boilerKey := it.Name + "." + structKey
				// fmt.Println(boilerKey, ":", boilerType)
				boilerType, ok := boilerTypeMap[boilerKey]
				if it.IsInput {
					boilerKey := strings.TrimSuffix(it.Name, "Input") + "." + structKey
					boilerType, ok = boilerTypeMap[boilerKey]
				}

				// if it.IsPayload {
				// 	boilerKey := strings.TrimSuffix(it.Name, "Payload") + "." + structKey
				// 	boilerType, ok = boilerTypeMap[boilerKey]
				// }

				var customBoilerName string
				if !ok {
					// ok appearently there are are sometimes when key contains 'type' the struct name is printed before
					cn := strings.TrimPrefix(structKey, it.Name)
					secondKey := it.Name + "." + cn
					if isRelation {
						cn = strings.TrimPrefix(structKey, it.Name)
						secondKey = it.Name + "." + structKey + "ID"
						boilerType, ok = boilerTypeMap[secondKey]
						if ok {
							customBoilerName = cn
							boilerKey = secondKey
						}
					} else {
						cn = strings.TrimPrefix(structKey, it.Name)
						secondKey = it.Name + "." + cn
						boilerType, ok = boilerTypeMap[secondKey]
						if ok {
							customBoilerName = cn
							boilerKey = secondKey
						}
					}

					if !ok {
						if it.IsPayload {
							//ignore
						} else if strings.Contains(boilerKey, "ClientMutationID") {
							// ignore
						} else if strings.Contains(boilerKey, ".") && pluralizer.IsPlural(strings.Split(boilerKey, ".")[1]) {
							// ignore
							// 	Could not find boilerType for key:type =  Flow.FlowBlocks
						} else {
							fmt.Println("Could not find boilerType for key:type = ", boilerKey, ":", boilerType)
						}
					}

				}

				var isCustomFunction bool
				var customFromFunction string
				var customToFunction string
				var customGraphType string
				var customBoilerType string

				if typ.String() != boilerType {
					// fmt.Println(fmt.Sprintf("%v != %v", typ.String(), boilerType))

					graphType := typ.String()
					boilType := boilerType
					if strings.HasPrefix(graphType, "*") {
						graphType = strings.TrimPrefix(graphType, "*")
						graphType = strcase.ToCamel(graphType)
						graphType = "Pointer" + graphType
					}
					if strings.HasPrefix(boilType, "null.") {
						boilType = strings.TrimPrefix(boilType, "null.")
						boilType = strcase.ToCamel(boilType)
						boilType = "NullDot" + boilType
					}
					if strings.HasPrefix(boilType, "types.") {
						boilType = strings.TrimPrefix(boilType, "types.")
						boilType = strcase.ToCamel(boilType)
						boilType = "Types" + boilType
					}

					isCustomFunction = true

					customFromFunction = strcase.ToCamel(graphType) + "To" + strcase.ToCamel(boilType)
					customToFunction = strcase.ToCamel(boilType) + "To" + strcase.ToCamel(graphType)
					// fmt.Println("before", typ.String())
					customGraphType = strcase.ToCamel(graphType)

					customBoilerType = strcase.ToCamel(boilType)
					// fmt.Println("after", customGraphType)
				}

				// if isId {
				// 	fmt.Println(isId)
				// } else {
				// 	fmt.Println(name)
				// }
				// fmt.Println(boilerType)
				var boilerName string
				if customBoilerName != "" {
					boilerName = customBoilerName
				} else {
					boilerName = name
				}

				var customBoilerIDFunction string
				var customGraphIDFunction string
				var isNullableID bool
				if isId {
					// fmt.Println("isId")
					if boilerName == "id" {
						customBoilerIDFunction = it.BoilerName + "ID" + "Unique"
						customGraphIDFunction = it.BoilerName + "ID"
					} else {
						customBoilerIDFunction = boilerName + "Unique"
						customGraphIDFunction = name
					}
					// fmt.Println("boilerType", boilerType)
					// fmt.Println("graphType", typ.String())

					graphTypeIsNullable := strings.HasPrefix(typ.String(), "*")
					boilerTypeIsNullable := strings.HasPrefix(boilerType, "null.")
					isNullableID = graphTypeIsNullable || boilerTypeIsNullable
					if isNullableID {
						customBoilerIDFunction = customBoilerIDFunction + "Nullable"
						customGraphIDFunction = customGraphIDFunction + "Nullable"
					}

					if isNullableID && (!graphTypeIsNullable || !boilerTypeIsNullable) {
						fmt.Println(fmt.Printf(`
						ERROR: nullable differs in model: %v, 
						you should make it the same 
						schema name: %v is nullable=%v 
						boiler name:  %v is nullable=%v`,
							it.Name,
							name, graphTypeIsNullable,
							boilerName, boilerTypeIsNullable,
						))
					}
				}
				if name == "clientMutationId" {
					continue
				}

				if boilerType == "" {
					if it.IsPayload {
						// ignore
					} else if pluralizer.IsPlural(name) {
						// ignore
					} else {
						fmt.Println("[WARN] boiler type not available for, continue", name)
					}

				}

				it.Fields = append(it.Fields, &Field{
					IsId:                   isId,
					IsRelation:             isRelation,
					IsCustomFunction:       isCustomFunction,
					CustomFromFunction:     customFromFunction,
					CustomToFunction:       customToFunction,
					CustomBoilerIDFunction: customBoilerIDFunction,
					CustomGraphIDFunction:  customGraphIDFunction,
					CustomGraphType:        customGraphType,
					CustomBoilerType:       customBoilerType,
					IsNullableID:           isNullableID,
					BoilerType:             boilerType,
					GraphType:              typ.String(),
					Name:                   name,
					CamelCaseName:          strcase.ToLowerCamel(name),
					IsPlural:               pluralizer.IsPlural(name),
					PluralName:             pluralizer.Plural(name),
					BoilerName:             boilerName,
					PluralBoilerName:       pluralizer.Plural(boilerName),
					Type:                   typ,
					Description:            field.Description,
					Tag:                    `json:"` + field.Name + `"`,
				})

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
	enhanceModelsWithPreloadMap(b.Models)
	renderError := templates.Render(templates.Options{
		PackageName:     "convert",
		Filename:        m.filename,
		Data:            b,
		GeneratedHeader: true,
	})

	if renderError != nil {
		fmt.Println("renderError", renderError)
	}
	return nil
}

func isPreloadableModel(m *Model) bool {
	if m.IsInput {
		return false
	}
	return true
}

func enhanceModelsWithPreloadMap(models []*Model) {

	// first assing basic first level relations
	for _, model := range models {
		model.PreloadMap = make(map[string]ColumnSetting)
		if !isPreloadableModel(model) {
			continue
		}
		for _, field := range model.Fields {
			// we only preload relations :-D
			if !field.IsRelation {
				continue
			}
			if field.IsPlural {
				model.PreloadMap[field.PluralName] = ColumnSetting{
					Name: fmt.Sprintf("models.%vRels.%v", model.Name, strcase.ToCamel(field.BoilerName)),
				}
			} else {
				model.PreloadMap[field.Name] = ColumnSetting{
					Name:        fmt.Sprintf("models.%vRels.%v", model.Name, strcase.ToCamel(field.BoilerName)),
					IDAvailable: true,
				}
			}
		}
	}

	// second level
	for _, model := range models {
		if !isPreloadableModel(model) {
			continue
		}
		for _, field := range model.Fields {
			// we only preload relations :-D
			if !field.IsRelation {
				continue
			}

			// e.g this is the value in the map for the
			// model -->___FlowBlock___<---
			//       "block": helper.ColumnSetting{
			// 	            Name:        models.FlowBlockRels.Block,
			// 	            IDAvailable: true,
			//       },

			// models.FlowBlockRels.Block has also relations which
			// we want the relations of the models.FlowBlockRels.Block model

			// loop generated model maps
			for _, relationModel := range models {
				if relationModel.Name == field.BoilerName {
					for key, value := range relationModel.PreloadMap {

						var prefix string
						if field.IsPlural {
							prefix = fmt.Sprintf("models.%vRels.%v", model.Name, strcase.ToCamel(field.BoilerName))
						} else {
							prefix = fmt.Sprintf("models.%vRels.%v", model.Name, strcase.ToCamel(field.BoilerName))
						}

						model.PreloadMap[field.Name+"."+key] = ColumnSetting{
							Name: prefix + `+"."+` + value.Name,
						}
					}
				}
				// if field.IsPlural {
				// 	model.PreloadMap[field.PluralName] = ColumnSetting{
				// 		Name: fmt.Sprintf("models.%vRels.%v", model.Name, strcase.ToCamel(field.BoilerName)),
				// 	}
				// } else {
				// 	model.PreloadMap[field.Name] = ColumnSetting{
				// 		Name:        fmt.Sprintf("models.%vRels.%v", model.Name, strcase.ToCamel(field.BoilerName)),
				// 		IDAvailable: true,
				// 	}
				// }

			}

		}
	}
}

func isStruct(t types.Type) bool {
	_, is := t.Underlying().(*types.Struct)
	return is
}
