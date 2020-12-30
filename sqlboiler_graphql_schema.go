// TODO: needs big refactor

package gbgen

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/iancoleman/strcase"
)

const (
	indent    = "  "
	lineBreak = "\n"
)

type SchemaConfig struct {
	BoilerModelDirectory Config
	Directives           []string
	SkipInputFields      []string
	GenerateBatchCreate  bool
	GenerateMutations    bool
	GenerateBatchDelete  bool
	GenerateBatchUpdate  bool
}

type SchemaGenerateConfig struct {
	MergeSchema bool
}

type SchemaModel struct {
	Name   string
	Fields []*SchemaField
}

type SchemaField struct {
	Name             string
	Type             string // String, ID, Integer
	RelationName     string // posts
	RelationType     string // Page, User, Post
	FullType         string // e.g String! or if array [String!]
	RelationFullType string // [Posts!]
	FullTypeOptional string // e.g. String or if array [String]
	BoilerField      *BoilerField
}

func SchemaWrite(config SchemaConfig, outputFile string, generateOptions SchemaGenerateConfig) error {
	// Generate schema based on config
	schema := SchemaGet(
		config,
	)

	// TODO: Write schema to the configured location
	if fileExists(outputFile) && generateOptions.MergeSchema {
		if err := mergeContentInFile(schema, outputFile); err != nil {
			log.Err(err).Msg("Could not write schema to disk")
			return err
		}
	} else {
		log.Debug().Int("bytes", len(schema)).Str("file", outputFile).Msg("write GraphQL schema to disk")
		if err := writeContentToFile(schema, outputFile); err != nil {
			log.Err(err).Msg("Could not write schema to disk")
			return err
		}
		return formatFile(outputFile)
	}

	return nil
}

