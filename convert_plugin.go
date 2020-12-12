package gbgen

import (
	"fmt"
	"go/types"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/web-ridge/gqlgen-sqlboiler/v3/customization"

	"github.com/99designs/gqlgen/codegen/config"
	gqlgenTemplates "github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	"github.com/iancoleman/strcase"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vektah/gqlparser/v2/ast"
	pluralize "github.com/web-ridge/go-pluralize"
	"github.com/web-ridge/gqlgen-sqlboiler/v3/templates"
)

var (
	pathRegex  *regexp.Regexp    //nolint:gochecknoglobals
	pluralizer *pluralize.Client //nolint:gochecknoglobals
)

func init() { //nolint:gochecknoinits
	fmt.Println("               _     _____  _     _            \n              | |   |  __ \\(_)   | |           \n __      _____| |__ | |__) |_  __| | __ _  ___ \n \\ \\ /\\ / / _ \\ '_ \\|  _  /| |/ _` |/ _` |/ _ \\\n  \\ V  V /  __/ |_) | | \\ \\| | (_| | (_| |  __/\n   \\_/\\_/ \\___|_.__/|_|  \\_\\_|\\__,_|\\__, |\\___|\n                                     __/ |     \n                                    |___/   ") //nolint:lll
	fmt.Println("")
	fmt.Println("  Please help us with feedback, stars and PR's to improve this plugin.")
	fmt.Println("  If you don't have time for that, please donate if you like this project.")
	fmt.Println("  Click the sponsor button (PayPal) on https://github.com/web-ridge/gqlgen-sqlboiler")
	fmt.Println("")

	pluralizer = pluralize.NewClient()
	pathRegex = regexp.MustCompile(`src/(.*)`)

	// Default level for this example is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

type Import struct {
	Alias      string
	ImportPath string
}

type ModelBuild struct {
	Backend      Config
	Frontend     Config
	PluginConfig ConvertPluginConfig
	PackageName  string
	Interfaces   []*Interface
	Models       []*Model
	Enums        []*Enum
	Scalars      []string
}

type Interface struct {
	Description string
	Name        string
}

type Preload struct {
	Key           string
	ColumnSetting ColumnSetting
}

type Model struct { //nolint:maligned
	Name           string
	PluralName     string
	BoilerModel    *BoilerModel
	PrimaryKeyType string
	Fields         []*Field
	IsNormal       bool
	IsInput        bool
	IsCreateInput  bool
	IsUpdateInput  bool
	IsNormalInput  bool
	IsPayload      bool
	IsConnection   bool
	IsEdge         bool
	IsOrdering     bool
	IsWhere        bool
	IsFilter       bool
	IsPreloadable  bool
	PreloadArray   []Preload

	HasPrimaryStringID bool
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

type Field struct { //nolint:maligned
	Name               string
	JSONName           string
	PluralName         string
	Type               string
	TypeWithoutPointer string
	IsNumberID         bool
	IsPrimaryNumberID  bool
	IsPrimaryStringID  bool
	IsPrimaryID        bool
	IsRequired         bool
	IsPlural           bool
	ConvertConfig      ConvertConfig
	Enum               *Enum
	// relation stuff
	IsRelation bool
	// boiler relation stuff is inside this field
	BoilerField BoilerField
	// graphql relation ship can be found here
	Relationship *Model
	IsOr         bool
	IsAnd        bool

	// Some stuff
	Description  string
	OriginalType types.Type
}

type Enum struct {
	Description string
	Name        string

	Values []*EnumValue
}

type EnumValue struct {
	Description string
	Name        string
	NameLower   string
}

func NewConvertPlugin(output, backend, frontend Config, pluginConfig ConvertPluginConfig) plugin.Plugin {
	return &ConvertPlugin{
		Output:         output,
		Backend:        backend,
		Frontend:       frontend,
		PluginConfig:   pluginConfig,
		rootImportPath: getRootImportPath(),
	}
}

type ConvertPlugin struct {
	Output         Config
	Backend        Config
	Frontend       Config
	PluginConfig   ConvertPluginConfig
	rootImportPath string
}

type Config struct {
	Directory   string
	PackageName string
}

type ConvertPluginConfig struct {
	UseReflectWorkaroundForSubModelFilteringInPostgresIssue25 bool
}

var _ plugin.ConfigMutator = &ConvertPlugin{}

func (m *ConvertPlugin) Name() string {
	return "convert-generator"
}

func copyConfig(cfg config.Config) *config.Config {
	return &cfg
}

func GetModelsWithInformation(backend Config, enums []*Enum, cfg *config.Config, boilerModels []*BoilerModel) []*Model {
	// get models based on the schema and sqlboiler structs
	models := getModelsFromSchema(cfg.Schema, boilerModels)

	// Now we have all model's let enhance them with fields
	enhanceModelsWithFields(enums, cfg.Schema, cfg, models)

	// Add preload maps
	enhanceModelsWithPreloadArray(backend, models)

	// Sort in same order
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	for _, m := range models {
		cfg.Models.Add(m.Name, cfg.Model.ImportPath()+"."+gqlgenTemplates.ToGo(m.Name))
	}
	return models
}

func (m *ConvertPlugin) MutateConfig(originalCfg *config.Config) error {
	b := &ModelBuild{
		PackageName: m.Output.PackageName,
		Backend: Config{
			Directory:   path.Join(m.rootImportPath, m.Backend.Directory),
			PackageName: m.Backend.PackageName,
		},
		Frontend: Config{
			Directory:   path.Join(m.rootImportPath, m.Frontend.Directory),
			PackageName: m.Frontend.PackageName,
		},
		PluginConfig: m.PluginConfig,
	}

	cfg := copyConfig(*originalCfg)

	// log.Debug().Msg("[customization] looking for *_customized files")

	log.Debug().Msg("[convert] get boiler models")
	boilerModels := GetBoilerModels(m.Backend.Directory)

	log.Debug().Msg("[convert] get extra's from schema")
	interfaces, enums, scalars := getExtrasFromSchema(cfg.Schema)

	log.Debug().Msg("[convert] get model with information")
	models := GetModelsWithInformation(b.Backend, enums, originalCfg, boilerModels)

	b.Models = models
	b.Interfaces = interfaces
	b.Enums = enumsWithout(enums, []string{"SortDirection"})
	b.Scalars = scalars
	if len(b.Models) == 0 {
		log.Warn().Msg("no models found in graphql so skipping generation")
		return nil
	}

	// for _, model := range models {
	// 	fmt.Println(model.Name, "->", model.BoilerModel.Name)
	// 	for _, field := range model.Fields {
	// 		fmt.Println("    ", field.Name, field.Type)
	// 		fmt.Println("    ", field.BoilerField.Name, field.BoilerField.Type)
	// 	}
	// }

	filesToGenerate := []string{
		"convert.go",
		"convert_input.go",
		"filter.go",
		"preload.go",
		"sort.go",
	}

	// We get all function names from helper repository to check if any customizations are available
	// we ignore the files we generated by this plugin
	userDefinedFunctions, err := customization.GetFunctionNamesFromDir(m.Output.PackageName, filesToGenerate)
	if err != nil {
		log.Err(err).Msg("could not find user defined functions")
	}

	for _, fileName := range filesToGenerate {
		templateName := fileName + "tpl"
		log.Debug().Msg("[convert] render " + templateName)

		templateContent, err := getTemplateContent(templateName)
		if err != nil {
			log.Err(err).Msg("error when reading " + templateName)
			continue
		}

		if renderError := templates.WriteTemplateFile(
			m.Output.Directory+"/"+fileName,
			templates.Options{
				Template:             templateContent,
				PackageName:          m.Output.PackageName,
				Data:                 b,
				UserDefinedFunctions: userDefinedFunctions,
			}); renderError != nil {
			log.Err(renderError).Msg("error while rendering " + templateName)
		}
		log.Debug().Msg("[convert] rendered " + templateName)
	}

	return nil
}

//// take a string in the form github.com/package/blah.Type and split it into package and type
//func PkgAndType(name string) (string, string) {
//	parts := strings.Split(name, ".")
//	if len(parts) == 1 {
//		return "", name
//	}
//
//	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
//}

func enumsWithout(enums []*Enum, skip []string) []*Enum {
	// lol: cleanup xD
	var a []*Enum
	for _, e := range enums {
		var skipped bool
		for _, skip := range skip {
			if skip == e.Name {
				skipped = true
			}
		}
		if !skipped {
			a = append(a, e)
		}
	}
	return a
}

func getTemplateContent(filename string) (string, error) {
	// load path relative to calling source file
	_, callerFile, _, _ := runtime.Caller(1) //nolint:dogsled
	rootDir := filepath.Dir(callerFile)
	content, err := ioutil.ReadFile(path.Join(rootDir, "template_files", filename))
	if err != nil {
		return "", fmt.Errorf("could not read template file: %v", err)
	}
	return string(content), nil
}

// getFieldType check's if user has defined a
func getFieldType(binder *config.Binder, schema *ast.Schema, cfg *config.Config, field *ast.FieldDefinition) (
	types.Type, error) {
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
				types.NewTypeName(0, cfg.Model.Pkg(), gqlgenTemplates.ToGo(field.Type.Name()), nil),
				types.NewInterfaceType([]*types.Func{}, []types.Type{}),
				nil,
			)

		case ast.Enum:
			// no user defined model, must reference a generated enum
			typ = types.NewNamed(
				types.NewTypeName(0, cfg.Model.Pkg(), gqlgenTemplates.ToGo(field.Type.Name()), nil),
				nil,
				nil,
			)

		case ast.Object, ast.InputObject:
			// no user defined model, must reference a generated struct
			typ = types.NewNamed(
				types.NewTypeName(0, cfg.Model.Pkg(), gqlgenTemplates.ToGo(field.Type.Name()), nil),
				types.NewStruct(nil, nil),
				nil,
			)

		default:
			panic(fmt.Errorf("unknown ast type %s", fieldDef.Kind))
		}
	}

	return typ, err
}

