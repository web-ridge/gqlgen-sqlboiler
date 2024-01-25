package cache

import (
	"fmt"
	"go/types"
	"sort"
	"strings"
	"unicode"

	"github.com/web-ridge/gqlgen-sqlboiler/v3/structs"

	"github.com/iancoleman/strcase"
	"github.com/volatiletech/strmangle"

	"github.com/99designs/gqlgen/codegen/config"
	gqlgenTemplates "github.com/99designs/gqlgen/codegen/templates"
	"github.com/rs/zerolog/log"
	"github.com/vektah/gqlparser/v2/ast"
)

type BoilerCache struct {
	BoilerModels []*structs.BoilerModel
	BoilerEnums  []*structs.BoilerEnum
}

func InitializeBoilerCache(backend structs.Config) *BoilerCache {
	log.Debug().Msg("[boiler-cache] building cache")
	boilerModels, boilerEnums := GetBoilerModels(backend.Directory)
	log.Debug().Msg("[boiler-cache] built cache!")
	return &BoilerCache{
		BoilerModels: boilerModels,
		BoilerEnums:  boilerEnums,
	}
}

type ModelCache struct {
	Models     []*structs.Model
	Interfaces []*structs.Interface
	Enums      []*structs.Enum
	Backend    structs.Config
	Frontend   structs.Config
	Output     structs.Config
	Scalars    []string
}

func copyConfig(cfg config.Config) *config.Config {
	return &cfg
}

func InitializeModelCache(config *config.Config, boilerCache *BoilerCache, output structs.Config, backend structs.Config, frontend structs.Config) *ModelCache {
	//config := copyConfig(*originalConfig)
	//config.ReloadAllPackages()
	//if err := config.Init(); err != nil {
	//	log.Err(err).Msg("failed to init config")
	//}
	//config := *originalConfig

	log.Debug().Msg("[model-cache] get structs")
	baseModels := getModelsFromSchema(config.Schema, boilerCache.BoilerModels)

	log.Debug().Msg("[model-cache] get extra's from schema")
	interfaces, enums, scalars := getExtrasFromSchema(config.Schema, boilerCache.BoilerEnums, baseModels)

	log.Debug().Msg("[model-cache] enhance structs with information")
	models := EnhanceModelsWithInformation(backend, enums, config, boilerCache.BoilerModels, baseModels, []string{frontend.PackageName, backend.PackageName, "boilergql"})
	log.Debug().Msg("[model-cache] built cache!")

	return &ModelCache{
		Models:     models,
		Output:     output,
		Backend:    backend,
		Frontend:   frontend,
		Interfaces: interfaces,
		Enums:      enumsWithout(enums, []string{"SortDirection", "Sort"}),
		Scalars:    scalars,
	}
}

