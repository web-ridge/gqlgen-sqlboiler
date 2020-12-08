package gbgen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/iancoleman/strcase"
)

type BoilerModel struct {
	Name               string
	TableName          string
	PluralName         string
	Fields             []*BoilerField
	HasPrimaryStringID bool
}

type BoilerField struct {
	Name             string
	PluralName       string
	Type             string
	IsForeignKey     bool
	IsRequired       bool
	IsArray          bool
	IsRelation       bool
	RelationshipName string
	Relationship     *BoilerModel
}

type BoilerType struct {
	Name string
	Type string
}

// parseModelsAndFieldsFromBoiler since these are like User.ID, User.Organization and we want them grouped by
// modelName and their belonging fields.
func GetBoilerModels(dir string) []*BoilerModel { //nolint:gocognit,gocyclo
	boilerTypeMap, _, boilerTypeOrder := parseBoilerFile(dir)
	boilerTypes := getSortedBoilerTypes(boilerTypeMap, boilerTypeOrder)
	tableNames := parseTableNames(dir)

	// sortedModelNames is needed to get the right order back of the models since we want the same order every time
	// this program has ran.
	var modelNames []string

	// fieldsPerModelName is needed to group the fields per model, so we can get all fields per modelName later on
	fieldsPerModelName := map[string][]*BoilerField{}
	relationsPerModelName := map[string][]*BoilerField{}

	// Anonymous function because this is used 2 times it prevents duplicated code
	// It's automatically inits an empty field array if it does not exist yet
	addFieldToMap := func(m map[string][]*BoilerField, modelName string, field *BoilerField) {
		modelNames = appendIfMissing(modelNames, modelName)
		_, ok := m[modelName]
		if !ok {
			m[modelName] = []*BoilerField{}
		}
		m[modelName] = append(m[modelName], field)
	}

	// Let's parse boilerTypes to models and fields
	for _, boiler := range boilerTypes {
		// split on . input is like model.Field e.g. -> User.ID
		splitted := strings.Split(boiler.Name, ".")
		// result in e.g. User
		modelName := splitted[0]
		// result in e.g. ID
		boilerFieldName := splitted[1]

		// handle names with lowercase e.g. userR, userL or other sqlboiler extra's
		if isFirstCharacterLowerCase(modelName) {
			// It's the relations of the model
			// let's add them so we can use them later
			if strings.HasSuffix(modelName, "R") {
				modelName = strcase.ToCamel(strings.TrimSuffix(modelName, "R"))

				isArray := strings.HasSuffix(boiler.Type, "Slice")
				boilerType := strings.TrimSuffix(boiler.Type, "Slice")

				relationField := &BoilerField{
					Name:             boilerFieldName,
					RelationshipName: strings.TrimSuffix(boilerFieldName, "ID"),
					PluralName:       pluralizer.Plural(boilerFieldName),
					Type:             boilerType,
					IsRelation:       true,
					IsRequired:       false,
					IsArray:          isArray,
				}
				addFieldToMap(relationsPerModelName, modelName, relationField)
			}

			// ignore the default handling since this field is already handled
			continue
		}

		// Ignore these since these are sqlboiler helper structs for preloading relationships
		if boilerFieldName == "L" || boilerFieldName == "R" {
			continue
		}
		isRelation := strings.HasSuffix(boilerFieldName, "ID") && boilerFieldName != "ID"

		addFieldToMap(fieldsPerModelName, modelName, &BoilerField{
			Name:             boilerFieldName,
			PluralName:       pluralizer.Plural(boilerFieldName),
			Type:             boiler.Type,
			IsRelation:       isRelation,
			IsRequired:       isRequired(boiler.Type),
			RelationshipName: strings.TrimSuffix(boilerFieldName, "ID"),
			IsForeignKey:     isRelation,
		})
	}
	sort.Strings(modelNames)

	// Let's generate the models in the same order as the sqlboiler structs were parsed
	models := make([]*BoilerModel, len(modelNames))
	for i, modelName := range modelNames {
		fields := fieldsPerModelName[modelName]
		tableName := findTableName(tableNames, modelName)

		var hasPrimaryStringID bool
		IDField := findBoilerField(fields, "ID")
		if IDField != nil && IDField.Type == "string" {
			hasPrimaryStringID = true
		}

		models[i] = &BoilerModel{
			Name:       modelName,
			TableName:  tableName,
			PluralName: pluralizer.Plural(modelName),
			Fields:     fields,

			HasPrimaryStringID: hasPrimaryStringID,
		}
	}

	// let's fill relationship models
	// We need to this after because we have pointers to relationships
	for _, model := range models {
		relationFields := relationsPerModelName[model.Name]
		for _, relationField := range relationFields {
			relationship := FindBoilerModel(models, relationField.Type)

			// try to find foreign key inside model
			foreignKey := findBoilerField(model.Fields, relationField.Name+"ID")
			if foreignKey != nil {
				foreignKey.Relationship = relationship
			} else {
				// fmt.Println("could not find foreignkey", foreignKey, model.Name, relationField.Name)
				// this is not a foreign key but a normal relationship
				relationField.Relationship = relationship
				model.Fields = append(model.Fields, relationField)
			}
		}
	}
	for _, model := range models {
		for _, field := range model.Fields {
			if field.IsRelation && field.Relationship == nil {
				log.Warn().Str("model", model.Name).Str("field", field.Name).Msg(
					"We could not find the relationship in the generated " +
						"boiler structs this could result in unexpected behavior, we marked this field as " +
						"non-relational \n")
				field.IsRelation = false
			}
		}
	}

	return models
}