func getGraphqlFieldName(cfg *config.Config, modelName string, field *ast.FieldDefinition) string {
	name := field.Name
	if nameOveride := cfg.Models[modelName].Fields[field.Name].FieldName; nameOveride != "" {
		// TODO: map overrides to sqlboiler the other way around?
		name = nameOveride
	}
	return name
}

//nolint:gocognit,gocyclo
func enhanceModelsWithFields(enums []*Enum, schema *ast.Schema, cfg *config.Config,
	models []*Model) {
	binder := cfg.NewBinder()

	// Generate the basic of the fields
	for _, m := range models {
		// Let's convert the pure ast fields to something usable for our templates
		for _, field := range m.PureFields {
			fieldDef := schema.Types[field.Type.Name()]

			// This calls some qglgen boilerType which gets the gqlgen type
			typ, err := getFieldType(binder, schema, cfg, field)
			if err != nil {
				log.Err(err).Msg("could not get field type from graphql schema")
			}
			jsonName := getGraphqlFieldName(cfg, m.Name, field)
			name := gqlgenTemplates.ToGo(jsonName)

			// just some (old) Relay clutter which is not needed anymore + we won't do anything with it
			// in our database converts.
			if strings.EqualFold(name, "clientMutationId") {
				continue
			}

			// override type struct with qqlgen code
			typ = binder.CopyModifiersFromAst(field.Type, typ)
			if isStruct(typ) && (fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject) {
				typ = types.NewPointer(typ)
			}

			// generate some booleans because these checks will be used a lot
			isRelation := fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject

			shortType := getShortType(typ.String())

			isPrimaryID := strings.EqualFold(name, "id")

			// get sqlboiler information of the field
			boilerField := findBoilerFieldOrForeignKey(m.BoilerModel.Fields, name, isRelation)
			isString := strings.Contains(strings.ToLower(boilerField.Type), "string")
			isNumberID := strings.HasSuffix(name, "ID") && !isString
			isPrimaryNumberID := isPrimaryID && !isString

			isPrimaryStringID := isPrimaryID && isString

			// enable simpler code in resolvers
			if isPrimaryStringID {
				m.HasPrimaryStringID = isPrimaryStringID
			}
			if isPrimaryNumberID || isPrimaryStringID {
				m.PrimaryKeyType = boilerField.Type
			}

			isEdges := strings.HasSuffix(m.Name, "Connection") && name == "Edges"
			isPageInfo := strings.HasSuffix(m.Name, "Connection") && name == "PageInfo"
			isSort := strings.HasSuffix(m.Name, "Ordering") && name == "Sort"
			isSortDirection := strings.HasSuffix(m.Name, "Ordering") && name == "Direction"
			isCursor := strings.HasSuffix(m.Name, "Edge") && name == "Cursor"
			isNode := strings.HasSuffix(m.Name, "Edge") && name == "Node"

			// log some warnings when fields could not be converted
			if boilerField.Type == "" {
				// TODO: add filter + where here
				switch {
				case m.IsPayload:
				case pluralizer.IsPlural(name):
				case (m.IsFilter || m.IsWhere) && (strings.EqualFold(name, "and") ||
					strings.EqualFold(name, "or") ||
					strings.EqualFold(name, "search") ||
					strings.EqualFold(name, "where")) ||
					isEdges ||
					isSort ||
					isSortDirection ||
					isPageInfo ||
					isCursor ||
					isNode:
					// ignore
				default:
					log.Warn().Str("model.field", m.Name+"."+name).Msg("boiler type not available (empty type)")
				}
			}

			if boilerField.Name == "" {
				if m.IsPayload || m.IsFilter || m.IsWhere || m.IsOrdering || m.IsEdge || isPageInfo || isEdges {
				} else {
					log.Warn().Str("model.field", m.Name+"."+name).Msg("boiler type not available")
					continue
				}
			}

			enum := findEnum(enums, shortType)

			field := &Field{
				Name:               name,
				JSONName:           jsonName,
				Type:               shortType,
				TypeWithoutPointer: strings.Replace(strings.TrimPrefix(shortType, "*"), ".", "Dot", -1),
				BoilerField:        boilerField,
				IsNumberID:         isNumberID,
				IsPrimaryID:        isPrimaryID,
				IsPrimaryNumberID:  isPrimaryNumberID,
				IsPrimaryStringID:  isPrimaryStringID,
				IsRelation:         isRelation,
				IsOr:               strings.EqualFold(name, "or"),
				IsAnd:              strings.EqualFold(name, "and"),
				IsPlural:           pluralizer.IsPlural(name),
				PluralName:         pluralizer.Plural(name),
				OriginalType:       typ,
				Description:        field.Description,
				Enum:               enum,
			}
			field.ConvertConfig = getConvertConfig(enums, m, field)
			m.Fields = append(m.Fields, field)
		}
	}

	for _, m := range models {
		for _, f := range m.Fields {
			if f.BoilerField.Relationship != nil {
				f.Relationship = findModel(models, f.BoilerField.Relationship.Name)
			}
		}
	}
}