//nolint:gocognit,gocyclo
func SchemaGet(
	config SchemaConfig,
) string {
	w := &SimpleWriter{}

	// Parse models and their fields based on the sqlboiler model directory
	boilerModels := GetBoilerModels(config.BoilerModelDirectory.Directory)
	models := boilerModelsToModels(boilerModels)

	fullDirectives := make([]string, len(config.Directives))
	for i, defaultDirective := range config.Directives {
		fullDirectives[i] = "@" + defaultDirective
		w.l(fmt.Sprintf("directive @%v on FIELD_DEFINITION", defaultDirective))
	}
	w.br()

	joinedDirectives := strings.Join(fullDirectives, " ")

	w.l(`schema {`)
	w.tl(`query: Query`)
	w.tl(`mutation: Mutation`)
	w.l(`}`)

	w.br()

	w.l(`interface Node {`)
	w.tl(`id: ID!`)
	w.l(`}`)

	w.br()

	w.l(`type PageInfo {`)
	w.tl(`hasNextPage: Boolean!`)
	w.tl(`hasPreviousPage: Boolean!`)
	w.tl(`startCursor: String`)
	w.tl(`endCursor: String`)
	w.l(`}`)

	w.br()

	// Generate sorting helpers
	w.l("enum SortDirection { ASC, DESC }")
	w.br()

	for _, model := range models {
		//	enum UserSort { FIRST_NAME, LAST_NAME }
		w.l("enum " + model.Name + "Sort {")
		for _, enum := range fieldAsEnumStrings(model.Fields) {
			w.tl(enum)
		}
		w.l("}")

		w.br()

		//	input UserOrdering {
		//		sort: UserSort!
		//		direction: SortDirection! = ASC
		//	}
		w.l("input " + model.Name + "Ordering {")
		w.tl("sort: " + model.Name + "Sort!")
		w.tl("direction: SortDirection! = ASC")
		w.l("}")

		w.br()
	}

	// Create basic structs e.g.
	// type User {
	// 	firstName: String!
	// 	lastName: String
	// 	isProgrammer: Boolean!
	// 	organization: Organization!
	// }
	for _, model := range models {
		w.l("type " + model.Name + " implements Node {")

		for _, field := range model.Fields {
			// e.g we have foreign key from user to organization
			// organizationID is clutter in your scheme
			// you only want Organization and OrganizationID should be skipped
			if field.BoilerField.IsRelation {
				w.tl(field.RelationName + ": " + field.RelationFullType)
			} else {
				w.tl(field.Name + ": " + field.FullType)
			}
		}
		w.l("}")

		w.br()
	}

	//type UserEdge {
	//	cursor: String!
	//	node: User
	//}
	for _, model := range models {
		w.l("type " + model.Name + "Edge {")

		w.tl(`cursor: String!`)
		w.tl(`node: ` + model.Name)
		w.l("}")

		w.br()
	}

	//type UserConnection {
	//	edges: [UserEdge]
	//	pageInfo: PageInfo!
	//}
	for _, model := range models {
		w.l("type " + model.Name + "Connection {")
		w.tl(`edges: [` + model.Name + `Edge]`)
		w.tl(`pageInfo: PageInfo!`)
		w.l("}")

		w.br()
	}

	// Add helpers for filtering lists
	w.l(queryHelperStructs)

	// generate filter structs per model
	for _, model := range models {
		// Ignore some specified input fields
		// Generate a type safe grapql filter

		// Generate the base filter
		// type UserFilter {
		// 	search: String
		// 	where: UserWhere
		// }
		w.l("input " + model.Name + "Filter {")
		w.tl("search: String")
		w.tl("where: " + model.Name + "Where")
		w.l("}")

		w.br()

		// Generate a where struct
		// type UserWhere {
		// 	id: IDFilter
		// 	title: StringFilter
		// 	organization: OrganizationWhere
		// 	or: FlowBlockWhere
		// 	and: FlowBlockWhere
		// }
		w.l("input " + model.Name + "Where {")

		for _, field := range model.Fields {
			if field.BoilerField.IsRelation {
				// Support filtering in relationships (atleast schema wise)
				w.tl(field.RelationName + ": " + field.RelationType + "Where")
			} else {
				w.tl(field.Name + ": " + field.Type + "Filter")
			}
		}
		w.tl("or: " + model.Name + "Where")
		w.tl("and: " + model.Name + "Where")
		w.l("}")

		w.br()
	}

	w.l("type Query {")
	w.tl("node(id: ID!): Node")

	for _, model := range models {
		// single models
		w.tl(strcase.ToLowerCamel(model.Name) + "(id: ID!): " + model.Name + "!" + joinedDirectives)

		// lists
		modelPluralName := pluralizer.Plural(model.Name)

		arguments := []string{
			"first: Int!",
			"after: String",
			"ordering: [" + model.Name + "Ordering!]",
			"filter: " + model.Name + "Filter",
		}
		w.tl(
			strcase.ToLowerCamel(modelPluralName) + "(" + strings.Join(arguments, ", ") + "): " +
				model.Name + "Connection!" + joinedDirectives)
	}
	w.l("}")

	w.br()

	// Generate input and payloads for mutatations
	if config.GenerateMutations { //nolint:nestif
		for _, model := range models {
			filteredFields := fieldsWithout(model.Fields, config.SkipInputFields)

			modelPluralName := pluralizer.Plural(model.Name)
			// input UserCreateInput {
			// 	firstName: String!
			// 	lastName: String
			//	organizationId: ID!
			// }
			w.l("input " + model.Name + "CreateInput {")

			for _, field := range filteredFields {
				// id is not required in create and will be specified in update resolver
				if field.Name == "id" {
					continue
				}
				// not possible yet in input
				// TODO: make this possible for one-to-one structs?
				// only for foreign keys inside model itself
				if field.BoilerField.IsRelation && field.BoilerField.IsArray ||
					field.BoilerField.IsRelation && !strings.HasSuffix(field.BoilerField.Name, "ID") {
					continue
				}
				w.tl(field.Name + ": " + field.FullType)
			}
			w.l("}")

			w.br()

			// input UserUpdateInput {
			// 	firstName: String!
			// 	lastName: String
			//	organizationId: ID!
			// }
			w.l("input " + model.Name + "UpdateInput {")

			for _, field := range filteredFields {
				// id is not required in create and will be specified in update resolver
				if field.Name == "id" {
					continue
				}
				// not possible yet in input
				// TODO: make this possible for one-to-one structs?
				// only for foreign keys inside model itself
				if field.BoilerField.IsRelation && field.BoilerField.IsArray ||
					field.BoilerField.IsRelation && !strings.HasSuffix(field.BoilerField.Name, "ID") {
					continue
				}
				w.tl(field.Name + ": " + field.FullTypeOptional)
			}
			w.l("}")

			w.br()

			if config.GenerateBatchCreate {
				w.l("input " + modelPluralName + "CreateInput {")

				w.tl(strcase.ToLowerCamel(modelPluralName) + ": [" + model.Name + "CreateInput!]!")
				w.l("}")

				w.br()
			}

			// if batchUpdate {
			// 	w.l("input " + modelPluralName + "UpdateInput {")
			// 	w.tl(strcase.ToLowerCamel(modelPluralName) + ": [" + model.Name + "UpdateInput!]!")
			// 	w.l("}")
			// 	w.br()
			// }

			// type UserPayload {
			// 	user: User!
			// }
			w.l("type " + model.Name + "Payload {")
			w.tl(strcase.ToLowerCamel(model.Name) + ": " + model.Name + "!")
			w.l("}")

			w.br()

			// TODO batch, delete input and payloads

			// type UserDeletePayload {
			// 	id: ID!
			// }
			w.l("type " + model.Name + "DeletePayload {")
			w.tl("id: ID!")
			w.l("}")

			w.br()

			// type UsersPayload {
			// 	users: [User!]!
			// }
			if config.GenerateBatchCreate {
				w.l("type " + modelPluralName + "Payload {")
				w.tl(strcase.ToLowerCamel(modelPluralName) + ": [" + model.Name + "!]!")
				w.l("}")

				w.br()
			}

			// type UsersDeletePayload {
			// 	ids: [ID!]!
			// }
			if config.GenerateBatchDelete {
				w.l("type " + modelPluralName + "DeletePayload {")
				w.tl("ids: [ID!]!")
				w.l("}")

				w.br()
			}
			// type UsersUpdatePayload {
			// 	ok: Boolean!
			// }
			if config.GenerateBatchUpdate {
				w.l("type " + modelPluralName + "UpdatePayload {")
				w.tl("ok: Boolean!")
				w.l("}")

				w.br()
			}
		}

		// Generate mutation queries
		w.l("type Mutation {")

		for _, model := range models {
			modelPluralName := pluralizer.Plural(model.Name)

			// create single
			// e.g createUser(input: UserInput!): UserPayload!
			w.tl("create" + model.Name + "(input: " + model.Name + "CreateInput!): " +
				model.Name + "Payload!" + joinedDirectives)

			// create multiple
			// e.g createUsers(input: [UsersInput!]!): UsersPayload!
			if config.GenerateBatchCreate {
				w.tl("create" + modelPluralName + "(input: " + modelPluralName + "CreateInput!): " +
					modelPluralName + "Payload!" + joinedDirectives)
			}

			// update single
			// e.g updateUser(id: ID!, input: UserInput!): UserPayload!
			w.tl("update" + model.Name + "(id: ID!, input: " + model.Name + "UpdateInput!): " +
				model.Name + "Payload!" + joinedDirectives)

			// update multiple (batch update)
			// e.g updateUsers(filter: UserFilter, input: UsersInput!): UsersPayload!
			if config.GenerateBatchUpdate {
				w.tl("update" + modelPluralName + "(filter: " + model.Name + "Filter, input: " +
					model.Name + "UpdateInput!): " + modelPluralName + "UpdatePayload!" + joinedDirectives)
			}

			// delete single
			// e.g deleteUser(id: ID!): UserPayload!
			w.tl("delete" + model.Name + "(id: ID!): " + model.Name + "DeletePayload!" + joinedDirectives)

			// delete multiple
			// e.g deleteUsers(filter: UserFilter, input: [UsersInput!]!): UsersPayload!
			if config.GenerateBatchDelete {
				w.tl("delete" + modelPluralName + "(filter: " + model.Name + "Filter): " +
					modelPluralName + "DeletePayload!" + joinedDirectives)
			}
		}
		w.l("}")

		w.br()
	}

	return w.s.String()
}

