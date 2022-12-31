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
Generate database structs with: [volatiletech/sqlboiler](https://github.com/volatiletech/sqlboiler)    

```sh 
sqlboiler mysql --no-back-referencing
```

### Step 2
Make sure you have [followed the prerequisites](https://github.com/web-ridge/gqlgen-sqlboiler#prerequisites)   
Generate schema, converts and resolvers  
```sh 
go run convert_plugin.go
```

See [example of `convert_plugin.go`](https://github.com/web-ridge/gqlgen-sqlboiler#convert_plugingo)   
   
   

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

### convert_plugin.go
Put something like the code below in file convert_plugin.go

```go
// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	gbgen "github.com/web-ridge/gqlgen-sqlboiler/v3"
)

func main() {
	output := gbgen.Config{
		Directory:   "helpers", // supports root or sub directories
		PackageName: "helpers",
	}
	backend := gbgen.Config{
		Directory:   "structs",
		PackageName: "structs",
	}
	frontend := gbgen.Config{
		Directory:   "graphql_models",
		PackageName: "graphql_models",
	}

	if err := gbgen.SchemaWrite(gbgen.SchemaConfig{
		BoilerModelDirectory: backend,
		// Directives:           []string{"IsAuthenticated"},
		// GenerateBatchCreate:  false, // Not implemented yet
		GenerateMutations:    true,
		GenerateBatchDelete:  true,
		GenerateBatchUpdate:  true,
	}, "schema.graphql", gbgen.SchemaGenerateConfig{
		MergeSchema: false, // uses three way merge to keep your customization
	}); err != nil {
		fmt.Println("error while trying to generate schema.graphql")
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}

	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	if err = api.Generate(cfg,
		api.AddPlugin(gbgen.NewConvertPlugin(
			output,   // directory where convert.go, convert_input.go and preload.go should live
			backend,  // directory where sqlboiler files are put
			frontend, // directory where gqlgen structs live
			gbgen.ConvertPluginConfig{
				DatabaseDriver: gbgen.MySQL, // or gbgen.PostgreSQL,
			},
		)),
		api.AddPlugin(gbgen.NewResolverPlugin(
			output,
			backend,
			frontend,
			gbgen.ResolverPluginConfig{
                   // See example for AuthorizationScopes here: https://github.com/web-ridge/gqlgen-sqlboiler-examples/blob/main/social-network/convert_plugin.go#L66
                },
		)),
	); err != nil {
		fmt.Println("error while trying generate resolver and converts")
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
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

If you don't have time or knowledge to contribute and we did save you a lot of time, please consider a donation so we can invest more time in this library. 
   
[![paypal](https://www.paypalobjects.com/en_US/i/btn/btn_donate_LG.gif)](https://www.paypal.com/cgi-bin/webscr?cmd=_s-xclick&hosted_button_id=7B9KKQLXTEW9Q&source=url)
