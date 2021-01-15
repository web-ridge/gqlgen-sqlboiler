package gbgen

import (
	"fmt"
	"path"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/iancoleman/strcase"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/plugin"
	"github.com/web-ridge/gqlgen-sqlboiler/v3/templates"
)

func NewResolverPlugin(output, backend, frontend Config, resolverPluginConfig ResolverPluginConfig) plugin.Plugin {
	return &ResolverPlugin{
		output:         output,
		backend:        backend,
		frontend:       frontend,
		pluginConfig:   resolverPluginConfig,
		rootImportPath: getRootImportPath(),
	}
}

type AuthorizationScope struct {
	ImportPath        string
	ImportAlias       string
	ScopeResolverName string
	BoilerColumnName  string
	AddHook           func(model *BoilerModel, resolver *Resolver, templateKey string) bool
}

type ResolverPluginConfig struct {
	AuthorizationScopes []*AuthorizationScope
}

type ResolverPlugin struct {
	output         Config
	backend        Config
	frontend       Config
	pluginConfig   ResolverPluginConfig
	rootImportPath string
}

var _ plugin.CodeGenerator = &ResolverPlugin{}

func (m *ResolverPlugin) Name() string {
	return "resolver-webridge"
}

func (m *ResolverPlugin) GenerateCode(data *codegen.Data) error {
	if !data.Config.Resolver.IsDefined() {
		return nil
	}

	// gqlgenTemplates.CurrentImports = &gqlgenTemplates.Imports{}

	// Get all models information
	log.Debug().Msg("[resolver] get boiler models")
	boilerModels := GetBoilerModels(m.backend.Directory)
	log.Debug().Msg("[resolver] get models with information")
	models := GetModelsWithInformation(m.backend, nil, data.Config, boilerModels)
	log.Debug().Msg("[resolver] generate file")
	// switch data.Config.Resolver.Layout {
	// case config.LayoutSingleFile:
	err := m.generateSingleFile(data, models, boilerModels)
	return err
	//case config.LayoutFollowSchema:
	//	return m.generatePerSchema(data, models, boilerModels)
	//}
	//log.Debug().Msg("[resolver] generated files")
	//return nil
}

func (m *ResolverPlugin) generateSingleFile(data *codegen.Data, models []*Model, _ []*BoilerModel) error {
	file := File{}

	file.Imports = append(file.Imports, Import{
		Alias:      ".",
		ImportPath: path.Join(m.rootImportPath, m.output.Directory),
	})

	file.Imports = append(file.Imports, Import{
		Alias:      "dm",
		ImportPath: path.Join(m.rootImportPath, m.backend.Directory),
	})

	file.Imports = append(file.Imports, Import{
		Alias:      "fm",
		ImportPath: path.Join(m.rootImportPath, m.frontend.Directory),	
	})

	file.Imports = append(file.Imports, Import{
		Alias:      "gm",
		ImportPath: buildImportPath(m.rootImportPath, data.Config.Exec.ImportPath()),
	})

	for _, scope := range m.pluginConfig.AuthorizationScopes {
		file.Imports = append(file.Imports, Import{
			Alias:      scope.ImportAlias,
			ImportPath: scope.ImportPath,
		})
	}

	for _, o := range data.Objects {
		if o.HasResolvers() {
			file.Objects = append(file.Objects, o)
		}
		for _, f := range o.Fields {
			if !f.IsResolver {
				continue
			}
			resolver := &Resolver{
				Object:         o,
				Field:          f,
				Implementation: `panic("not implemented yet")`,
			}
			enhanceResolver(resolver, models)
			if resolver.Model.BoilerModel != nil && resolver.Model.BoilerModel.Name != "" {
				file.Resolvers = append(file.Resolvers, resolver)
			} else if resolver.Field.GoFieldName != "Node" {
				log.Debug().Str("resolver", resolver.Object.Name).Str("field", resolver.Field.GoFieldName).Msg(
					"skipping resolver since no model found")
			}
		}
	}

	resolverBuild := &ResolverBuild{
		File:                &file,
		PackageName:         data.Config.Resolver.Package,
		ResolverType:        data.Config.Resolver.Type,
		HasRoot:             true,
		Models:              models,
		AuthorizationScopes: m.pluginConfig.AuthorizationScopes,
	}

	templateName := "resolver.gotpl"
	templateContent, err := getTemplateContent(templateName)
	if err != nil {
		log.Err(err).Msg("error when reading " + templateName)
		return err
	}

	return templates.WriteTemplateFile(data.Config.Resolver.Filename, templates.Options{
		Template:    templateContent,
		PackageName: data.Config.Resolver.Package,
		Data:        resolverBuild,
	})
}

func buildImportPath(rootImportPath, directory string) string {
	index := strings.Index(directory, rootImportPath)
	if index > 0 {
		return directory[index:]
	}
	return directory
}

type ResolverBuild struct {
	*File
	HasRoot             bool
	PackageName         string
	ResolverType        string
	Models              []*Model
	AuthorizationScopes []*AuthorizationScope
	TryHook             func(string) bool
}

type File struct {
	// These are separated because the type definition of the resolver object may live in a different file from the
	// resolver method implementations, for example when extending a type in a different graphql schema file
	Objects         []*codegen.Object
	Resolvers       []*Resolver
	Imports         []Import
	RemainingSource string
}

type Resolver struct {
	Object *codegen.Object
	Field  *codegen.Field

	Implementation            string
	IsSingle                  bool
	IsList                    bool
	IsListForward             bool
	IsListBackward            bool
	IsCreate                  bool
	IsUpdate                  bool
	IsDelete                  bool
	IsBatchCreate             bool
	IsBatchUpdate             bool
	IsBatchDelete             bool
	ResolveOrganizationID     bool // TODO: something more pluggable
	ResolveUserOrganizationID bool // TODO: something more pluggable
	ResolveUserID             bool // TODO: something more pluggable
	Model                     Model
	InputModel                Model
	BoilerWhiteList           string
	PublicErrorKey            string
	PublicErrorMessage        string
}

