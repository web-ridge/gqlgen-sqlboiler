# gqlgen-sqlboiler

We want developers to be able to build software faster using modern tools like GraphQL, Golang, React Native without depending on commercial providers like Firebase or AWS Amplify.

This program generates code like this between your generated gqlgen and sqlboiler with support for Relay.dev (unique id's etc). We can automatically generate the implementation of queries and mutations like create, update, delete working based on your graphql scheme and your sqlboiler models.

To make this program a success tight coupling (same naming) between your database and graphql scheme is needed at the moment. The advantage of this program is the most when you have a database already designed. However everything is created with support for change so you could write some extra GrapQL resolvers if you'd like without a problem.

## Why gqlgen and sqlboiler
They go back to a schema first approach which we like. The generated code with these tools are the most efficient and fast in the Golang system (and probably outside of it too).
- SQLBoiler: https://github.com/volatiletech/sqlboiler#benchmarks
- GQLGen: https://github.com/appleboy/golang-graphql-benchmark#summary

It's really amazing how fast a generated api with these techniques is!


## Usage

1. Generate database structs with: https://github.com/volatiletech/sqlboiler (--no-back-referencing is IMPORTANT!)
   e.g. `sqlboiler mysql --no-back-referencing`
2. (optional, but recommended) Generate GrapQL scheme from sqlboiler structs: https://github.com/web-ridge/sqlboiler-graphql-schema  
   e.g. `go run github.com/web-ridge/sqlboiler-graphql-schema --output=schema.graphql --skip-input-fields=userId --directives=isAuthenticated --pagination=no`
3. Install: https://github.com/99designs/gqlgen
4. Generate gqlgen structs + converts between gqlgen and sqlboiler with this program  
   e.g. `go run convert_plugin.go` for file contents of that program see bottom of this readme

## Done

- [x] Generate converts between sqlboiler structs and graphql (with relations included)
- [x] Generate converts between input models and sqlboiler
- [x] Genarated code understands the difference between empty and null for update inputs so you can set things empty if you explicicitly set them in your mutation!
- [x] Fetch sqlboiler preloads from graphql context
- [x] Support for foreign keys named differently than their corresponding model
- [x] New plugin which generates CRUD resolvers based on mutations in graphql scheme.
- [x] Support one-to-one relationships inside input types.
- [x] Generate code which implements the generated where and search filters
- [x] Batch update/delete generation in resolvers (Not tested yet).
- [x] Enum support.
- [x] public errors in resolvers + logging via zerolog. (feel free for PR for configurable logging!)

## Roadmap

- [ ] Batch create generation in resolvers (have working version here for.PostgreSQL https://github.com/web-ridge/contact-tracing, need maybe different implementation for different ORM's?).
- [ ] Support gqlgen multiple .graphql files
- [ ] Edges/connections
- [ ] Generate tests
- [ ] Run automatic tests in Github CI/CD in https://github.com/web-ridge/gqlgen-sqlboiler-examples
- [ ] Crud of adding/removing relationships from many-to-many on edges.
- [ ] Support more relationships inside input types
- [ ] Do a three-way-diff merge for changes and let user choose parts of code which should not take over generated code.

## Requirements

- Use unsigned ints for foreign keys + ids. Otherwise converts will give compile errors.
  Unsigned ints for id's is allso recommended since it gives you twice as big id's and id's should not be negative anyway ;)

## Examples

https://github.com/web-ridge/gqlgen-sqlboiler-examples

More examples are welcome. Send a PR ;-)

### Example result of this plugin

https://github.com/web-ridge/gqlgen-sqlboiler-examples/tree/master/social-network/helpers

**Code snippet**

```golang
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
	gbgen "github.com/web-ridge/gqlgen-sqlboiler/v2"
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
		api.AddPlugin(gbgen.NewConvertPlugin(
			convertHelpersDir, // directory where convert.go, convert_input.go and preload.go should live
			sqlboilerDir,      // directory where sqlboiler files are put
			gqlgenModelDir,    // directory where gqlgen models live
		)),
		api.AddPlugin(gbgen.NewResolverPlugin(
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

## Donate

Did we save you a lot of time? Please consider a donation so we can invest more time in this library: [![paypal](https://www.paypalobjects.com/en_US/i/btn/btn_donate_LG.gif)](https://www.paypal.com/cgi-bin/webscr?cmd=_s-xclick&hosted_button_id=7B9KKQLXTEW9Q&source=url)

