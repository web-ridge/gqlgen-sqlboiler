This program generates code like this between your generated gqlgen and sqlboiler with support for Relay.dev (unique id's etc). We can automatically generate the implementation of queries and mutations like create, update, delete working based on your graphql scheme and your sqlboiler models.

To make this program a success tight coupling (same naming) between your database and graphql scheme is needed at the moment. The advantage of this program is the most when you have a database already designed. However everything is created with support for change so you could write some extra GrapQL resolvers if you'd like without a problem.

## Flow

1. Generate database structs with: https://github.com/volatiletech/sqlboiler  
   e.g. `sqlboiler mysql`
2. (optional, but recommended) Generate GrapQL scheme from sqlboiler structs: https://github.com/web-ridge/sqlboiler-graphql-schema  
   e.g. `go run github.com/web-ridge/sqlboiler-graphql-schema --output=../schema.graphql`
3. Generate GrapQL structs with: https://github.com/99designs/gqlgen  
   e.g. `go run github.com/99designs/gqlgen`
4. Generate converts between gqlgen-sqlboiler with this program  
   e.g. `go run convert_plugin.go` for file contents of that program see bottom of this readme

DONE: Generate converts between sqlboiler structs and graphql (with relations included)  
DONE: Generate converts between input models and sqlboiler  
DONE: Fetch sqlboiler preloads from graphql context  
DONE: Support for foreign keys named differently than their corresponding model  
DONE: New plugin which generates CRUD resolvers based on mutations in graphql scheme  
DONE: Support one-to-one relationships inside input types
TODO: Generate code which implements the generated where and search filters  
TODO: Batch create/update/delete generation in resolvers  
TODO: Support gqlgen multiple .graphql files  
TODO: Edges/connections  
TODO: Crud of adding/removing relationships from many-to-many on edges
TODO: Support more relationships inside input types  
TODO: Do a three-way-diff merge for changes and let user choose parts of code which should not take over generated code

## Case

You have a personal project with a very big database and a 'Laravel API'. I want to be able to generate a new Golang GraphQL API for this project in no time.

## Example result of this plugin

```golang
func AddressToGraphQL(m *models.Address, roots []interface{}) *graphql_models.Address {
	if m == nil {
		return nil
	}

	r := &graphql_models.Address{
		ID:          AddressIDUnique(m.ID),
		Street:      helper.NullDotStringToPointerString(m.Street),
		HouseNumber: helper.NullDotStringToPointerString(m.HouseNumber),
		ZipAddress:  helper.NullDotStringToPointerString(m.ZipAddress),
		City:        helper.NullDotStringToPointerString(m.City),
		Longitude:   helper.TypesNullDecimalToFloat64(m.Longitude),
		Latitude:    helper.TypesNullDecimalToFloat64(m.Latitude),
		Description: helper.NullDotStringToPointerString(m.Description),
		Name:        helper.NullDotStringToPointerString(m.Name),
		Permission:  helper.NullDotBoolToPointerBool(m.Permission),
		UpdatedAt:   helper.NullDotTimeToPointerInt(m.UpdatedAt),
		DeletedAt:   helper.NullDotTimeToPointerInt(m.DeletedAt),
		CreatedAt:   helper.NullDotTimeToPointerInt(m.CreatedAt),
	}

	if helper.UintIsFilled(m.AddressStatusID) {
		if m.R != nil && m.R.AddressStatus != nil {
			if !alreadyConverted(roots, m.R.AddressStatus) {
				r.AddressStatus = AddressStatusToGraphQL(m.R.AddressStatus, append(roots, m))
			}
		} else {
			r.AddressStatus = AddressStatusWithUintID(m.AddressStatusID)
		}
	}

	if helper.NullDotUintIsFilled(m.CompanyID) {
		if m.R != nil && m.R.Company != nil {
			if !alreadyConverted(roots, m.R.Company) {
				r.Company = CompanyToGraphQL(m.R.Company, append(roots, m))
			}
		} else {
			r.Company = CompanyWithNullDotUintID(m.CompanyID)
		}
	}

	if helper.NullDotUintIsFilled(m.ContactPersonID) {
		if m.R != nil && m.R.ContactPerson != nil {
			if !alreadyConverted(roots, m.R.ContactPerson) {
				r.ContactPerson = PersonToGraphQL(m.R.ContactPerson, append(roots, m))
			}
		} else {
			r.ContactPerson = PersonWithNullDotUintID(m.ContactPersonID)
		}
	}

	if helper.NullDotUintIsFilled(m.HouseTypeID) {
		if m.R != nil && m.R.HouseType != nil {
			if !alreadyConverted(roots, m.R.HouseType) {
				r.HouseType = HouseTypeToGraphQL(m.R.HouseType, append(roots, m))
			}
		} else {
			r.HouseType = HouseTypeWithNullDotUintID(m.HouseTypeID)
		}
	}

	if helper.NullDotUintIsFilled(m.OwnerID) {
		if m.R != nil && m.R.Owner != nil {
			if !alreadyConverted(roots, m.R.Owner) {
				r.Owner = PersonToGraphQL(m.R.Owner, append(roots, m))
			}
		} else {
			r.Owner = PersonWithNullDotUintID(m.OwnerID)
		}
	}

	if helper.UintIsFilled(m.UserOrganizationID) {
		if m.R != nil && m.R.UserOrganization != nil {
			if !alreadyConverted(roots, m.R.UserOrganization) {
				r.UserOrganization = UserOrganizationToGraphQL(m.R.UserOrganization, append(roots, m))
			}
		} else {
			r.UserOrganization = UserOrganizationWithUintID(m.UserOrganizationID)
		}
	}
	if m.R != nil && m.R.Calamities != nil {
		r.Calamities = CalamitiesToGraphQL(m.R.Calamities, append(roots, m))
	}
	if m.R != nil && m.R.People != nil {
		r.People = PeopleToGraphQL(m.R.People, append(roots, m))
	}

	return r
}
```

sqlboiler.yml

```yaml
mysql:
  dbname: dbname
  host: localhost
  port: 8889
  user: root
  pass: root
  sslmode: "false"
  blacklist:
    - notifications
    - jobs
    - password_resets
    - migrations
mysqldump:
  column-statistics: 0
```

gqlgen.yml

```yaml
schema:
  - schema.graphql
exec:
  filename: graphql_models/generated.go
  package: graphql_models
model:
  filename: graphql_models/genereated_models.go
  package: graphql_models
resolver:
  filename: resolver.go
  type: Resolver
```

Run normal generator
`go run github.com/99designs/gqlgen -v`

Put this in your program convert_plugin.go e.g.

```golang
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	cm "github.com/web-ridge/gqlgen-sqlboiler/convert"
	rm "github.com/web-ridge/gqlgen-sqlboiler/resolver"
)

func main() {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	convertHelpersDir := "helpers"
	sqlboilerDir := "models"
	gqlgenModelDir := "graphql_models"
	err = api.Generate(cfg,
		api.AddPlugin(cm.New(
			convertHelpersDir, // directory where convert.go, convert_input.go and preload.go should live
			sqlboilerDir,      // directory where sqlboiler files are put
			gqlgenModelDir,    // directory where gqlgen models live
		)),
		api.AddPlugin(rm.New(
			convertHelpersDir,
			sqlboilerDir,
			gqlgenModelDir,
			"github.com/yourauth/implementation" // leave empty if you don't have auth
		)),
	)
	if err != nil {
		fmt.Println("error!!")
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
}
```

`go run convert_plugin.go`
