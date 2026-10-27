package gbgen

import (
	"strings"
	"testing"

	"github.com/web-ridge/gqlgen-sqlboiler/v3/cache"
	"github.com/web-ridge/gqlgen-sqlboiler/v3/structs"
)

func optInTestCache() *cache.BoilerCache {
	organization := &structs.BoilerModel{Name: "Organization", TableName: "organization", PluralName: "Organizations"}
	organization.Fields = []*structs.BoilerField{
		{Name: "ID", Type: "uint", InTable: true},
		{Name: "Name", Type: "string", IsRequired: true, InTable: true, InTableNotID: true},
	}
	secret := &structs.BoilerModel{Name: "Secret", TableName: "secret", PluralName: "Secrets"}
	secret.Fields = []*structs.BoilerField{
		{Name: "ID", Type: "uint", InTable: true},
		{Name: "Value", Type: "string", IsRequired: true, InTable: true, InTableNotID: true},
	}
	roleEnum := &structs.BoilerEnum{Name: "UserRole", ModelName: "User", ModelFieldKey: "Role", Values: []*structs.BoilerEnumValue{{Name: "UserRoleAdmin"}}}
	secretEnum := &structs.BoilerEnum{Name: "SecretKind", ModelName: "Secret", ModelFieldKey: "Kind", Values: []*structs.BoilerEnumValue{{Name: "SecretKindToken"}}}
	user := &structs.BoilerModel{Name: "User", TableName: "user", PluralName: "Users"}
	user.Fields = []*structs.BoilerField{
		{Name: "ID", Type: "uint", InTable: true},
		{Name: "Name", Type: "string", IsRequired: true, InTable: true, InTableNotID: true},
		{Name: "Email", Type: "string", IsRequired: true, InTable: true, InTableNotID: true},
		{Name: "Role", Type: "string", IsRequired: true, IsEnum: true, Enum: *roleEnum, InTable: true, InTableNotID: true},
		{
			Name: "OrganizationID", Type: "uint", IsRequired: true, IsRelation: true, IsForeignKey: true,
			RelationshipName: "Organization", Relationship: organization, InTable: true, InTableNotID: true,
		},
		{
			Name: "SecretID", Type: "uint", IsRequired: true, IsRelation: true, IsForeignKey: true,
			RelationshipName: "Secret", Relationship: secret, InTable: true, InTableNotID: true,
		},
	}
	return &cache.BoilerCache{
		BoilerModels: []*structs.BoilerModel{organization, secret, user},
		BoilerEnums:  []*structs.BoilerEnum{roleEnum, secretEnum},
	}
}

func TestSchemaWithoutOptInKeepsEverything(t *testing.T) {
	schema, err := schemaGet(SchemaConfig{BoilerCache: optInTestCache(), GenerateMutations: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"secrets(first: Int!, after: String, ordering: [SecretOrdering!], filter: SecretFilter): SecretConnection!",
		"createSecret(input: SecretCreateInput!): SecretPayload!",
		"deleteOrganization(id: ID!): OrganizationDeletePayload!",
		"secret: SecretWhere",
		"enum SecretKind {",
		"mutation: Mutation",
	} {
		if !strings.Contains(schema, want) {
			t.Errorf("schema without opt-in lacks %q", want)
		}
	}
}

func TestSchemaOptIn(t *testing.T) {
	schema, err := schemaGet(SchemaConfig{
		BoilerCache:       optInTestCache(),
		GenerateMutations: true,
		OptIn:             true,
		Models: map[string]ModelExposure{
			"User":         {List: true, Filter: true, WhereFields: []string{"name"}},
			"Organization": {Update: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		snippet string
		present bool
	}{
		{"list without ordering argument", "users(first: Int!, after: String, filter: UserFilter): UserConnection!", true},
		{"pagination types of a list", "input UserOrdering {", true},
		{"no single query", "user(id: ID!)", false},
		{"type-only relation target", "type Organization implements Node {", true},
		{"relation to a listed model", "organization: Organization!", true},
		{"no query for a model without operations", "organizations(", false},
		{"unlisted model", "Secret", false},
		{"enum of an unlisted model", "SecretKind", false},
		{"enum of a listed model", "enum UserRole {", true},
		{"allowed where field", "name: StringFilter", true},
		{"where field outside the allowlist", "email: StringFilter", false},
		{"relation to a model without where", "organization: OrganizationWhere", false},
		{"id stays in a narrowed where", "id: IDFilter", true},
		{"no user mutations", "UserCreateInput", false},
		{"update mutation", "updateOrganization(id: ID!, input: OrganizationUpdateInput!): OrganizationPayload!", true},
		{"no delete payload without delete", "OrganizationDeletePayload", false},
		{"no create input without create", "OrganizationCreateInput", false},
		{"no filter without a list filter", "OrganizationFilter", false},
	}
	for _, tt := range tests {
		if got := strings.Contains(schema, tt.snippet); got != tt.present {
			t.Errorf("%v: contains %q = %v, want %v", tt.name, tt.snippet, got, tt.present)
		}
	}
}

func TestSchemaOptInWithoutMutations(t *testing.T) {
	schema, err := schemaGet(SchemaConfig{
		BoilerCache:       optInTestCache(),
		GenerateMutations: true,
		OptIn:             true,
		Models:            map[string]ModelExposure{"Organization": {Single: true, Payload: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(schema, "Mutation") {
		t.Error("schema without mutations still declares a Mutation type")
	}
	if !strings.Contains(schema, "type OrganizationPayload {") {
		t.Error("explicit payload is missing")
	}
}

func TestSchemaOptInRejectsUnknownNames(t *testing.T) {
	_, err := schemaGet(SchemaConfig{
		BoilerCache: optInTestCache(),
		OptIn:       true,
		Models: map[string]ModelExposure{
			"Usr":  {List: true},
			"User": {Where: true, WhereFields: []string{"nme"}},
		},
	})
	if err == nil {
		t.Fatal("expected an error for unknown names")
	}
	for _, want := range []string{"unknown model Usr", "unknown where field User.nme"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
