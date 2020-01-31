package convert

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
	pluralize "github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
	"github.com/vektah/gqlparser/ast"
	"github.com/web-ridge/gqlgen-sqlboiler/boiler"
)

var pathRegex *regexp.Regexp
var pluralizer *pluralize.Client

func init() {
	var initError error
	pluralizer = pluralize.NewClient()
	pathRegex, initError = regexp.Compile(`src\/(.*)`)
	if initError != nil {
		fmt.Println("could not compile the path regex")
	}
}

type ModelBuild struct {
	BackendModelsPath  string
	FrontendModelsPath string
	PackageName        string
	Interfaces         []*Interface
	Models             []*Model
	Enums              []*Enum
	Scalars            []string
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
	PureFields  []*ast.FieldDefinition
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
	IsID                   bool
	IsPrimaryID            bool
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

	b := &ModelBuild{
		FrontendModelsPath: getGoImportFromFile(m.frontendModelsPath),
		BackendModelsPath:  getGoImportFromFile(m.backendModelsPath),
		PackageName:        "convert", // TODO convert?
	}

	boilerTypeMap, boilerStructMap, _ := boiler.ParseBoilerFile(m.backendModelsPath)

	// get models based on the schema and sqlboiler structs
	models := getModelsFromSchema(schema, boilerStructMap)

	// Now we have all model's let enhance them with fields
	enhanceModelsWithFields(schema, cfg, models, boilerTypeMap)

	// Add preload maps
	enhanceModelsWithPreloadMap(models)

	// Add models to the build config
	b.Models = models
	interfaces, enums, scalars := getExtrasFromSchema(schema)
	b.Interfaces = interfaces
	b.Enums = enums
	b.Scalars = scalars
	// Sort in same order
	sort.Slice(b.Models, func(i, j int) bool { return b.Models[i].Name < b.Models[j].Name })
	for _, m := range b.Models {
		cfg.Models.Add(m.Name, cfg.Model.ImportPath()+"."+templates.ToGo(m.Name))
	}

	if len(b.Models) == 0 {
		fmt.Println("return nil")
		return nil
	}

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

// getFieldType check's if user has defined a
func getFieldType(binder *config.Binder, schema *ast.Schema, cfg *config.Config, field *ast.FieldDefinition) (types.Type, error) {
	var typ types.Type
	var err error

	fieldDef := schema.Types[field.Type.Name()]
	if cfg.Models.UserDefined(field.Type.Name()) {
		pkg, typeName := PkgAndType(cfg.Models[field.Type.Name()].Model[0])
		typ, err = binder.FindType(pkg, typeName)
		if err != nil {
			return typ, err
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

	return typ, err
}

func enhanceModelsWithFields(schema *ast.Schema, cfg *config.Config, models []*Model, boilerTypeMap map[string]string) {

	binder, binderErr := cfg.NewBinder(schema)
	if binderErr != nil {
		fmt.Println("could not bind config: ", binderErr)
		return
	}

	for _, m := range models {
		// skip boiler plate from ggqlgen, we only want the models
		if strings.HasPrefix(m.Name, "_") {
			continue
		}

		// Let's convert the pure ast fields to something usable for our template
		for _, field := range m.PureFields {
			fieldDef := schema.Types[field.Type.Name()]

			// This calls some qglgen boilerType which gets the gqlgen type
			typ, err := getFieldType(binder, schema, cfg, field)
			if err != nil {
				fmt.Println("Could not get field type from graphql schema: ", err)
			}

			name := field.Name
			if nameOveride := cfg.Models[m.Name].Fields[field.Name].FieldName; nameOveride != "" {
				// TODO: map overrides to sqlboiler the other way around?
				name = nameOveride
			}

			// just some (old) Relay clutter which is not needed anymore + we can't do anything with it
			if name == "clientMutationId" {
				continue
			}

			// override type struct with qqlgen code
			typ = binder.CopyModifiersFromAst(field.Type, typ)
			if isStruct(typ) && (fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject) {
				typ = types.NewPointer(typ)
			}

			// get golang friendly fieldName because we want to check if boiler name is available
			gqlFieldName := getgqlFieldName(name)

			// generate some booleans because these checks will be used a lot
			isRelation := fieldDef.Kind == ast.Object
			isID := strings.Contains(gqlFieldName, "ID")
			isPrimaryID := gqlFieldName == "ID"

			// get sqlboiler information of the field
			boilerName, _, boilerType := getBoilerKeyAndType(m, name, gqlFieldName, isRelation, boilerTypeMap)

			// get some custom convert functions if the fields are more advanced, like relationships or custom enums
			convertConfig := getConvertConfig(typ, boilerType)

			// get some custom ID functions because we want to support unique id's we need to add them to foreign keys
			convertIDConfig := getConvertConfigID(m, typ, name, boilerName, boilerType, isID)

			// log some warnings when fields could not be converted
			if boilerType == "" {
				// TODO: add filter + where here
				if m.IsPayload {
					// ignore
				} else if pluralizer.IsPlural(name) {
					// ignore
				} else {
					fmt.Println("[WARN] boiler type not available for, continue", name)
				}
			}

			m.Fields = append(m.Fields, &Field{
				IsID:                   isID,
				IsPrimaryID:            isPrimaryID,
				IsRelation:             isRelation,
				IsCustomFunction:       convertConfig.isCustom,
				CustomFromFunction:     convertConfig.customFrom,
				CustomToFunction:       convertConfig.customTo,
				CustomBoilerIDFunction: convertIDConfig.boilerIDFunc,
				CustomGraphIDFunction:  convertIDConfig.graphIDFunc,
				CustomGraphType:        convertConfig.customGraphType,
				CustomBoilerType:       convertConfig.customBoilerType,
				IsNullableID:           convertIDConfig.isNullableID,
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
	}
}

type IDConvertConfig struct {
	boilerIDFunc string
	graphIDFunc  string
	isNullableID bool
}

// func getModelBasedOnBoilerType
func getConvertConfigID(m *Model, typ types.Type, name string, boilerName string, boilerType string, isID bool) (cc IDConvertConfig) {
	if isID {
		// fmt.Println("isId")
		if boilerName == "id" {
			cc.boilerIDFunc = m.BoilerName + "ID" + "Unique"
			cc.graphIDFunc = m.BoilerName + "ID"
		} else {
			// TODO: We want to have the model name of the relationship of the foreign key

			cc.boilerIDFunc = boilerName + "Unique"
			cc.graphIDFunc = name
		}
		// fmt.Println("boilerType", boilerType)
		// fmt.Println("graphType", typ.String())

		graphTypeIsNullable := strings.HasPrefix(typ.String(), "*")
		boilerTypeIsNullable := strings.HasPrefix(boilerType, "null.")
		cc.isNullableID = graphTypeIsNullable || boilerTypeIsNullable
		if cc.isNullableID {
			cc.boilerIDFunc = cc.boilerIDFunc + "Nullable"
			cc.graphIDFunc = cc.graphIDFunc + "Nullable"
		}

		if cc.isNullableID && (!graphTypeIsNullable || !boilerTypeIsNullable) {
			fmt.Println(fmt.Printf(`
				WARNING: nullable differs in model: %v, 
				it's recommended to make it the same 
				schema name: %v is nullable=%v 
				boiler name:  %v is nullable=%v`,
				m.Name,
				name,
				graphTypeIsNullable,
				boilerName,
				boilerTypeIsNullable,
			))
		}
	}
	return
}

type ConvertConfig struct {
	isCustom         bool
	customFrom       string
	customTo         string
	customGraphType  string
	customBoilerType string
}

func getConvertConfig(typ types.Type, boilerType string) (cc ConvertConfig) {
	if typ.String() != boilerType {
		// fmt.Println(fmt.Sprintf("%v != %v", typ.String(), boilerType))

		graphType := typ.String()
		boilType := boilerType

		// Make this go-friendly for the helper/convert.go package
		if strings.HasPrefix(graphType, "*") {
			graphType = strings.TrimPrefix(graphType, "*")
			graphType = strcase.ToCamel(graphType)
			graphType = "Pointer" + graphType
		}

		// Make this go-friendly for the helper/convert.go package
		if strings.HasPrefix(boilType, "null.") {
			boilType = strings.TrimPrefix(boilType, "null.")
			boilType = strcase.ToCamel(boilType)
			boilType = "NullDot" + boilType
		}

		// Make this go-friendly for the helper/convert.go package
		if strings.HasPrefix(boilType, "types.") {
			boilType = strings.TrimPrefix(boilType, "types.")
			boilType = strcase.ToCamel(boilType)
			boilType = "Types" + boilType
		}

		cc.isCustom = true
		cc.customFrom = strcase.ToCamel(graphType) + "To" + strcase.ToCamel(boilType)
		cc.customTo = strcase.ToCamel(boilType) + "To" + strcase.ToCamel(graphType)
		cc.customGraphType = strcase.ToCamel(graphType)
		cc.customBoilerType = strcase.ToCamel(boilType)
	}

	return
}

func getBoilerKeyAndType(m *Model, originalFieldName string, gqlFieldName string, isRelation bool,
	boilerTypeMap map[string]string) (string, string, string) {
	boilerKey := m.Name + "." + gqlFieldName
	boilerType, ok := boilerTypeMap[boilerKey]
	if m.IsInput {
		boilerKey := strings.TrimSuffix(m.Name, "Input") + "." + gqlFieldName
		boilerType, ok = boilerTypeMap[boilerKey]
	}

	if m.IsPayload {
		boilerKey := strings.TrimSuffix(m.Name, "Payload") + "." + gqlFieldName
		boilerType, ok = boilerTypeMap[boilerKey]
	}

	boilerName := originalFieldName
	if !ok {
		// there are are cases when key contains 'type' the struct name is printed before
		cn := strings.TrimPrefix(gqlFieldName, m.Name)
		secondKey := m.Name + "." + cn
		if isRelation {
			cn = strings.TrimPrefix(gqlFieldName, m.Name)
			secondKey = m.Name + "." + gqlFieldName + "ID"
			boilerType, ok = boilerTypeMap[secondKey]
			if ok {
				boilerName = cn
				boilerKey = secondKey
			}
		} else {
			cn = strings.TrimPrefix(gqlFieldName, m.Name)
			secondKey = m.Name + "." + cn
			boilerType, ok = boilerTypeMap[secondKey]
			if ok {
				boilerName = cn
				boilerKey = secondKey
			}
		}

		if !ok {
			if m.IsPayload {
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

	return boilerName, boilerKey, boilerType
}

func getgqlFieldName(name string) string {
	gqlFieldName := strcase.ToCamel(name)

	// in golang Id = ID
	gqlFieldName = strings.Replace(gqlFieldName, "Id", "ID", -1)
	// in golang Url = URL
	gqlFieldName = strings.Replace(gqlFieldName, "Url", "URL", -1)

	return gqlFieldName
}

func getExtrasFromSchema(schema *ast.Schema) (interfaces []*Interface, enums []*Enum, scalars []string) {
	for _, schemaType := range schema.Types {
		switch schemaType.Kind {
		case ast.Interface, ast.Union:
			interfaces = append(interfaces, &Interface{
				Description: schemaType.Description,
				Name:        schemaType.Name,
			})

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

			enums = append(enums, it)
		case ast.Scalar:
			scalars = append(scalars, schemaType.Name)
		}
	}

	return
}

func getModelsFromSchema(schema *ast.Schema, boilerStructMap map[string]string) (models []*Model) {
	for _, schemaType := range schema.Types {

		// if cfg.Models.UserDefined(schemaType.Name) {
		// 	fmt.Println("continue")
		// 	continue
		// }

		switch schemaType.Kind {

		case ast.Object, ast.InputObject:
			{
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
				// fmt.Println("GRAPHQL MODEL ::::", m.Name)
				if strings.HasPrefix(modelName, "_") {
					continue
				}

				// We will try to handle Input mutations to graphql objects
				if !hasBoilerName {
					fmt.Println(fmt.Sprintf("    [WARN] Skip  %v because it there is no database model found", modelName))
					continue
				}

				m := &Model{
					Description: schemaType.Description,
					Name:        modelName,
					PluralName:  pluralizer.Plural(modelName),
					BoilerName:  boilerName,
					IsInput:     strings.HasSuffix(modelName, "Input"),
					IsPayload:   strings.HasSuffix(modelName, "Payload"),
				}

				for _, implementor := range schema.GetImplements(schemaType) {
					m.Implements = append(m.Implements, implementor.Name)
				}

				m.PureFields = append(m.PureFields, schemaType.Fields...)
				models = append(models, m)
			}
		}
	}
	return
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
