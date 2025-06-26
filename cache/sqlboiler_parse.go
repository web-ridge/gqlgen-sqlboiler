package cache

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
	"unicode"

	"github.com/web-ridge/gqlgen-sqlboiler/v3/structs"

	"github.com/iancoleman/strcase"
	"github.com/rs/zerolog/log"
)

// parseModelsAndFieldsFromBoiler since these are like User.ID, User.Organization and we want them grouped by
// modelName and their belonging fields.
func GetBoilerModels(dir string) ([]*structs.BoilerModel, []*structs.BoilerEnum) { //nolint:gocognit,gocyclo
	boilerTypeMap, _, boilerTypeOrder := parseBoilerFile(dir)
	boilerTypes := getSortedBoilerTypes(boilerTypeMap, boilerTypeOrder)
	tableNames := parseTableNames(dir)
	viewNames := parseViews(dir)
	allTableNames := append(tableNames, viewNames...)
	enums := parseEnums(dir, allTableNames)

	// sortedModelNames is needed to get the right order back of the structs since we want the same order every time
	// this program has ran.
	var modelNames []string

	// fieldsPerModelName is needed to group the fields per model, so we can get all fields per modelName later on
	fieldsPerModelName := map[string][]*structs.BoilerField{}
	relationsPerModelName := map[string][]*structs.BoilerField{}

	// Anonymous function because this is used 2 times it prevents duplicated code
	// It's automatically init an empty field array if it does not exist yet
	addFieldToMap := func(m map[string][]*structs.BoilerField, modelName string, field *structs.BoilerField) {
		modelNames = AppendIfMissing(modelNames, modelName)
		_, ok := m[modelName]
		if !ok {
			m[modelName] = []*structs.BoilerField{}
		}
		m[modelName] = append(m[modelName], field)
	}

	// Let's parse boilerTypes to structs and fields
	for _, boiler := range boilerTypes {
		// split on . input is like model.Field e.g. -> User.ID
		splitted := strings.Split(boiler.Name, ".")
		// result in e.g. User
		modelName := splitted[0]

		// result in e.g. ID
		boilerFieldName := splitted[1]
		somethingFcm := strings.Contains(strings.ToLower(modelName), "fcm")

		// handle names with lowercase e.g. userR, userL or other sqlboiler extra's
		if IsFirstCharacterLowerCase(modelName) {
			// It's the relations of the model
			// let's add them so we can use them later
			if strings.HasSuffix(modelName, "R") {
				modelNameBefore := strings.TrimSuffix(modelName, "R")
				modelName = strings.ToUpper(string(modelNameBefore[0])) + modelNameBefore[1:]
				isArray := strings.HasSuffix(boiler.Type, "Slice")
				boilerType := strings.TrimSuffix(boiler.Type, "Slice")

				if somethingFcm {
					fmt.Println("boilerType", boilerType)
					fmt.Println("boilerFieldName", boilerFieldName)
				}

				relationField := &structs.BoilerField{
					Name:             boilerFieldName,
					RelationshipName: strings.TrimSuffix(boilerFieldName, "ID"),
					PluralName:       Plural(boilerFieldName),
					Type:             boilerType,
					IsRelation:       true,
					IsRequired:       false,
					IsArray:          isArray,
					InTable:          false,
					InTableNotID:     false,
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
		isID := boilerFieldName == "ID"
		isRelation := strings.HasSuffix(boilerFieldName, "ID") && !isID

		addFieldToMap(fieldsPerModelName, modelName, &structs.BoilerField{
			Name:             boilerFieldName,
			PluralName:       Plural(boilerFieldName),
			Type:             boiler.Type,
			IsRelation:       isRelation,
			IsRequired:       isRequired(boiler.Type),
			RelationshipName: strings.TrimSuffix(boilerFieldName, "ID"),
			IsForeignKey:     isRelation,
			InTable:          true,
			InTableNotID:     !isID,
		})
	}
	sort.Strings(modelNames)

	// Let's generate the structs in the same order as the sqlboiler structs were parsed
	models := make([]*structs.BoilerModel, len(modelNames))
	for i, modelName := range modelNames {
		fields := fieldsPerModelName[modelName]
		tableName := findTableName(tableNames, modelName)

		var hasPrimaryStringID bool
		IDField := findBoilerField(fields, "ID")
		if IDField != nil && IDField.Type == "string" {
			hasPrimaryStringID = true
		}

		var hasDeletedAt bool
		deletedAtField := findBoilerField(fields, "DeletedAt")
		if deletedAtField != nil {
			hasDeletedAt = true
		}

		models[i] = &structs.BoilerModel{
			Name:               modelName,
			TableName:          tableName,
			PluralName:         Plural(modelName),
			Fields:             fields,
			Enums:              filterEnumsByModelName(enums, modelName),
			HasPrimaryStringID: hasPrimaryStringID,
			HasDeletedAt:       hasDeletedAt,
			IsView:             SliceContains(viewNames, modelName),
		}
	}

	// let's fill relationship structs
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
			enumForField := getEnumByModelNameAndFieldName(enums, model.Name, field.Name)
			// fmt.Println("enumForField", field.Name, enumForField)
			if enumForField != nil {
				field.IsEnum = true
				field.IsRelation = false
				field.Enum = *enumForField
			}

			if field.IsRelation && field.Relationship == nil {
				//log.Debug().Str("model", model.Name).Str("field", field.Name).Msg(
				//	"We could not find the relationship in the generated " +
				//		"boiler structs this could result in unexpected behavior, we marked this field as " +
				//		"non-relational \n")
				field.IsRelation = false
			}

		}
	}

	return models, enums
}

func getEnumByModelNameAndFieldName(enums []*structs.BoilerEnum, modelName string, fieldName string) *structs.BoilerEnum {
	for _, e := range enums {
		// fmt.Println("        ", e.ModelName, modelName)
		// fmt.Println("        ", e.ModelFieldKey, fieldName)
		if e.ModelName == modelName && e.ModelFieldKey == fieldName {
			return e
		}
	}
	return nil
}

func filterEnumsByModelName(enums []*structs.BoilerEnum, modelName string) []*structs.BoilerEnum {
	var a []*structs.BoilerEnum
	for _, e := range enums {
		if e.ModelName == modelName {
			a = append(a, e)
		}
	}
	return a
}

func findBoilerField(fields []*structs.BoilerField, fieldName string) *structs.BoilerField {
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
		if Plural(modelName) == tableName {
			return tableName
		}
	}
	return modelName
}