var ignoreTypePrefixes = []string{"graphql_models", "models", "boilergql"} //nolint:gochecknoglobals

func getShortType(longType string) string {
	// longType e.g = gitlab.com/decicify/app/backend/graphql_models.FlowWhere
	splittedBySlash := strings.Split(longType, "/")
	// gitlab.com, decicify, app, backend, graphql_models.FlowWhere

	lastPart := splittedBySlash[len(splittedBySlash)-1]
	isPointer := strings.HasPrefix(longType, "*")
	isStructInPackage := strings.Count(lastPart, ".") > 0

	if isStructInPackage {
		// if packages are deeper they don't have pointers but *time.Time will since it's not deep
		returnType := strings.TrimPrefix(lastPart, "*")
		for _, ignoreType := range ignoreTypePrefixes {
			fullIgnoreType := ignoreType + "."
			returnType = strings.TrimPrefix(returnType, fullIgnoreType)
		}

		if isPointer {
			return "*" + returnType
		}
		return returnType
	}

	return longType
}

func findModel(models []*Model, search string) *Model {
	for _, m := range models {
		if m.Name == search {
			return m
		}
	}
	return nil
}

//func findField(fields []*Field, search string) *Field {
//	for _, f := range fields {
//		if f.Name == search {
//			return f
//		}
//	}
//	return nil
//}

