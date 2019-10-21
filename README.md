Put this in your program

Run normal generator
`go run github.com/99designs/gqlgen -v`

```golang
// +build ignore

// +build ignore

package main

import (
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	cm "github.com/web-ridge/gqlgen-sqlboiler/modelgen"
)

func main() {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load config", err.Error())
		os.Exit(2)
	}

	fmt.Println()

	err = api.Generate(cfg,
		api.AddPlugin(cm.New(
			"convert/convert.go",
			"models",
			"graphql_models",
		)), // This is the magic line
	)
	if err != nil {
		fmt.Println("error!!")
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(3)
	}
}


```

`go run custom_plugin_name.go`
