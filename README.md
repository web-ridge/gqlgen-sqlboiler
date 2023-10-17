# gqlgen-sqlboiler

We want developers to be able to build software faster using modern tools like GraphQL, Golang, React Native without depending on commercial providers like Firebase or AWS Amplify.

Our plugins generate type-safe code between gqlgen and sqlboiler models with support for unique id's across your whole database. We can automatically generate the implementation of queries and mutations like create, update, delete based on your graphql scheme and your sqlboiler models.

Tight coupling between your database and graphql scheme is required otherwise generation will be skipped. The advantage of this program is the most when you have a database already designed. You can write extra GrapQL resolvers, and override the generated functions so you can iterate fast.

## Why gqlgen and sqlboiler

They go back to a schema first approach which we like. The generated code with these tools are the most efficient and fast in the Golang system (and probably outside of it too).
* [sqlboiler](https://github.com/volatiletech/sqlboiler#benchmarks)
* [gqlgen](https://github.com/appleboy/golang-graphql-benchmark#summary)

It's really amazing how fast a generated api with these techniques is!

## Usage

### Step 1
Create folder convert/convert.go with the following content:
See [example of `convert.go`](https://github.com/web-ridge/gqlgen-sqlboiler#convert.go)

### Step 2
run `go mod tidy` in `convert/` 

### Step 3
Make sure you have [followed the prerequisites](https://github.com/web-ridge/gqlgen-sqlboiler#prerequisites)   

### Step 4
```sh 
(cd convert && go run convert.go)
```


   
   

## Features

- [x] schema.graphql based on sqlboiler structs
- [x] converts between sqlboiler and gqlgen
- [x] connections / edges / filtering / ordering / sorting
- [x] three-way-merge schema re-generate
- [x] converts between input models and sqlboiler
- [x] understands the difference between empty and null in update input
- [x] sqlboiler preloads from graphql context
- [x] foreign keys and relations
- [x] resolvers based on queries/mutations in schema
- [x] one-to-one relationships inside input types.
- [x] batch update/delete generation in resolvers.
- [x] enum support (only in graphql schema right now).
- [x] public errors in resolvers + logging via zerolog.
- [x] [overriding convert functions](https://github.com/web-ridge/gqlgen-sqlboiler#overriding-converts)
- [x] [custom scope resolvers](https://github.com/web-ridge/gqlgen-sqlboiler-examples/blob/main/social-network/convert_plugin.go#L66) e.g userId, organizationId
- [x] Support gqlgen multiple .graphql files
- [x] Batch create helpers for sqlboiler and integration batch create inputs
### Relay
- [x] [GraphQL Cursor Connections Specification](https://relay.dev/graphql/connections.htm)
- [x] [Global Object Identification](https://graphql.org/learn/global-object-identification/)
### Roadmap
- [ ] Support automatic converts for custom schema objects
- [ ] Support overriding resolvers
- [ ] Support multiple resolvers (per schema)
- [ ] Adding automatic database migrations and integration with [web-ridge/dbifier](https://github.com/web-ridge/dbifier)
- [ ] Crud of adding/removing relationships from one-to-many and many-to-many on edges
- [ ] Support more relationships inside input types
- [ ] Generate tests
- [ ] Run automatic tests in Github CI/CD in https://github.com/web-ridge/gqlgen-sqlboiler-examples

## Examples
Checkout our examples to see the generated schema.grapql, converts and resolvers.   
[web-ridge/gqlgen-sqlboiler-examples](https://github.com/web-ridge/gqlgen-sqlboiler-examples)

### Output example
```go
func PostToGraphQL(m *models.Post) *graphql_models.Post {
	if m == nil {
		return nil
	}
	r := &graphql_models.Post{
		ID:      PostIDToGraphQL(m.ID),
		Content: m.Content,
	}
	if boilergql.UintIsFilled(m.UserID) {
		if m.R != nil && m.R.User != nil {
			r.User = UserToGraphQL(m.R.User)
		} else {
			r.User = UserWithUintID(m.UserID)
		}
	}
	if m.R != nil && m.R.Comments != nil {
		r.Comments = CommentsToGraphQL(m.R.Comments)
	}
	if m.R != nil && m.R.Images != nil {
		r.Images = ImagesToGraphQL(m.R.Images)
	}
	if m.R != nil && m.R.Likes != nil {
		r.Likes = LikesToGraphQL(m.R.Likes)
	}
	return r
}
```


## Prerequisites

### sqlboiler.yml

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

### gqlgen.yml

```yaml
schema:
  - *.graphql
exec:
  filename: models/fm/generated.go
  package: fm
model:
  filename: models/fm/generated_models.go
  package: fm
models:
  ConnectionBackwardPagination:
    model: github.com/web-ridge/utils-go/boilergql/v3.ConnectionBackwardPagination
  ConnectionForwardPagination:
    model: github.com/web-ridge/utils-go/boilergql/v3.ConnectionForwardPagination
  ConnectionPagination:
    model: github.com/web-ridge/utils-go/boilergql/v3.ConnectionPagination
  SortDirection:
    model: github.com/web-ridge/utils-go/boilergql/v3.SortDirection
```

### resolver/resolver.go
```go

package resolvers

import (
	"database/sql"
)

type Resolver struct {
	db        *sql.DB
	// you can add more here
}

func NewResolver(db *sql.DB) *Resolver {
	return &Resolver{
		db:        db,
        // you can add more here
	}
}

```

### convert.go
Put something like the code below in file convert/convert.go

```go
package main

import (
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/rs/zerolog/log"
	gbgen "github.com/web-ridge/gqlgen-sqlboiler/v3"
	"github.com/web-ridge/gqlgen-sqlboiler/v3/cache"
	"github.com/web-ridge/gqlgen-sqlboiler/v3/structs"
	"os"
	"os/exec"
	"strings"
)

func main() {
	// change working directory to parent directory where all configs are located
	newDir, _ := os.Getwd()
	os.Chdir(strings.TrimSuffix(newDir, "/convert"))

	enableSoftDeletes := true
	boilerArgs := []string{"mysql", "--no-back-referencing", "--wipe", "-d"}
	if enableSoftDeletes {
		boilerArgs = append(boilerArgs, "--add-soft-deletes")
	}
	cmd := exec.Command("sqlboiler", boilerArgs...)

	err := cmd.Run()
	if err != nil {
		log.Fatal().Err(err).Str("command", cmd.String()).Msg("error generating dm models running sql-boiler")
	}

	output := structs.Config{
		Directory:   "helpers", // supports root or sub directories
		PackageName: "helpers",
	}
	backend := structs.Config{
		Directory:   "models/dm",
		PackageName: "dm",
	}
	frontend := structs.Config{
		Directory:   "models/fm",
		PackageName: "fm",
	}

	boilerCache := cache.InitializeBoilerCache(backend)

	generateSchema := true
	generatedSchema := !generateSchema
	if generateSchema {
		if err := gbgen.SchemaWrite(
			gbgen.SchemaConfig{
				BoilerCache:         boilerCache,
				Directives:          []string{"isAuthenticated"},
				SkipInputFields:     []string{"createdAt", "updatedAt", "deletedAt"},
				GenerateMutations:   true,
				GenerateBatchCreate: false,
				GenerateBatchDelete: false,
				GenerateBatchUpdate: false,
				HookShouldAddModel: func(model gbgen.SchemaModel) bool {
					if model.Name == "Config" {
						return false
					}
					return true
				},
				HookChangeFields: func(model *gbgen.SchemaModel, fields []*gbgen.SchemaField, parenType gbgen.ParentType) []*gbgen.SchemaField {
					//profile: UserPayload! @isAuthenticated

					return fields
				},
				HookChangeField: func(model *gbgen.SchemaModel, field *gbgen.SchemaField) {
					//"userId", "userOrganizationId",
					if field.Name == "userId" && model.Name != "UserUserOrganization" {
						field.SkipInput = true
					}
					if field.Name == "userOrganizationId" && model.Name != "UserUserOrganization" {
						field.SkipInput = true
					}
				},
			},
			"../frontend/schema.graphql",
			gbgen.SchemaGenerateConfig{
				MergeSchema: false,
			},
		); err != nil {
			log.Fatal().Err(err).Msg("error generating schema")
		}
		generatedSchema = true
	}
	if generatedSchema {

		cfg, err := config.LoadConfigFromDefaultLocations()
		if err != nil {
			log.Fatal().Err(err).Msg("error loading config")
		}

		data, err := gbgen.NewModelPlugin().GenerateCode(cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("error generating graphql models using gqlgen")
		}

		modelCache := cache.InitializeModelCache(cfg, boilerCache, output, backend, frontend)

		if err := gbgen.NewConvertPlugin(
			modelCache,
			gbgen.ConvertPluginConfig{
				DatabaseDriver: gbgen.MySQL,
				//Searchable: {
				//	Company: {
				//		Column: dm.CompanyColumns.Name
				//	},
				//},
			},
		).GenerateCode(); err != nil {
			log.Fatal().Err(err).Msg("error while generating convert/filters")
		}

		if err := gbgen.NewResolverPlugin(
			config.ResolverConfig{
				Filename: "resolvers/all_generated_resolvers.go",
				Package:  "resolvers",
				Type:     "Resolver",
			},
			output,
			boilerCache,
			modelCache,
			gbgen.ResolverPluginConfig{

				EnableSoftDeletes: enableSoftDeletes,
				// Authorization scopes can be used to override e.g. userId, organizationId, tenantId
				// This will be resolved used the provided ScopeResolverName if the result of the AddTrigger=true
				// You would need this if you don't want to require these fields in your schema but you want to add them
				// to the db model.
				// If you do have these fields in your schema but want them authorized you could use a gqlgen directive
				AuthorizationScopes: []*gbgen.AuthorizationScope{},
				// 	{
				// 		ImportPath:        "github.com/my-repo/app/backend/auth",
				// 		ImportAlias:       "auth",
				// 		ScopeResolverName: "UserIDFromContext", // function which is called with the context of the resolver
				// 		BoilerColumnName:  "UserID",
				// 		AddHook: func(model *gbgen.BoilerModel, resolver *gbgen.Resolver, templateKey string) bool {
				// 			// fmt.Println(model.Name)
				// 			// fmt.Println(templateKey)
				// 			// templateKey contains a unique where the resolver tries to add something
				// 			// e.g.
				// 			// most of the time you can ignore this

				// 			// we want the delete call to work for every object we don't want to take in account te user-id here
				// 			if resolver.IsDelete {
				// 				return false
				// 			}

				// 			var addResolver bool
				// 			for _, field := range model.Fields {
				// 				if field.Name == "UserID" {
				// 					addResolver = true
				// 				}
				// 			}
				// 			return addResolver
				// 		},
				// 	},
				// 	{
				// 		ImportPath:        "github.com/my-repo/app/backend/auth",
				// 		ImportAlias:       "auth",
				// 		ScopeResolverName: "UserOrganizationIDFromContext", // function which is called with the context of the resolver
				// 		BoilerColumnName:  "UserOrganizationID",

				// 		AddHook: func(model *gbgen.BoilerModel, resolver *gbgen.Resolver, templateKey string) bool {
				// 			// fmt.Println(model.Name)
				// 			// fmt.Println(templateKey)
				// 			// templateKey contains a unique where the resolver tries to add something
				// 			// e.g.
				// 			// most of the time you can ignore this
				// 			var addResolver bool
				// 			for _, field := range model.Fields {
				// 				if field.Name == "UserOrganizationID" {
				// 					addResolver = true
				// 				}
				// 			}
				// 			return addResolver
				// 		},
				// 	},
				// },
			},
		).GenerateCode(data); err != nil {
			log.Fatal().Err(err).Msg("error while generating resolvers")
		}

	}
}
```


## Overriding converts
Put a file in your helpers/ directory e.g. convert_override_user.go
```golang
package helpers

import (
	"github.com/../app/backend/graphql_models"
	"github.com/../app/backend/models"
)

// use same name as in one of the generated files to override
func UserCreateInputToBoiler(
	m *graphql_models.UserCreateInput,
) *models.User {
	if m == nil {
		return nil
	}

	originalConvert := originalUserCreateInputToBoiler(m)
	// e.g. bcrypt password
	return originalConvert
}
```

If you re-generate the original convert will get changed to originalUserCreateInputToBoiler which you can still use in your overridden convert.

## Help us

We're the most happy with your time investments and/or pull request to improve this plugin. Feedback is also highly appreciated.