func findBoilerFieldOrForeignKey(fields []*BoilerField, golangGraphQLName string, isRelation bool) BoilerField {
	// get database friendly struct for this model
	for _, field := range fields {
		if isRelation {
			// If it a relation check to see if a foreign key is available
			if strings.EqualFold(field.Name, golangGraphQLName+"ID") {
				return *field
			}
		}
		if strings.EqualFold(field.Name, golangGraphQLName) {
			return *field
		}
	}
	return BoilerField{}
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
					NameLower:   strcase.ToLowerCamel(strings.ToLower(v.Name)),
					Description: v.Description,
				})
			}
			if strings.HasPrefix(it.Name, "_") {
				continue
			}
			enums = append(enums, it)
		case ast.Scalar:
			scalars = append(scalars, schemaType.Name)
		}
	}
	return
}

func getModelsFromSchema(schema *ast.Schema, boilerModels []*BoilerModel) (models []*Model) { //nolint:gocognit,gocyclo
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
				boilerModel := FindBoilerModel(boilerModels, getBaseModelFromName(modelName))

				isInput := doesEndWith(modelName, "Input")
				isCreateInput := doesEndWith(modelName, "CreateInput")
				isUpdateInput := doesEndWith(modelName, "UpdateInput")
				isFilter := doesEndWith(modelName, "Filter")
				isWhere := doesEndWith(modelName, "Where")
				isPayload := doesEndWith(modelName, "Payload")
				isEdge := doesEndWith(modelName, "Edge")
				isConnection := doesEndWith(modelName, "Connection")
				isPageInfo := modelName == "PageInfo"
				isOrdering := doesEndWith(modelName, "Ordering")

				var isPagination bool
				paginationTriggers := []string{
					"ConnectionBackwardPagination",
					"ConnectionPagination",
					"ConnectionForwardPagination",
				}
				for _, p := range paginationTriggers {
					if modelName == p {
						isPagination = true
					}
				}

				// if no boiler model is found
				if boilerModel == nil || boilerModel.Name == "" {
					if isInput || isWhere || isFilter || isPayload || isPageInfo || isPagination {
						// silent continue
						continue
					}
					log.Warn().Str("model", modelName).Msg("skipped because no database model found")
					continue
				}

				isNormalInput := isInput && !isCreateInput && !isUpdateInput
				isNormal := !isInput && !isWhere && !isFilter && !isPayload && !isEdge && !isConnection && !isOrdering
				m := &Model{
					Name:          modelName,
					Description:   schemaType.Description,
					PluralName:    pluralizer.Plural(modelName),
					BoilerModel:   boilerModel,
					IsInput:       isInput,
					IsFilter:      isFilter,
					IsWhere:       isWhere,
					IsUpdateInput: isUpdateInput,
					IsCreateInput: isCreateInput,
					IsNormalInput: isNormalInput,
					IsConnection:  isConnection,
					IsEdge:        isEdge,
					IsPayload:     isPayload,
					IsOrdering:    isOrdering,
					IsNormal:      isNormal,
					IsPreloadable: isNormal,
				}

				for _, implementor := range schema.GetImplements(schemaType) {
					m.Implements = append(m.Implements, implementor.Name)
				}

				m.PureFields = append(m.PureFields, schemaType.Fields...)
				models = append(models, m)
			}
		}
	}
	return //nolint:nakedret
}

