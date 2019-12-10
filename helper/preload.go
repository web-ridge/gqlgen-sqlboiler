package helper

import (
	"context"
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
		}
		if len(dPreloadParts) > 0 {
			queryMods = append(queryMods, qm.Load(strings.Join(dPreloadParts, ".")))
		}
	}
	return
}

func GetPreloads(ctx context.Context) []string {
	return GetNestedPreloads(
		graphql.GetRequestContext(ctx),
		graphql.CollectFieldsCtx(ctx, nil),
		"",
	)
}

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
