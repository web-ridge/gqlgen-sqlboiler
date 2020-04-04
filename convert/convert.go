package convert

import (
	"fmt"
	"go/types"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	pluralize "github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
	"github.com/vektah/gqlparser/v2/ast"
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
	Name                  string
	PluralName            string
	BoilerModel           boiler.BoilerModel
	Fields                []*Field
	IsInput               bool
	IsCreateInput         bool
	IsUpdateInput         bool
	IsNormalInput         bool
	IsPayload             bool
	PreloadMap            map[string]ColumnSetting
	HasOrganizationID     bool
	HasUserOrganizationID bool
	HasUserID             bool
	// other stuff
	Description string
	PureFields  []*ast.FieldDefinition
	Implements  []string
}

type ColumnSetting struct {
	Name                  string
	RelationshipModelName string
	IDAvailable           bool
}

type Field struct {
	Name          string
	PluralName    string
	Type          string
	IsID          bool
	IsPrimaryID   bool
	IsRequired    bool
	IsPlural      bool
	ConvertConfig ConvertConfig
	// relation stuff
	IsRelation bool
	// boiler relation stuff is inside this field
	BoilerField boiler.BoilerField
	// graphql relation ship can be found here
	Relationship *Model

	// Some stuff
	Description  string
	OriginalType types.Type
	Tag          string
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

func New(directory, backendModelsPath, frontendModelsPath string) plugin.Plugin {
	return &Plugin{directory: directory, backendModelsPath: backendModelsPath, frontendModelsPath: frontendModelsPath}
}

type Plugin struct {
	directory          string
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
	longPath, err := filepath.Abs(dir)
	if err != nil {
		fmt.Println("error while trying to convert folder to gopath", err)
	}
	// src/Users/.../go/src/gitlab.com/.../app/backend/graphql_models
	return strings.TrimPrefix(pathRegex.FindString(longPath), "src/")
}

func GetModelsWithInformation(cfg *config.Config, boilerModels []*boiler.BoilerModel) []*Model {

	// get models based on the schema and sqlboiler structs
	models := getModelsFromSchema(cfg.Schema, boilerModels)

	// Now we have all model's let enhance them with fields
	enhanceModelsWithFields(cfg.Schema, cfg, models)

	// Add preload maps
	enhanceModelsWithPreloadMap(models)

	// Sort in same order
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	for _, m := range models {
		cfg.Models.Add(m.Name, cfg.Model.ImportPath()+"."+templates.ToGo(m.Name))
	}
	return models
}

func (m *Plugin) MutateConfig(originalCfg *config.Config) error {
	b := &ModelBuild{
		PackageName:        m.directory,
		FrontendModelsPath: getGoImportFromFile(m.frontendModelsPath),
		BackendModelsPath:  getGoImportFromFile(m.backendModelsPath),
	}

	cfg := copyConfig(*originalCfg)

	fmt.Println("[convert] get boiler models")
	boilerModels := boiler.GetBoilerModels(m.backendModelsPath)

	fmt.Println("[convert] get model with information")
	models := GetModelsWithInformation(originalCfg, boilerModels)

	fmt.Println("[convert] get extra's from schema")
	interfaces, enums, scalars := getExtrasFromSchema(cfg.Schema)
	b.Models = models
	b.Interfaces = interfaces
	b.Enums = enums
	b.Scalars = scalars
	if len(b.Models) == 0 {
		fmt.Println("return nil")
		return nil
	}

	// for _, model := range models {
	// 	fmt.Println(model.Name, "->", model.BoilerModel.Name)
	// 	for _, field := range model.Fields {
	// 		fmt.Println("    ", field.Name, field.Type)
	// 		fmt.Println("    ", field.BoilerField.Name, field.BoilerField.Type)
	// 	}
	// }

	fmt.Println("[convert] render preload.gotpl")
	templates.CurrentImports = nil
	if renderError := templates.Render(templates.Options{
		Template:        getTemplate("preload.gotpl"),
		PackageName:     m.directory,
		Filename:        m.directory + "/" + "preload.go",
		Data:            b,
		GeneratedHeader: true,
		Packages:        cfg.Packages,
	}); renderError != nil {
		fmt.Println("renderError", renderError)
	}
	templates.CurrentImports = nil
	fmt.Println("[convert] render convert.gotpl")
	if renderError := templates.Render(templates.Options{
		Template:        getTemplate("convert.gotpl"),
		PackageName:     m.directory,
		Filename:        m.directory + "/" + "convert.go",
		Data:            b,
		GeneratedHeader: true,
		Packages:        cfg.Packages,
	}); renderError != nil {
		fmt.Println("renderError", renderError)
	}
	templates.CurrentImports = nil
	fmt.Println("[convert] render convert_input.gotpl")
	if renderError := templates.Render(templates.Options{
		Template:        getTemplate("convert_input.gotpl"),
		PackageName:     m.directory,
		Filename:        m.directory + "/" + "convert_input.go",
		Data:            b,
		GeneratedHeader: true,
		Packages:        cfg.Packages,
	}); renderError != nil {
		fmt.Println("renderError", renderError)
	}
	templates.CurrentImports = nil
	fmt.Println("[convert] render filter.gotpl")
	if renderError := templates.Render(templates.Options{
		Template:        getTemplate("filter.gotpl"),
		PackageName:     m.directory,
		Filename:        m.directory + "/" + "filter.go",
		Data:            b,
		GeneratedHeader: true,
		Packages:        cfg.Packages,
	}); renderError != nil {
		fmt.Println("renderError", renderError)
	}

	return nil
}

func getTemplate(filename string) string {
	// load path relative to calling source file
	_, callerFile, _, _ := runtime.Caller(1)
	rootDir := filepath.Dir(callerFile)
	content, err := ioutil.ReadFile(path.Join(rootDir, filename))
	if err != nil {
		fmt.Println("Could not read .gotpl file", err)
		return "Could not read .gotpl file"
	}
	return string(content)
}

// getFieldType check's if user has defined a
func getFieldType(binder *config.Binder, schema *ast.Schema, cfg *config.Config, field *ast.FieldDefinition) (types.Type, error) {
	var typ types.Type
	var err error

	fieldDef := schema.Types[field.Type.Name()]
	if cfg.Models.UserDefined(field.Type.Name()) {
		typ, err = binder.FindTypeFromName(cfg.Models[field.Type.Name()].Model[0])
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

func getPlularBoilerRelationShipName(modelName string) string {
	// sqlboiler adds Slice when multiple, we don't want that
	// since our converts are named plular of model and not Slice
	// e.g. UsersToGraphQL and not UserSliceToGraphQL
	modelName = strings.TrimSuffix(modelName, "Slice")
	return pluralizer.Plural(modelName)
}

func enhanceModelsWithFields(schema *ast.Schema, cfg *config.Config, models []*Model) {

	binder := cfg.NewBinder()

	// Generate the basic of the fields
	for _, m := range models {

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

			// just some (old) Relay clutter which is not needed anymore + we won't do anything with it
			// in our database converts.
			if name == "clientMutationId" {
				continue
			}

			// override type struct with qqlgen code
			typ = binder.CopyModifiersFromAst(field.Type, typ)
			if isStruct(typ) && (fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject) {
				typ = types.NewPointer(typ)
			}

			// get golang friendly fieldName because we want to check if boiler name is available
			golangName := getgqlFieldName(name)

			// generate some booleans because these checks will be used a lot
			isRelation := fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject

			isID := strings.Contains(golangName, "ID")
			isPrimaryID := golangName == "ID"

			// get sqlboiler information of the field
			boilerField := findBoilerField(m.BoilerModel.Fields, golangName, isRelation)

			// log some warnings when fields could not be converted
			if boilerField.Type == "" {
				// TODO: add filter + where here
				if m.IsPayload {
					// ignore
				} else if pluralizer.IsPlural(name) {
					// ignore
				} else {
					fmt.Println("[WARN] boiler type not available for ", name)
				}
			}

			if boilerField.Name == "" {
				if m.IsPayload {
				} else {
					fmt.Println("[WARN] boiler name not available for ", m.Name+"."+golangName)
				}
			}
			field := &Field{
				Name:         name,
				Type:         typ.String(),
				BoilerField:  boilerField,
				IsID:         isID,
				IsPrimaryID:  isPrimaryID,
				IsRelation:   isRelation,
				IsPlural:     pluralizer.IsPlural(name),
				PluralName:   pluralizer.Plural(name),
				OriginalType: typ,
				Description:  field.Description,
				Tag:          `json:"` + field.Name + `"`,
			}
			field.ConvertConfig = getConvertConfig(m, field)
			m.Fields = append(m.Fields, field)
		}
	}

	for _, m := range models {
		m.HasOrganizationID = findField(m.Fields, "organizationId") != nil
		m.HasUserOrganizationID = findField(m.Fields, "userOrganizationId") != nil
		m.HasUserID = findField(m.Fields, "userId") != nil
		for _, f := range m.Fields {
			f.Relationship = findModel(models, f.BoilerField.Relationship.Name)
		}
	}
}

func findModel(models []*Model, search string) *Model {
	for _, m := range models {
		if m.Name == search {
			return m
		}
	}
	return nil
}

func findField(fields []*Field, search string) *Field {
	for _, f := range fields {
		if f.Name == search {
			return f
		}
	}
	return nil
}
func findRelationModelForForeignKeyAndInput(currentModelName string, foreignKey string, models []*Model) *Field {
	return findRelationModelForForeignKey(getBaseModelFromName(currentModelName), foreignKey, models)
}

func findRelationModelForForeignKey(currentModelName string, foreignKey string, models []*Model) *Field {

	model := findModel(models, currentModelName)
	if model != nil {
		// Use case
		// we want a foreignKey of ParentID but the foreign key resolves to Calamity
		// We could know this based on the boilerType information
		// withou this function the generated convert is like this

		// r.Parent = ParentToGraphQL(m.R.Parent, m)
		// but it needs to be
		// r.Parent = CalamityToGraphQL(m.R.Parent, m)
		foreignKey = strings.TrimSuffix(foreignKey, "Id")

		field := findField(model.Fields, foreignKey)
		if field != nil {
			// fmt.Println("Found graph type", field.Name, "for foreign key", foreignKey)
			return field
		}
	}

	return nil
}

func findBoilerField(fields []*boiler.BoilerField, golangGraphQLName string, isRelation bool) boiler.BoilerField {
	// get database friendly struct for this model
	for _, field := range fields {
		if isRelation {
			// If it a relation check to see if a foreign key is available
			if field.Name == golangGraphQLName+"ID" {
				return *field
			}
		}
		if field.Name == golangGraphQLName {
			return *field
		}
	}

	// // fallback on foreignKey

	// }

	// fmt.Println("???", golangGraphQLName)

	return boiler.BoilerField{}
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

func getModelsFromSchema(schema *ast.Schema, boilerModels []*boiler.BoilerModel) (models []*Model) {
	for _, schemaType := range schema.Types {

		// skip boiler plate from ggqlgen, we only want the models
		if strings.HasPrefix(schemaType.Name, "_") {
			continue
		}

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

				// fmt.Println("GRAPHQL MODEL ::::", m.Name)
				if strings.HasPrefix(modelName, "_") {
					continue
				}

				// We will try to find a corresponding boiler struct
				boilerModel := boiler.FindBoilerModel(boilerModels, getBaseModelFromName(modelName))

				// if no boiler model is found
				if boilerModel.Name == "" {
					if strings.HasSuffix(modelName, "Filter") && modelName != "Filter" {
						// silent continue
						continue
					}
					if strings.HasSuffix(modelName, "Payload") && modelName != "Payload" {
						// silent continue
						continue
					}
					if strings.HasSuffix(modelName, "Input") && modelName != "Input" {
						// silent continue
						continue
					}
					if strings.HasSuffix(modelName, "Where") && modelName != "Where" {
						// silent continue
						continue
					}

					fmt.Println(fmt.Sprintf("[WARN] Skip %v because no database model found", modelName))
					continue
				}

				isInput := strings.HasSuffix(modelName, "Input")
				isCreateInput := strings.HasSuffix(modelName, "CreateInput")
				isUpdateInput := strings.HasSuffix(modelName, "UpdateInput")
				isNormalInput := isInput && !isCreateInput && !isUpdateInput

				m := &Model{
					Name:          modelName,
					Description:   schemaType.Description,
					PluralName:    pluralizer.Plural(modelName),
					BoilerModel:   boilerModel,
					IsInput:       isInput,
					IsUpdateInput: isUpdateInput,
					IsCreateInput: isCreateInput,
					IsNormalInput: isNormalInput,
					IsPayload:     strings.HasSuffix(modelName, "Payload"),
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

func getPreloadMapForModel(model *Model) map[string]ColumnSetting {
	preloadMap := map[string]ColumnSetting{}
	for _, field := range model.Fields {
		// only relations are preloadable
		if !field.IsRelation {
			continue
		}
		// var key string
		// if field.IsPlural {
		key := field.Name
		// } else {
		// 	key = field.PluralName
		// }
		name := fmt.Sprintf("models.%vRels.%v", model.Name, foreignKeyToRel(field.BoilerField.Name))
		setting := ColumnSetting{
			Name:                  name,
			IDAvailable:           !field.IsPlural,
			RelationshipModelName: field.BoilerField.Relationship.Name,
		}

		preloadMap[key] = setting
	}
	return preloadMap
}

func enhanceModelsWithPreloadMap(models []*Model) {
	preloadMapPerModel := map[string]map[string]ColumnSetting{}
	// first assing basic first level relations
	for _, model := range models {
		if !isPreloadableModel(model) {
			continue
		}
		preloadMapPerModel[model.Name] = getPreloadMapForModel(model)
		model.PreloadMap = getPreloadMapForModel(model)
	}

	// reverse loop since nested count works that way
	// otherwise too much fields are added on the last models
	for i := len(models) - 1; i >= 0; i-- {
		model := models[i]
		if !isPreloadableModel(model) {
			continue
		}
		enhancePreloadMapWithNestedRelations(model.PreloadMap, preloadMapPerModel, 0)
	}

}

func enhancePreloadMapWithNestedRelations(preloadMap map[string]ColumnSetting, preloadMapPerModel map[string]map[string]ColumnSetting, nested int) {
	if nested > 5 {
		return
	}
	for key, value := range preloadMap {

		// check if relation exist
		if value.RelationshipModelName != "" {
			nestedPreloads, ok := preloadMapPerModel[value.RelationshipModelName]
			if ok {
				for nestedKey, nestedValue := range nestedPreloads {
					preloadMap[key+`.`+nestedKey] = ColumnSetting{
						Name:                  value.Name + `+ "." +` + nestedValue.Name,
						RelationshipModelName: nestedValue.RelationshipModelName,
					}
				}

			}
		}
	}
}

// The relationship is defined in the normal model but not in the input, where etc structs
// So just find the normal model and get the relationship type :)
func getBaseModelFromName(v string) string {
	v = strings.TrimSuffix(v, "CreateInput")
	v = strings.TrimSuffix(v, "UpdateInput")
	v = strings.TrimSuffix(v, "Input")
	v = strings.TrimSuffix(v, "Payload")
	return v
}

func foreignKeyToRel(v string) string {
	return strings.TrimSuffix(strcase.ToCamel(v), "ID")
}

func isStruct(t types.Type) bool {
	_, is := t.Underlying().(*types.Struct)
	return is
}

type ConvertConfig struct {
	IsCustom         bool
	ToBoiler         string
	ToGraphQL        string
	GraphTypeAsText  string
	BoilerTypeAsText string
}

func getConvertConfig(model *Model, field *Field) (cc ConvertConfig) {
	graphType := field.Type
	boilType := field.BoilerField.Type

	// fmt.Println("boilType for", field.Name, ":", boilType)

	if graphType != boilType {
		cc.IsCustom = true

		if field.IsPrimaryID || field.IsID {

			cc.ToGraphQL = "VALUE"
			cc.ToBoiler = "VALUE"

			// first unpointer json type if is pointer
			if strings.HasPrefix(graphType, "*") {
				cc.ToBoiler = "helper.PointerStringToString(VALUE)"
			}

			goToUint := getBoilerTypeAsText(boilType) + "ToUint"
			if goToUint != "UintToUint" {
				cc.ToGraphQL = "helper." + goToUint + "(VALUE)"
			}

			if field.IsPrimaryID {
				cc.ToGraphQL = model.Name + "IDToGraphQL(" + cc.ToGraphQL + ")"
			} else if field.IsID {
				cc.ToGraphQL = field.BoilerField.Relationship.Name + "IDToGraphQL(" + cc.ToGraphQL + ")"
			}

			cc.ToBoiler = fmt.Sprintf("helper.StringToIntID(%v)", cc.ToGraphQL)

			// unpointer id type

			// cc.ToBoiler = cc.ToGraphQL
			// StringToIntID

			// cc.ToGraphQL = model.Name + "ID" + "Unique"
			// cc.ToBoiler = "StringToIntID"
			cc.ToGraphQL = strings.Replace(cc.ToGraphQL, "VALUE", "m."+strcase.ToCamel(field.BoilerField.Name), -1)
			cc.ToBoiler = strings.Replace(cc.ToGraphQL, "VALUE", "m."+strcase.ToCamel(field.Name), -1)

		} else {
			// Make these go-friendly for the helper/convert.go package
			cc.ToBoiler = getToBoiler(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))
			cc.ToGraphQL = getToGraphQL(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))
		}

	}
	// fmt.Println("boilType for", field.Name, ":", boilType)

	cc.GraphTypeAsText = getGraphTypeAsText(graphType)
	cc.BoilerTypeAsText = getBoilerTypeAsText(boilType)

	return
}

func getToBoiler(boilType, graphType string) string {
	return "helper." + getGraphTypeAsText(graphType) + "To" + getBoilerTypeAsText(boilType)
}

func getToGraphQL(boilType, graphType string) string {
	return "helper." + getBoilerTypeAsText(boilType) + "To" + getGraphTypeAsText(graphType)
}

func getBoilerTypeAsText(boilType string) string {
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
	return strcase.ToCamel(boilType)
}

func getGraphTypeAsText(graphType string) string {
	if strings.HasPrefix(graphType, "*") {
		graphType = strings.TrimPrefix(graphType, "*")
		graphType = strcase.ToCamel(graphType)
		graphType = "Pointer" + graphType
	}
	return strcase.ToCamel(graphType)
}
