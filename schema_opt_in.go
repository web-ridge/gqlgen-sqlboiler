package gbgen

import (
	"fmt"
	"sort"
	"strings"
)

// ModelExposure declares what the generated schema contains for one model when
// SchemaConfig.OptIn is set. Root operations pull in the types they need, so
// List brings its Connection, Edge, Ordering and Sort types, Create its
// CreateInput and Payload, and so on. The type flags below the operations are
// for schema written by hand that references a generated type without using
// the generated operation.
type ModelExposure struct {
	// Single generates `model(id: ID!): Model!`.
	Single bool
	// List generates the paginated `models(first, after, ...)` query.
	List bool
	// Create, Update and Delete generate the matching mutations.
	Create bool
	Update bool
	Delete bool

	// Filter generates ModelFilter (search + where) and adds the `filter`
	// argument to the list query. It implies Where.
	Filter bool
	// Ordering adds the `ordering` argument to the list query.
	Ordering bool
	// Where generates ModelWhere, also when it is only reached through the
	// relation of another Where.
	Where bool
	// WhereFields limits ModelWhere to these GraphQL field names; nil keeps
	// every field. id, or, and and withDeleted are always kept. A relation is
	// only kept when the related model has a Where as well.
	WhereFields []string

	// Connection generates ModelConnection, ModelEdge, ModelOrdering and
	// ModelSort without a list query.
	Connection bool
	// Payload generates ModelPayload, DeletePayload ModelDeletePayload.
	Payload       bool
	DeletePayload bool
	// CreateInput and UpdateInput generate the input types without the
	// mutations.
	CreateInput bool
	UpdateInput bool
}

func (e ModelExposure) hasWhere() bool         { return e.Where || e.Filter }
func (e ModelExposure) hasPagination() bool    { return e.List || e.Connection || e.Ordering }
func (e ModelExposure) hasPayload() bool       { return e.Create || e.Update || e.Payload }
func (e ModelExposure) hasDeletePayload() bool { return e.Delete || e.DeletePayload }
func (e ModelExposure) hasCreateInput() bool   { return e.Create || e.CreateInput }
func (e ModelExposure) hasUpdateInput() bool   { return e.Update || e.UpdateInput }

var fullExposure = ModelExposure{ //nolint:gochecknoglobals
	Single: true, List: true, Create: true, Update: true, Delete: true,
	Filter: true, Ordering: true, Where: true,
}

// alwaysWhereFields are needed by the generated filter code itself.
var alwaysWhereFields = map[string]bool{"id": true, "or": true, "and": true, "withDeleted": true} //nolint:gochecknoglobals

type exposure struct {
	optIn  bool
	models map[string]ModelExposure
	// enums holds the enums that a kept field still uses.
	enums map[string]bool
}

func newExposure(config SchemaConfig) exposure {
	return exposure{optIn: config.OptIn, models: config.Models, enums: map[string]bool{}}
}

func (x exposure) of(modelName string) ModelExposure {
	if !x.optIn {
		return fullExposure
	}
	return x.models[modelName]
}

func (x exposure) isExposed(modelName string) bool {
	if !x.optIn {
		return true
	}
	_, ok := x.models[modelName]
	return ok
}

// relationTarget returns the model a relation field points to, or "" for a
// plain column.
func relationTarget(field *SchemaField) string {
	if field.BoilerField == nil || !field.BoilerField.IsRelation || field.BoilerField.Relationship == nil {
		return ""
	}
	return field.BoilerField.Relationship.Name
}

// apply drops every model that is not listed, and every relation that points
// at a model that is not listed, so no generated type references a type that
// does not exist.
func (x exposure) apply(models []*SchemaModel) []*SchemaModel {
	if !x.optIn {
		return models
	}
	var kept []*SchemaModel
	for _, m := range models {
		if !x.isExposed(m.Name) {
			continue
		}
		var fields []*SchemaField
		for _, f := range m.Fields {
			if target := relationTarget(f); target != "" && !x.isExposed(target) {
				continue
			}
			fields = append(fields, f)
			if f.BoilerField != nil && f.BoilerField.IsEnum {
				x.enums[f.BoilerField.Enum.Name] = true
			}
		}
		m.Fields = fields
		kept = append(kept, m)
	}
	return kept
}

func (x exposure) keepWhereField(model *SchemaModel, field *SchemaField) bool {
	if !x.optIn {
		return true
	}
	if target := relationTarget(field); target != "" && !x.of(target).hasWhere() {
		return false
	}
	allowed := x.of(model.Name).WhereFields
	if allowed == nil || alwaysWhereFields[field.Name] {
		return true
	}
	name := field.Name
	if relationTarget(field) != "" {
		name = getRelationName(field)
	}
	for _, a := range allowed {
		if a == name {
			return true
		}
	}
	return false
}

func (x exposure) keepEnum(enumName string) bool {
	return !x.optIn || x.enums[enumName]
}

// validate reports models and Where fields in the allowlist that the database
// does not have, which are almost always typos.
func (x exposure) validate(models []*SchemaModel) error {
	if !x.optIn {
		return nil
	}
	byName := map[string]*SchemaModel{}
	for _, m := range models {
		byName[m.Name] = m
	}
	var problems []string
	for name, e := range x.models {
		m, ok := byName[name]
		if !ok {
			problems = append(problems, "unknown model "+name)
			continue
		}
		if e.WhereFields == nil {
			continue
		}
		known := map[string]bool{}
		for _, f := range m.Fields {
			known[f.Name] = true
			if relationTarget(f) != "" {
				known[getRelationName(f)] = true
			}
		}
		for _, f := range e.WhereFields {
			if !known[f] {
				problems = append(problems, fmt.Sprintf("unknown where field %v.%v", name, f))
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("opt-in schema config: %v", strings.Join(problems, ", "))
}