func fieldAsEnumStrings(fields []*SchemaField) []string {
	var enums []string
	for _, field := range fields {
		if field.BoilerField != nil && (!field.BoilerField.IsRelation && !field.BoilerField.IsForeignKey) {
			enums = append(enums, strcase.ToScreamingSnake(field.Name))
		}
	}
	return enums
}

func getFullType(fieldType string, isArray bool, isRequired bool) string {
	gType := fieldType

	if isArray {
		// To use a list type, surround the type in square brackets, so [Int] is a list of integers.
		gType = "[" + gType + "]"
	}
	if isRequired {
		// Use an exclamation point to indicate a type cannot be nullable,
		// so String! is a non-nullable string.
		gType += "!"
	}
	return gType
}

func boilerModelsToModels(boilerModels []*BoilerModel) []*SchemaModel {
	models := make([]*SchemaModel, len(boilerModels))
	for i, boilerModel := range boilerModels {
		models[i] = &SchemaModel{
			Name:   boilerModel.Name,
			Fields: boilerFieldsToFields(boilerModel.Fields),
		}
	}
	return models
}

func boilerFieldsToFields(boilerFields []*BoilerField) []*SchemaField {
	fields := make([]*SchemaField, len(boilerFields))
	for i, boilerField := range boilerFields {
		fields[i] = boilerFieldToField(boilerField)
	}
	return fields
}