func EnhanceModelsWithInformation(
	backend structs.Config,
	enums []*structs.Enum,
	cfg *config.Config,
	boilerModels []*structs.BoilerModel,
	models []*structs.Model,
	ignoreTypePrefixes []string) []*structs.Model {
	// always sort enums the same way to prevent merge conflicts in generated code
	sort.Slice(enums, func(i, j int) bool {
		return enums[i].Name < enums[j].Name
	})

	// Now we have all model's let enhance them with fields
	enhanceModelsWithFields(enums, cfg.Schema, cfg, models, ignoreTypePrefixes)

	// Add preload maps
	enhanceModelsWithPreloadArray(backend, models)

	// Sort in same order
	sort.Slice(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	for _, m := range models {
		cfg.Models.Add(m.Name, cfg.Model.ImportPath()+"."+gqlgenTemplates.ToGo(m.Name))
	}
	return models
}

//nolint:gocognit,gocyclo
func enhanceModelsWithFields(enums []*structs.Enum, schema *ast.Schema, cfg *config.Config,
	models []*structs.Model, ignoreTypePrefixes []string) {
	binder := cfg.NewBinder()

	// Generate the basic of the fields
	for _, m := range models {
		if !m.HasBoilerModel {
			continue
		}
		// Let's convert the pure ast fields to something usable for our templates
		for _, field := range m.PureFields {
			fieldDef := schema.Types[field.Type.Name()]

			// This calls some qglgen boilerType which gets the gqlgen type
			typ, err := getAstFieldType(binder, schema, cfg, field)
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
			isObject := fieldDef.Kind == ast.Object || fieldDef.Kind == ast.InputObject

			shortType := getShortType(typ.String(), ignoreTypePrefixes)

			isPrimaryID := strings.EqualFold(name, "id")

			// get sqlboiler information of the field
			boilerField := findBoilerFieldOrForeignKey(m.BoilerModel, name, isObject)
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
				skipWarningInFilter :=
					strings.EqualFold(name, "and") ||
						strings.EqualFold(name, "or") ||
						strings.EqualFold(name, "search") ||
						strings.EqualFold(name, "where") ||
						strings.EqualFold(name, "withDeleted")

				switch {
				case m.IsPayload:
				case IsPlural(name):
				case ((m.IsFilter || m.IsWhere) && skipWarningInFilter) ||
					isEdges ||
					isSort ||
					isSortDirection ||
					isPageInfo ||
					isCursor ||
					isNode:
					// ignore
				default:
					log.Warn().Str("field", m.Name+"."+name).Msg("no database mapping")
				}
			}

			if boilerField.Name == "" {
				if m.IsPayload || m.IsFilter || m.IsWhere || m.IsOrdering || m.IsEdge || isPageInfo || isEdges {
				} else {
					log.Warn().Str("field", m.Name+"."+name).Msg("no database mapping")
					continue
				}
			}

			enum := findEnum(enums, shortType)
			field := &structs.Field{
				Name:               name,
				JSONName:           jsonName,
				Type:               shortType,
				TypeWithoutPointer: strings.Replace(strings.TrimPrefix(shortType, "*"), ".", "Dot", -1),
				BoilerField:        boilerField,
				IsNumberID:         isNumberID,
				IsPrimaryID:        isPrimaryID,
				IsPrimaryNumberID:  isPrimaryNumberID,
				IsPrimaryStringID:  isPrimaryStringID,
				IsRelation:         boilerField.IsRelation,
				IsRelationAndNotForeignKey: boilerField.IsRelation &&
					!strings.HasSuffix(strings.ToLower(name), "id"),
				IsObject:      isObject,
				IsOr:          strings.EqualFold(name, "or"),
				IsAnd:         strings.EqualFold(name, "and"),
				IsWithDeleted: strings.EqualFold(name, "withDeleted"),
				IsPlural:      IsPlural(name),
				PluralName:    Plural(name),
				OriginalType:  typ,
				Description:   field.Description,
				Enum:          enum,
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

func enumsWithout(enums []*structs.Enum, skip []string) []*structs.Enum {
	// lol: cleanup xD
	var a []*structs.Enum
	for _, e := range enums {
		var skipped bool
		for _, skip := range skip {
			if strings.HasSuffix(e.Name, skip) {
				skipped = true
			}
		}
		if !skipped {
			a = append(a, e)
		}
	}
	return a
}

// getAstFieldType check's if user has defined a
func getAstFieldType(binder *config.Binder, schema *ast.Schema, cfg *config.Config, field *ast.FieldDefinition) (
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

// TaskBlockedBies -> TaskBlockedBy
// People -> Person
func Singular(s string) string {
	singular := strmangle.Singular(strcase.ToSnake(s))

	singularTitle := strmangle.TitleCase(singular)
	if IsFirstCharacterLowerCase(s) {
		a := []rune(singularTitle)
		a[0] = unicode.ToLower(a[0])
		return string(a)
	}
	return singularTitle
}

// TaskBlockedBy -> TaskBlockedBies
// Person -> Persons
// Person -> People
func Plural(s string) string {
	plural := strmangle.Plural(strcase.ToSnake(s))

	pluralTitle := strmangle.TitleCase(plural)
	if IsFirstCharacterLowerCase(s) {
		a := []rune(pluralTitle)
		a[0] = unicode.ToLower(a[0])
		return string(a)
	}
	return pluralTitle
}

func IsFirstCharacterLowerCase(s string) bool {
	if len(s) > 0 && s[0] == strings.ToLower(s)[0] {
		return true
	}
	return false
}

func IsPlural(s string) bool {
	return s == Plural(s)
}

func IsSingular(s string) bool {
	return s == Singular(s)
}

func getShortType(longType string, ignoreTypePrefixes []string) string {
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

func findModel(models []*structs.Model, search string) *structs.Model {
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

func findBoilerFieldOrForeignKey(boilerModel *structs.BoilerModel, golangGraphQLName string, isObject bool) structs.BoilerField {
	if boilerModel == nil {
		return structs.BoilerField{}
	}

	// get database friendly struct for this model
	for _, field := range boilerModel.Fields {
		if isObject {
			// If it a relation check to see if a foreign key is available
			if strings.EqualFold(field.Name, golangGraphQLName+"ID") {
				return *field
			}
		}
		if strings.EqualFold(field.Name, golangGraphQLName) {
			return *field
		}
	}
	return structs.BoilerField{}
}

func getExtrasFromSchema(schema *ast.Schema, boilerEnums []*structs.BoilerEnum, models []*structs.Model) (interfaces []*structs.Interface, enums []*structs.Enum, scalars []string) {
	for _, schemaType := range schema.Types {
		switch schemaType.Kind {
		case ast.Interface, ast.Union:
			interfaces = append(interfaces, &structs.Interface{
				Description: schemaType.Description,
				Name:        schemaType.Name,
			})
		case ast.Enum:
			boilerEnum := findBoilerEnum(boilerEnums, schemaType.Name)
			it := &structs.Enum{
				Name:          schemaType.Name,
				PluralName:    Plural(schemaType.Name),
				Description:   schemaType.Description,
				HasBoilerEnum: boilerEnum != nil,
				BoilerEnum:    boilerEnum,
				HasFilter:     findModel(models, schemaType.Name+"Filter") != nil,
			}
			for _, v := range schemaType.EnumValues {
				it.Values = append(it.Values, &structs.EnumValue{
					Name:            v.Name,
					NameLower:       strcase.ToLowerCamel(strings.ToLower(v.Name)),
					Description:     v.Description,
					BoilerEnumValue: findBoilerEnumValue(boilerEnum, v.Name),
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

func getModelsFromSchema(schema *ast.Schema, boilerModels []*structs.BoilerModel) (models []*structs.Model) { //nolint:gocognit,gocyclo
	for _, schemaType := range schema.Types {
		// skip boiler plate from ggqlgen, we only want the structs
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

				hasEmptyBoilerModel := boilerModel == nil || boilerModel.Name == ""
				// TODO: make this cleaner and support custom structs
				if !isFilter {
					if hasEmptyBoilerModel {
						if isInput || isWhere || isPayload || isPageInfo || isPagination {
							// silent continue
							continue
						}
						log.Debug().Str("model", modelName).Msg("skipped because no database model found")
						continue
					}
				}

				isNormalInput := isInput && !isCreateInput && !isUpdateInput
				isNormal := !isInput && !isWhere && !isFilter && !isPayload && !isEdge && !isConnection && !isOrdering

				hasBoilerModel := !hasEmptyBoilerModel
				hasDeletedAt := hasBoilerModel && boilerModel.HasDeletedAt
				tableNameResolverName := "TableNames"
				if hasBoilerModel && boilerModel.IsView {
					tableNameResolverName = "ViewNames"
				}
				m := &structs.Model{
					Name:                  modelName,
					JSONName:              strcase.ToCamel(modelName),
					Description:           schemaType.Description,
					PluralName:            Plural(modelName),
					BoilerModel:           boilerModel,
					HasBoilerModel:        hasBoilerModel,
					IsInput:               isInput,
					IsFilter:              isFilter,
					IsWhere:               isWhere,
					IsUpdateInput:         isUpdateInput,
					IsCreateInput:         isCreateInput,
					IsNormalInput:         isNormalInput,
					IsConnection:          isConnection,
					IsEdge:                isEdge,
					IsPayload:             isPayload,
					IsOrdering:            isOrdering,
					IsNormal:              isNormal,
					IsPreloadable:         isNormal,
					HasDeletedAt:          hasDeletedAt,
					TableNameResolverName: tableNameResolverName,
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

func getPreloadMapForModel(backend structs.Config, model *structs.Model) map[string]structs.ColumnSetting {
	preloadMap := map[string]structs.ColumnSetting{}
	for _, field := range model.Fields {
		// only relations are preloadable
		if !field.IsObject || !field.BoilerField.IsRelation {
			continue
		}
		// var key string
		// if field.IsPlural {
		key := field.JSONName
		// } else {
		// 	key = field.PluralName
		// }
		name := fmt.Sprintf("%v.%vRels.%v", backend.PackageName, model.Name, foreignKeyToRel(field.BoilerField.Name))
		setting := structs.ColumnSetting{
			Name:                  name,
			IDAvailable:           !field.IsPlural,
			RelationshipModelName: field.BoilerField.Relationship.TableName,
		}

		preloadMap[key] = setting
	}
	return preloadMap
}

func enhanceModelsWithPreloadArray(backend structs.Config, models []*structs.Model) {
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

		model.PreloadArray = make([]structs.Preload, len(sortedPreloadKeys))
		for i, k := range sortedPreloadKeys {
			columnSetting := modelPreloadMap[k]
			model.PreloadArray[i] = structs.Preload{
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
	return strings.TrimSuffix(v, "ID")
}

func isStruct(t types.Type) bool {
	_, is := t.Underlying().(*types.Struct)
	return is
}

func findBoilerEnum(enums []*structs.BoilerEnum, graphType string) *structs.BoilerEnum {
	for _, enum := range enums {
		if enum.Name == graphType {
			return enum
		}
	}
	return nil
}

func findBoilerEnumValue(enum *structs.BoilerEnum, name string) *structs.BoilerEnumValue {
	if enum != nil {
		for _, v := range enum.Values {
			boilerName := strings.TrimPrefix(v.Name, enum.Name)
			frontendName := strings.Replace(name, "_", "", -1)
			if strings.EqualFold(boilerName, frontendName) {
				return v
			}
		}
		log.Error().Str(enum.Name, name).Msg("could not find sqlboiler enum value")
	}

	return nil
}

func findEnum(enums []*structs.Enum, graphType string) *structs.Enum {
	for _, enum := range enums {
		if enum.Name == graphType {
			return enum
		}
	}
	return nil
}

func getConvertConfig(enums []*structs.Enum, model *structs.Model, field *structs.Field) (cc structs.ConvertConfig) { //nolint:gocognit,gocyclo
	graphType := field.Type
	boilType := field.BoilerField.Type

	enum := findEnum(enums, field.TypeWithoutPointer)
	if enum != nil { //nolint:nestif
		cc.IsCustom = true
		cc.ToBoiler = strings.TrimPrefix(
			getToBoiler(
				getBoilerTypeAsText(boilType),
				getGraphTypeAsText(graphType),
			), boilerPackage)

		cc.ToGraphQL = strings.TrimPrefix(
			getToGraphQL(
				getBoilerTypeAsText(boilType),
				getGraphTypeAsText(graphType),
			), boilerPackage)
	} else if graphType != boilType {
		cc.IsCustom = true
		if field.IsPrimaryID || field.IsNumberID && field.BoilerField.IsRelation {
			// TODO: more dynamic and universal
			cc.ToGraphQL = "VALUE"
			cc.ToBoiler = "VALUE"

			// first unpointer json type if is pointer
			if strings.HasPrefix(graphType, "*") {
				cc.ToBoiler = boilerPackage + "PointerStringToString(VALUE)"
			}

			goToUint := getBoilerTypeAsText(boilType) + "ToUint"
			if goToUint == "IntToUint" {
				cc.ToGraphQL = "uint(VALUE)"
			} else if goToUint != "UintToUint" {
				cc.ToGraphQL = boilerPackage + goToUint + "(VALUE)"
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
			// Make these go-friendly for the helper/plugin_convert.go package
			cc.ToBoiler = getToBoiler(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))
			cc.ToGraphQL = getToGraphQL(getBoilerTypeAsText(boilType), getGraphTypeAsText(graphType))
		}
	}

	// fmt.Println("boilType for", field.Name, ":", boilType)

	// JSON let the user convert how it looks in a custom file
	if strings.Contains(boilType, "JSON") {
		cc.ToBoiler = strings.TrimPrefix(cc.ToBoiler, boilerPackage)
		cc.ToGraphQL = strings.TrimPrefix(cc.ToGraphQL, boilerPackage)
	}

	cc.GraphTypeAsText = getGraphTypeAsText(graphType)
	cc.BoilerTypeAsText = getBoilerTypeAsText(boilType)

	return //nolint:nakedret
}

const boilerPackage = "boilergql."

func getToBoiler(boilType, graphType string) string {
	return boilerPackage + getGraphTypeAsText(graphType) + "To" + getBoilerTypeAsText(boilType)
}

func getToGraphQL(boilType, graphType string) string {
	return boilerPackage + getBoilerTypeAsText(boilType) + "To" + getGraphTypeAsText(graphType)
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

func FindBoilerModel(models []*structs.BoilerModel, modelName string) *structs.BoilerModel {
	for _, m := range models {
		if strings.ToLower(m.Name) == strings.ToLower(modelName) {
			return m
		}
	}
	return nil
}
