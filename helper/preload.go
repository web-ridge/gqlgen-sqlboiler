package helper

import (
	"context"
	"fmt"
	"strings"

	qm "github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/99designs/gqlgen/graphql"
)

type ColumnSetting struct {
	Name        string
	IDAvailable bool // ID is available without preloading
}

func PreloadsContainMoreThanId(a []string, v string) bool {
	for _, av := range a {
		if strings.HasPrefix(av, v) &&
			av != v && // e.g. parentTable
			!strings.HasPrefix(av, v+".id") { // e.g parentTable.id
			return true
		}
	}
	return false
}

func PreloadsContain(a []string, v string) bool {
	for _, av := range a {
		if av == v {
			return true
		}
	}
	return false
}

func GetPreloadMods(ctx context.Context, preloadColumnMap map[string]ColumnSetting) (queryMods []qm.QueryMod) {
	gPreloads := GetPreloads(ctx)
	for _, gPreload := range gPreloads {
		dPreloadParts := []string{}
		fmt.Println("preloadje??", gPreload)
		for _, gPreloadPart := range strings.Split(gPreload, ".") {
			columnSetting, ok := preloadColumnMap[gPreloadPart]
			if ok {
				if columnSetting.IDAvailable {
					if PreloadsContainMoreThanId(gPreloads, gPreloadPart) {
						dPreloadParts = append(dPreloadParts, columnSetting.Name)
					}
					// TODO
					// dPreloadParts = append(dPreloadParts, columSetting.Name)
				} else {
					dPreloadParts = append(dPreloadParts, columnSetting.Name)
				}
			}
			// else {
			// 	fmt.Println(gPreload, " not found s")
			// }
		}
		if len(dPreloadParts) > 0 {
			queryMods = append(queryMods, qm.Load(strings.Join(dPreloadParts, ".")))
		}
	}
	return
}

func GetPreloads(ctx context.Context) []string {
	// return

	return GetNestedPreloads(
		graphql.GetOperationContext(ctx),
		graphql.CollectFieldsCtx(ctx, nil),
		"",
	)
	// arr2 := graphql.CollectAllFields(ctx)
	// return append(arr1, arr2...)
}

// // CollectAllFields returns a slice of all GraphQL field names that were selected for the current resolver context.
// // The slice will contain the unique set of all field names requested regardless of fragment type conditions.
// func CollectAllFields(ctx context.Context) []string {
// 	resctx := graphql.GetFieldContext(ctx)
// 	collected := graphql.CollectFields(graphql.GetOperationContext(ctx), resctx.Field.Selections, nil)
// 	uniq := make([]string, 0, len(collected))

// 	for _, f := range collected {
// 		fmt.Println("CollectAllFields add", f.Name)
// 		for _, s := range f.Selections {

// 		}
// 		uniq = append(uniq, f.Name)
// 	}
// 	return uniq
// }

func GetNestedPreloads(ctx *graphql.RequestContext, fields []graphql.CollectedField, prefix string) (preloads []string) {
	for _, column := range fields {
		prefixColumn := GetPreloadString(prefix, column.Name)
		preloads = append(preloads, prefixColumn)
		preloads = append(preloads, GetNestedPreloads(ctx, graphql.CollectFields(ctx, column.SelectionSet, nil), prefixColumn)...)
	}
	return
}

func GetPreloadString(prefix, name string) string {
	if len(prefix) > 0 {
		return prefix + "." + name
	}
	return name
}