func boilerFieldToField(boilerField *BoilerField) *SchemaField {
	var relationName string
	var relationType string
	var relationFullType string
	if boilerField.Relationship != nil {
		relationName = strcase.ToLowerCamel(boilerField.RelationshipName)
		relationType = boilerField.Relationship.Name

		relationFullType = getFullType(
			relationType,
			boilerField.IsArray,
			boilerField.IsRequired,
		)
	}

	t := toGraphQLType(boilerField.Name, boilerField.Type)
	return &SchemaField{
		Name:             toGraphQLName(boilerField.Name),
		RelationName:     relationName,
		RelationType:     relationType,
		Type:             t,
		FullType:         getFullType(t, boilerField.IsArray, boilerField.IsRequired),
		FullTypeOptional: getFullType(t, boilerField.IsArray, false),
		RelationFullType: relationFullType,
		BoilerField:      boilerField,
	}
}

func toGraphQLName(fieldName string) string {
	graphqlName := fieldName

	// Golang ID to Id the right way
	// Primary key
	if graphqlName == "ID" {
		graphqlName = "id"
	}

	if graphqlName == "URL" {
		graphqlName = "url"
	}

	// e.g. OrganizationID, TODO: more robust solution?
	graphqlName = strings.Replace(graphqlName, "ID", "Id", -1)
	graphqlName = strings.Replace(graphqlName, "URL", "Url", -1)

	return strcase.ToLowerCamel(graphqlName)
}

func toGraphQLType(fieldName, boilerType string) string {
	lowerFieldName := strings.ToLower(fieldName)
	lowerBoilerType := strings.ToLower(boilerType)

	if strings.HasSuffix(lowerFieldName, "id") {
		return "ID"
	}
	if strings.Contains(lowerBoilerType, "string") {
		return "String"
	}
	if strings.Contains(lowerBoilerType, "int") {
		return "Int"
	}
	if strings.Contains(lowerBoilerType, "byte") {
		return "String"
	}
	if strings.Contains(lowerBoilerType, "decimal") || strings.Contains(lowerBoilerType, "float") {
		return "Float"
	}
	if strings.Contains(lowerBoilerType, "bool") {
		return "Boolean"
	}

	// TODO: make this a scalar or something configurable?
	// I like to use unix here
	if strings.Contains(lowerBoilerType, "time") {
		return "Int"
	}

	// E.g. UserSlice
	boilerType = strings.TrimSuffix(boilerType, "Slice")

	// e.g. null.JSON let user define how it looks with their own struct
	return strcase.ToCamel(fieldName)
}

func fieldsWithout(fields []*SchemaField, skipFieldNames []string) []*SchemaField {
	var filteredFields []*SchemaField
	for _, field := range fields {
		if !sliceContains(skipFieldNames, field.Name) {
			filteredFields = append(filteredFields, field)
		}
	}
	return filteredFields
}