func doesEndWith(s string, suffix string) bool {
	return strings.HasSuffix(s, suffix) && s != suffix
}

func getPreloadMapForModel(backend Config, model *Model) map[string]ColumnSetting {
	preloadMap := map[string]ColumnSetting{}
	for _, field := range model.Fields {
		// only relations are preloadable
		if !field.IsRelation {
			continue
		}
		// var key string
		// if field.IsPlural {
		key := field.JSONName
		// } else {
		// 	key = field.PluralName
		// }
		name := fmt.Sprintf("%v.%vRels.%v", backend.PackageName, model.Name, foreignKeyToRel(field.BoilerField.Name))
		setting := ColumnSetting{
			Name:                  name,
			IDAvailable:           !field.IsPlural,
			RelationshipModelName: field.BoilerField.Relationship.TableName,
		}

		preloadMap[key] = setting
	}
	return preloadMap
}

func enhanceModelsWithPreloadArray(backend Config, models []*Model) {
	// first adding basic first level relations
	for _, model := range models {
		if !model.IsPreloadable {
			continue
		}

		modelPreloadMap := getPreloadMapForModel(backend, model)

		sortedPreloadKeys := make([]string, 0, len(modelPreloadMap))
		for k := range modelPreloadMap {
			sortedPreloadKeys = append(sortedPreloadKeys, k)
		}
		sort.Strings(sortedPreloadKeys)

		model.PreloadArray = make([]Preload, len(sortedPreloadKeys))
		for i, k := range sortedPreloadKeys {
			columnSetting := modelPreloadMap[k]
			model.PreloadArray[i] = Preload{
				Key:           k,
				ColumnSetting: columnSetting,
			}
		}
	}
}

// The relationship is defined in the normal model but not in the input, where etc structs
// So just find the normal model and get the relationship type :)
func getBaseModelFromName(v string) string {
	v = safeTrim(v, "CreateInput")
	v = safeTrim(v, "UpdateInput")
	v = safeTrim(v, "Input")
	v = safeTrim(v, "Payload")
	v = safeTrim(v, "Where")
	v = safeTrim(v, "Filter")
	v = safeTrim(v, "Ordering")
	v = safeTrim(v, "Edge")
	v = safeTrim(v, "Connection")

	return v
}

