package helper

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
)

func GetInputFromContext(ctx context.Context, key string) map[string]interface{} {
	requestContext := graphql.GetRequestContext(ctx)
	m, ok := requestContext.Variables[key].(map[string]interface{})
	if !ok {
		fmt.Println("can not get input from context")
	}
	return m
}