func findBoilerField(fields []*BoilerField, fieldName string) *BoilerField {
	for _, m := range fields {
		if m.Name == fieldName {
			return m
		}
	}
	return nil
}

func findTableName(tableNames []string, modelName string) string {
	for _, tableName := range tableNames {
		if modelName == tableName {
			return tableName
		}
	}

	// if database name is plural
	for _, tableName := range tableNames {
		if pluralizer.Plural(modelName) == tableName {
			return tableName
		}
	}
	return modelName
}

func FindBoilerModel(models []*BoilerModel, modelName string) *BoilerModel {
	for _, m := range models {
		if m.Name == modelName {
			return m
		}
	}
	return nil
}

func isRequired(boilerType string) bool {
	if strings.HasPrefix(boilerType, "null.") || strings.HasPrefix(boilerType, "*") {
		return false
	}
	return true
}

func isFirstCharacterLowerCase(s string) bool {
	if len(s) > 0 && s[0] == strings.ToLower(s)[0] {
		return true
	}
	return false
}

// getSortedBoilerTypes orders the sqlboiler struct in an ordered slice of BoilerType
func getSortedBoilerTypes(boilerTypeMap map[string]string, boilerTypeOrder map[string]int) (
	sortedBoilerTypes []*BoilerType) {
	boilerTypeKeys := make([]string, 0, len(boilerTypeMap))
	for k := range boilerTypeMap {
		boilerTypeKeys = append(boilerTypeKeys, k)
	}

	// order same way as sqlboiler fields with one exception
	// let createdAt, updatedAt and deletedAt as last
	sort.Slice(boilerTypeKeys, func(i, b int) bool {
		aKey := boilerTypeKeys[i]
		bKey := boilerTypeKeys[b]

		aOrder := boilerTypeOrder[aKey]
		bOrder := boilerTypeOrder[bKey]

		higherOrders := []string{"createdat", "updatedat", "deletedat"}
		for i, higherOrder := range higherOrders {
			if strings.HasSuffix(strings.ToLower(aKey), higherOrder) {
				aOrder += 1000000 * i
			}
			if strings.HasSuffix(strings.ToLower(bKey), higherOrder) {
				bOrder += 10000000 * i
			}
		}

		return aOrder < bOrder
	})

	for _, modelAndField := range boilerTypeKeys {
		// fmt.Println(modelAndField)
		sortedBoilerTypes = append(sortedBoilerTypes, &BoilerType{
			Name: modelAndField,
			Type: boilerTypeMap[modelAndField],
		})
	}
	return //nolint:nakedret
}

var tableNameRegex = regexp.MustCompile(`\s*(.*[^ ])\s*string`) //nolint:gochecknoglobals

func parseTableNames(dir string) []string {
	dir, err := filepath.Abs(dir)
	errMessage := "could not open boiler table names file, this could not lead to problems if you're " +
		"using plural table names"
	if err != nil {
		log.Warn().Err(err).Msg(errMessage)
		return nil
	}
	content, err := ioutil.ReadFile(filepath.Join(dir, "boil_table_names.go"))
	if err != nil {
		log.Warn().Err(err).Msg(errMessage)
		return nil
	}
	tableNamesMatches := tableNameRegex.FindAllStringSubmatch(string(content), -1)
	tableNames := make([]string, len(tableNamesMatches))
	for i, tableNameMatch := range tableNamesMatches {
		tableNames[i] = tableNameMatch[1]
	}
	return tableNames
}

// will return StructName.key
// e.g.
// Address.ID: null.Integer
// Address.Longitude: null.String
// Address.Latitude : null.Decimal
// needed to generate the right convert code
func parseBoilerFile(dir string) (map[string]string, map[string]string, map[string]int) { //nolint:gocognit,gocyclo
	fieldsMap := make(map[string]string)
	structsMap := make(map[string]string)
	fieldsOrder := make(map[string]int)

	dir, err := filepath.Abs(dir)
	if err != nil {
		log.Err(err).Msg("parseBoilerFile filepath.Abs error")
		return fieldsMap, structsMap, fieldsOrder
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Err(err).Msg("parseBoilerFile ioutil.ReadDir error")
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
		if src, err := parser.ParseFile(fset, filename, nil, parser.ParseComments); err == nil { //nolint:nestif
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
							} // else {
							// fmt.Println("len(field.Names) == 0", field)
						//	}
						case *ast.ArrayType:

							name := field.Names[0].Name
							fmt.Println(name)
							if !isFirstCharacterLowerCase(name) {
								fmt.Println("add", name)
								k := safeTypeSpec.Name.String() + "." + name

								fieldsMap[k] = "unknown"
								fieldsOrder[k] = i
							}

						case *ast.SelectorExpr:
							//nolint:errcheck //TODO: handle errors
							t, _ := field.Type.(*ast.SelectorExpr)
							name := field.Names[0].Name

							k := safeTypeSpec.Name.String() + "." + name

							fieldsMap[k] = t.X.(*ast.Ident).Name + "." + t.Sel.Name
							fieldsOrder[k] = i

						case *ast.Ident:
							//nolint:errcheck //TODO: handle errors
							t, _ := field.Type.(*ast.Ident) // The type as a string
							typeName := t.Name
							name := field.Names[0].Name // name as a string

							k := safeTypeSpec.Name.String() + "." + name
							fieldsMap[k] = typeName
							fieldsOrder[k] = i

						default:
							fmt.Println("ignoring....", field.Names, field)
						}
						i++
					}
				}
			}
		}
	}

	// To store the keys in slice in sorted order
	// var keys []string
	// for k := range fieldsMap {
	// 	keys = append(keys, k)
	// }
	// sort.Strings(keys)

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
