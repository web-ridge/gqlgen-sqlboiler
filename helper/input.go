package helper

import (
	"context"
	"fmt"
	"strings"

	"github.com/99designs/gqlgen/graphql"
)

func GetInputFromContext(ctx context.Context, key string) map[string]interface{} {
	requestContext := graphql.GetOperationContext(ctx)
	variables := requestContext.Variables

	// pick nested inputs
	var ok bool
	for _, splittedKey := range strings.Split(key, ".") {
		variables, ok = variables[splittedKey].(map[string]interface{})
		if !ok {
			fmt.Println("can not get input from context for key: "+key+" splitted key: ", splittedKey)
		}
	}

	return variables
}