func isRequired(boilerType string) bool {
	if strings.HasPrefix(boilerType, "null.") ||
		strings.HasPrefix(boilerType, "types.Null") ||
		strings.HasPrefix(boilerType, "*") {
		return false
	}
	return true
}

// getSortedBoilerTypes orders the sqlboiler struct in an ordered slice of BoilerType
func getSortedBoilerTypes(boilerTypeMap map[string]string, boilerTypeOrder map[string]int) (
	sortedBoilerTypes []*structs.BoilerType) {
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
				aOrder += 100 + 100*i
			}
			if strings.HasSuffix(strings.ToLower(bKey), higherOrder) {
				bOrder += 100 + 100*i
			}
		}

		return aOrder < bOrder
	})

	for _, modelAndField := range boilerTypeKeys {
		// fmt.Println(modelAndField)
		sortedBoilerTypes = append(sortedBoilerTypes, &structs.BoilerType{
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

func parseViews(dir string) []string {
	dir, err := filepath.Abs(dir)
	errMessage := "could not open boiler table names file, this could not lead to problems if you're " +
		"using plural table names"
	if err != nil {
		log.Warn().Err(err).Msg(errMessage)
		return nil
	}
	content, err := ioutil.ReadFile(filepath.Join(dir, "boil_view_names.go"))
	if err != nil {
		log.Warn().Err(err).Msg(errMessage)
		return nil
	}
	viewNamesMatches := tableNameRegex.FindAllStringSubmatch(string(content), -1)
	viewNames := make([]string, len(viewNamesMatches))
	for i, tableNameMatch := range viewNamesMatches {
		viewNames[i] = tableNameMatch[1]
	}
	return viewNames
}

var (
	enumRegex       = regexp.MustCompile(`// Enum values for (.*)\nconst\s\(\n(:?(.|\n)*?)\n\)`) //nolint:gochecknoglobals
	enumValuesRegex = regexp.MustCompile(`\s(\w+)\s*string\s*=\s*"(\w+)"`)                       //nolint:gochecknoglobals
)

func parseEnums(dir string, allTableNames []string) []*structs.BoilerEnum {
	dir, err := filepath.Abs(dir)
	errMessage := "could not open enum names file, this could not lead to problems if you're " +
		"using enums in your db"
	if err != nil {
		log.Warn().Err(err).Msg(errMessage)
		return nil
	}
	content, err := ioutil.ReadFile(filepath.Join(dir, "boil_types.go"))
	if err != nil {
		log.Warn().Err(err).Msg(errMessage)
		return nil
	}
	matches := enumRegex.FindAllStringSubmatch(string(content), -1)
	a := make([]*structs.BoilerEnum, len(matches))
	for i, match := range matches {
		// 1: messageLetterStatus
		// 2: status
		// 3: contents
		modelName, fieldKey := stripLastWord(match[1], allTableNames)
		name := strcase.ToCamel(match[1])

		a[i] = &structs.BoilerEnum{
			Name:          name,
			ModelName:     strcase.ToCamel(modelName),
			ModelFieldKey: strcase.ToCamel(fieldKey),
			Values:        parseEnumValues(match[2]),
		}
	}
	return a
}

func stripLastWord(v string, allTableNames []string) (string, string) {
	// longest tables first
	sort.Slice(allTableNames, func(i, j int) bool {
		return len(allTableNames[i]) > len(allTableNames[j])
	})

	for _, tableName := range allTableNames {
		if strings.HasPrefix(v, tableName) {
			return tableName, strings.TrimPrefix(v, tableName)
		}
	}
	log.Warn().Str("enumName", v).Msg("could not find model by enum")
	return "", ""
}

func isUpperRune(s rune) bool {
	if !unicode.IsUpper(s) && unicode.IsLetter(s) {
		return false
	}
	return true
}

func parseEnumValues(content string) []*structs.BoilerEnumValue {
	matches := enumValuesRegex.FindAllStringSubmatch(content, -1)
	a := make([]*structs.BoilerEnumValue, len(matches))
	for i, match := range matches {
		// 1: message_letter
		// 2: status
		// 2: status
		// 3: contents
		a[i] = &structs.BoilerEnumValue{
			Name: match[1],
		}
	}
	return a
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
				// TODO: make cleaner
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

							if !IsFirstCharacterLowerCase(name) {
								//nolint:errcheck //TODO: handle errors
								t, _ := field.Type.(*ast.ArrayType)

								k := safeTypeSpec.Name.String() + "." + name

								fieldsMap[k] = t.Elt.(*ast.Ident).Name + "Slice"
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

	//// To store the keys in slice in sorted order
	//var keys []string
	//for k := range fieldsMap {
	//	keys = append(keys, k)
	//}
	//sort.Strings(keys)
	//
	//fmt.Println(" ")
	//fmt.Println(" ")
	//fmt.Println(" ")
	//fmt.Println("START OF MAP DUMP")
	//fmt.Println("START OF MAP DUMP")
	//fmt.Println("START OF MAP DUMP")
	//fmt.Println(" ")
	//fmt.Println(" ")
	//for _, key := range keys {
	//	fmt.Println(key, ":", fieldsMap[key])
	//}
	//fmt.Println(" ")
	//fmt.Println(" ")
	//fmt.Println("END OF MAP DUMP")
	//fmt.Println("END OF MAP DUMP")
	//fmt.Println("END OF MAP DUMP")
	//fmt.Println(" ")
	//fmt.Println(" ")
	//fmt.Println(" ")

	return fieldsMap, structsMap, fieldsOrder
}