func (rb *ResolverBuild) getResolverType(ty string) string {
	for _, imp := range rb.Imports {
		if strings.Contains(ty, imp.ImportPath) {
			if imp.Alias != "" {
				ty = strings.Replace(ty, imp.ImportPath, imp.Alias, -1)
			} else {
				ty = strings.Replace(ty, imp.ImportPath, "", -1)
			}
		}
	}
	return ty
}

func (rb *ResolverBuild) ShortResolverDeclaration(r *Resolver) string {
	res := "(ctx context.Context"

	if !r.Field.Object.Root {
		res += fmt.Sprintf(", obj %s", rb.getResolverType(r.Field.Object.Reference().String()))
	}
	for _, arg := range r.Field.Args {
		res += fmt.Sprintf(", %s %s", arg.VarName, rb.getResolverType(arg.TypeReference.GO.String()))
	}

	result := rb.getResolverType(r.Field.TypeReference.GO.String())
	if r.Field.Object.Stream {
		result = "<-chan " + result
	}

	res += fmt.Sprintf(") (%s, error)", result)
	return res
}

func enhanceResolver(r *Resolver, models []*Model) { //nolint:gocyclo
	nameOfResolver := r.Field.GoFieldName

	// get model names + model convert information
	modelName, inputModelName := getModelNames(nameOfResolver, false)
	// modelPluralName, _ := getModelNames(nameOfResolver, true)

	model := findModelOrEmpty(models, modelName)
	inputModel := findModelOrEmpty(models, inputModelName)

	// save for later inside file
	r.Model = model
	r.InputModel = inputModel

	switch r.Object.Name {
	case "Mutation":
		r.IsCreate = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Create")
		r.IsUpdate = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Update")
		r.IsDelete = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Delete")
		r.IsBatchCreate = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Create")
		r.IsBatchUpdate = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Update")
		r.IsBatchDelete = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Delete")
	case "Query":
		isPlural := pluralizer.IsPlural(nameOfResolver)
		if isPlural {
			r.IsList = isPlural
			r.IsListBackward = strings.Contains(r.Field.GoFieldName, "first int") &&
				strings.Contains(r.Field.GoFieldName, "after *string")
			r.IsListBackward = strings.Contains(r.Field.GoFieldName, "last int") &&
				strings.Contains(r.Field.GoFieldName, "before *string")
		}

		r.IsSingle = !r.IsList
	default:
		log.Warn().Str("unknown", r.Object.Name).Msg(
			"only Query and Mutation are handled we don't recognize the following")
	}

	lmName := strcase.ToLowerCamel(model.Name)
	lmpName := strcase.ToLowerCamel(model.PluralName)
	r.PublicErrorKey = "public"

	if (r.IsCreate || r.IsDelete || r.IsUpdate) && strings.HasSuffix(lmName, "Batch") {
		r.PublicErrorKey += "One"
	}
	r.PublicErrorKey += model.Name

	switch {
	case r.IsSingle:
		r.PublicErrorKey += "Single"
		r.PublicErrorMessage = "could not get " + lmName
	case r.IsList:
		r.PublicErrorKey += "List"
		r.PublicErrorMessage = "could not list " + lmpName
	case r.IsCreate:
		r.PublicErrorKey += "Create"
		r.PublicErrorMessage = "could not create " + lmName
	case r.IsUpdate:
		r.PublicErrorKey += "Update"
		r.PublicErrorMessage = "could not update " + lmName
	case r.IsDelete:
		r.PublicErrorKey += "Delete"
		r.PublicErrorMessage = "could not delete " + lmName
	case r.IsBatchCreate:
		r.PublicErrorKey += "BatchCreate"
		r.PublicErrorMessage = "could not create " + lmpName
	case r.IsBatchUpdate:
		r.PublicErrorKey += "BatchUpdate"
		r.PublicErrorMessage = "could not update " + lmpName
	case r.IsBatchDelete:
		r.PublicErrorKey += "BatchDelete"
		r.PublicErrorMessage = "could not delete " + lmpName
	}

	r.PublicErrorKey += "Error"
}

func findModelOrEmpty(models []*Model, modelName string) Model {
	if modelName == "" {
		return Model{}
	}
	for _, m := range models {
		if m.Name == modelName {
			return *m
		}
	}
	return Model{}
}

var InputTypes = []string{"Create", "Update", "Delete"} //nolint:gochecknoglobals

func getModelNames(v string, plural bool) (modelName, inputModelName string) {
	var prefix string
	var isInputType bool
	for _, inputType := range InputTypes {
		if strings.HasPrefix(v, inputType) {
			isInputType = true
			v = strings.TrimPrefix(v, inputType)
			prefix = inputType
		}
	}
	var s string
	if plural {
		s = pluralizer.Plural(v)
	} else {
		s = pluralizer.Singular(v)
	}

	if isInputType {
		return s, s + prefix + "Input"
	}

	return s, ""
}

func containsPrefixAndPartAfterThatIsSingle(v string, prefix string) bool {
	partAfterThat := strings.TrimPrefix(v, prefix)
	return strings.HasPrefix(v, prefix) && pluralizer.IsSingular(partAfterThat)
}

func containsPrefixAndPartAfterThatIsPlural(v string, prefix string) bool {
	partAfterThat := strings.TrimPrefix(v, prefix)
	return strings.HasPrefix(v, prefix) && pluralizer.IsPlural(partAfterThat)
}