func mergeContentInFile(content, outputFile string) error {
	baseFile := filenameWithoutExtension(outputFile) +
		"-empty" +
		getFilenameExtension(outputFile)

	newOutputFile := filenameWithoutExtension(outputFile) +
		"-new" +
		getFilenameExtension(outputFile)

	// remove previous files if exist
	_ = os.Remove(baseFile)
	_ = os.Remove(newOutputFile)

	if err := writeContentToFile(content, newOutputFile); err != nil {
		return fmt.Errorf("could not write schema to disk: %v", err)
	}
	//if err := formatFile(outputFile); err != nil {
	//	return fmt.Errorf("could not format with prettier %v", err)
	//}
	//if err := formatFile(newOutputFile); err != nil {
	//	return fmt.Errorf("could not format with prettier%v", err)
	//}

	// Three way merging done based on this answer
	// https://stackoverflow.com/a/9123563/2508481

	// Empty file as base per the stackoverflow answer
	name := "touch"
	args := []string{baseFile}
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		log.Err(err).Str("name", name).Str("args", strings.Join(args, " ")).Msg("merging failed")
		return fmt.Errorf("merging failed %v: %v", err, out)
	}

	// Let's do the merge
	name = "git"
	args = []string{"merge-file", outputFile, baseFile, newOutputFile}
	out, err = exec.Command(name, args...).Output()
	if err != nil {
		log.Err(err).Str("name", name).Str("args", strings.Join(args, " ")).Msg("executing command failed")

		// remove base file
		_ = os.Remove(baseFile)
		return fmt.Errorf("merging failed or had conflicts %v: %v", err, out)
	}
	log.Info().Msg("merging done without conflicts")

	// remove files
	_ = os.Remove(baseFile)
	_ = os.Remove(newOutputFile)
	return nil
}

func getFilenameExtension(fn string) string {
	return path.Ext(fn)
}

func filenameWithoutExtension(fn string) string {
	return strings.TrimSuffix(fn, path.Ext(fn))
}

func formatFile(filename string) error {
	name := "prettier"
	args := []string{filename, "--write"}

	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return fmt.Errorf("executing command: '%v %v' failed with: %v, output: %v", name, strings.Join(args, " "), err, out)
	}
	// fmt.Println(fmt.Sprintf("Formatting of %v done", filename))
	return nil
}

func writeContentToFile(content string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("could not write %v to disk: %v", filename, err)
	}

	// Close file if this functions returns early or at the end
	defer func() {
		closeErr := file.Close()
		if closeErr != nil {
			log.Err(closeErr).Msg("error while closing file")
		}
	}()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("could not write content to file %v: %v", filename, err)
	}

	return nil
}

type SimpleWriter struct {
	s strings.Builder
}

func (sw *SimpleWriter) l(v string) {
	sw.s.WriteString(v + lineBreak)
}

func (sw *SimpleWriter) br() {
	sw.s.WriteString(lineBreak)
}

func (sw *SimpleWriter) tl(v string) {
	sw.s.WriteString(indent + v + lineBreak)
}

// TODO: only generate these if they are set
const queryHelperStructs = `
input IDFilter {
	equalTo: ID
	notEqualTo: ID
	in: [ID!]
	notIn: [ID!]
}

input StringFilter {
	equalTo: String
	notEqualTo: String

	in: [String!]
	notIn: [String!]

	startWith: String
	notStartWith: String

	endWith: String
	notEndWith: String

	contain: String
	notContain: String

	startWithStrict: String # Camel sensitive
	notStartWithStrict: String # Camel sensitive

	endWithStrict: String # Camel sensitive
	notEndWithStrict: String # Camel sensitive

	containStrict: String # Camel sensitive
	notContainStrict: String # Camel sensitive
}

input IntFilter {
	equalTo: Int
	notEqualTo: Int
	lessThan: Int
	lessThanOrEqualTo: Int
	moreThan: Int
	moreThanOrEqualTo: Int
	in: [Int!]
	notIn: [Int!]
}

input FloatFilter {
	equalTo: Float
	notEqualTo: Float
	lessThan: Float
	lessThanOrEqualTo: Float
	moreThan: Float
	moreThanOrEqualTo: Float
	in: [Float!]
	notIn: [Float!]
}

input BooleanFilter {
	equalTo: Boolean
	notEqualTo: Boolean
}
`