func safeTrim(v string, trimSuffix string) string {
	// let user still choose Payload as model names
	// not recommended but could be done theoretically :-)
	if v != trimSuffix {
		v = strings.TrimSuffix(v, trimSuffix)
	}
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

func findEnum(enums []*Enum, graphType string) *Enum {
	for _, enum := range enums {
		if enum.Name == graphType {
			return enum
		}
	}
	return nil
}

func getConvertConfig(enums []*Enum, model *Model, field *Field) (cc ConvertConfig) { //nolint:gocognit,gocyclo
	graphType := field.Type
	boilType := field.BoilerField.Type

	enum := findEnum(enums, field.TypeWithoutPointer)
	if enum != nil { //nolint:nestif
		cc.IsCustom = true
		cc.ToBoiler = strings.TrimPrefix(
			getToBoiler(
				getBoilerTypeAsText(boilType),
				getGraphTypeAsText(graphType),
			), "boilergql.")

		cc.ToGraphQL = strings.TrimPrefix(
			getToGraphQL(
				getBoilerTypeAsText(boilType),
				getGraphTypeAsText(graphType),
			), "boilergql.")
	} else if graphType != boilType {
		cc.IsCustom = true
		if field.IsPrimaryID || field.IsNumberID && field.BoilerField.IsRelation {
			cc.ToGraphQL = "VALUE"
			cc.ToBoiler = "VALUE"

			// first unpointer json type if is pointer
			if strings.HasPrefix(graphType, "*") {
				cc.ToBoiler = "boilergql.PointerStringToString(VALUE)"
			}

			goToUint := getBoilerTypeAsText(boilType) + "ToUint"
			if goToUint == "IntToUint" {
				cc.ToGraphQL = "uint(VALUE)"
			} else if goToUint != "UintToUint" {
				cc.ToGraphQL = "boilergql." + goToUint + "(VALUE)"
			}

			if field.IsPrimaryID {
				cc.ToGraphQL = model.Name + "IDToGraphQL(" + cc.ToGraphQL + ")"
			} else if field.IsNumberID {
				cc.ToGraphQL = field.BoilerField.Relationship.Name + "IDToGraphQL(" + cc.ToGraphQL + ")"
			}

			isInt := strings.HasPrefix(strings.ToLower(boilType), "int") && !strings.HasPrefix(strings.ToLower(boilType), "uint")

			if strings.HasPrefix(boilType, "null") {
				cc.ToBoiler = fmt.Sprintf("boilergql.IDToNullBoiler(%v)", cc.ToBoiler)
				if isInt {
					cc.ToBoiler = fmt.Sprintf("boilergql.NullUintToNullInt(%v)", cc.ToBoiler)
				}
			} else {
				cc.ToBoiler = fmt.Sprintf("boilergql.IDToBoiler(%v)", cc.ToBoiler)
				if isInt {
					cc.ToBoiler = fmt.Sprintf("int(%v)", cc.ToBoiler)
				}
			}

			cc.ToGraphQL = strings.Replace(cc.ToGraphQL, "VALUE", "m."+field.BoilerField.Name, -1)
			cc.ToBoiler = strings.Replace(cc.ToBoiler, "VALUE", "m."+field.Name, -1)
		} else {
			// Make these go-friendly for the helper/convert_plugin.go package
			cc.ToBoiler = getToBoiler(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))
			cc.ToGraphQL = getToGraphQL(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))
		}
	}
	// fmt.Println("boilType for", field.Name, ":", boilType)

	cc.GraphTypeAsText = getGraphTypeAsText(graphType)
	cc.BoilerTypeAsText = getBoilerTypeAsText(boilType)

	return //nolint:nakedret
}

func getToBoiler(boilType, graphType string) string {
	return "boilergql." + getGraphTypeAsText(graphType) + "To" + getBoilerTypeAsText(boilType)
}

func getToGraphQL(boilType, graphType string) string {
	return "boilergql." + getBoilerTypeAsText(boilType) + "To" + getGraphTypeAsText(graphType)
}

func getBoilerTypeAsText(boilType string) string {
	// backward compatible missed Dot
	if strings.HasPrefix(boilType, "types.") {
		boilType = strings.TrimPrefix(boilType, "types.")
		boilType = strcase.ToCamel(boilType)
		boilType = "Types" + boilType
	}

	// if strings.HasPrefix(boilType, "null.") {
	// 	boilType = strings.TrimPrefix(boilType, "null.")
	// 	boilType = strcase.ToCamel(boilType)
	// 	boilType = "NullDot" + boilType
	// }
	boilType = strings.Replace(boilType, ".", "Dot", -1)

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
