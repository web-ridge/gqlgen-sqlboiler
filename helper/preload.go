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
	preloads := GetPreloadsFromContext(ctx)
	for _, preload := range preloads {
		dbPreloads := []string{}
		columnSetting, ok := preloadColumnMap[preload]
		if ok {
			if columnSetting.IDAvailable {
				if PreloadsContainMoreThanId(preloads, preload) {
					dbPreloads = append(dbPreloads, columnSetting.Name)
				}
			} else {
				dbPreloads = append(dbPreloads, columnSetting.Name)
			}
		}
		if len(dbPreloads) > 0 {
			queryMods = append(queryMods, qm.Load(strings.Join(dbPreloads, ".")))
		}
	}
	return
}

func GetPreloadsFromContext(ctx context.Context) []string {
	return StripPreloads(GetNestedPreloads(
		graphql.GetRequestContext(ctx),
		graphql.CollectFieldsCtx(ctx, nil),
		"",
	), GetPreloadLevelFromContext(ctx))
}

// A private key for context that only this package can access. This is important
// to prevent collisions between different context uses
var inputCtxKey = &contextKey{"inputLevelWebRidge"}

type contextKey struct {
	name string
}

// GetInputLevelFromContext finds the user from the context. REQUIRES Middleware to have run.
func GetPreloadLevelFromContext(ctx context.Context) string {
	level, _ := ctx.Value(inputCtxKey).(string)
	return level
}

// GetLevelFromContext finds the user from the context. REQUIRES Middleware to have run.
func ContextWithPreloadLevel(ctx context.Context, level string) context.Context {
	return context.WithValue(ctx, inputCtxKey, level)
}

// e.g. sometimes input is deeper and we want
// createdFlowBlock.block.blockChoice => when we fetch block in database we want to strip flowBlock
func StripPreloads(preloads []string, prefix string) []string {
	if prefix == "" {
		return preloads
	}
	for i, preload := range preloads {
		preloads[i] = strings.TrimPrefix(preload, prefix+".")
	}
	return preloads
}

func GetNestedPreloads(ctx *graphql.RequestContext, fields []graphql.CollectedField, prefix string) (preloads []string) {
	for _, column := range fields {
		prefixColumn := GetPreloadString(prefix, column.Name)
		preloads = append(preloads, prefixColumn)
		preloads = append(preloads, GetNestedPreloads(ctx, graphql.CollectFields(ctx, column.SelectionSet, nil), prefixColumn)...)
		preloads = append(preloads, GetNestedPreloads(ctx, graphql.CollectFields(ctx, column.Selections, nil), prefixColumn)...)
	}
	return
}

func GetPreloadString(prefix, name string) string {
	if len(prefix) > 0 {
		return prefix + "." + name
	}
	return name
}
