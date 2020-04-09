package resolvergen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/web-ridge/gqlgen-sqlboiler/boiler"
	"github.com/web-ridge/gqlgen-sqlboiler/convert"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	pluralize "github.com/gertd/go-pluralize"
	"github.com/pkg/errors"
)

var pathRegex *regexp.Regexp
var pluralizer *pluralize.Client

func init() {
	var initError error
	pluralizer = pluralize.NewClient()
	pathRegex, initError = regexp.Compile(`src\/(.*)`)
	if initError != nil {
		fmt.Println("could not compile the path regex")
	}
}

func New(convertHelpersDir, backendModelsPath, frontendModelsPath string, authImport string) plugin.Plugin {
	return &Plugin{convertHelpersDir: convertHelpersDir, backendModelsPath: backendModelsPath, frontendModelsPath: frontendModelsPath, authImport: authImport}
}

type Plugin struct {
	convertHelpersDir  string
	backendModelsPath  string
	frontendModelsPath string
	authImport         string
}

var _ plugin.CodeGenerator = &Plugin{}

func (m *Plugin) Name() string {
	return "resolvergen"
}

func (m *Plugin) GenerateCode(data *codegen.Data) error {
	if !data.Config.Resolver.IsDefined() {
		return nil
	}

	// Get all models information
	fmt.Println("[resolver] get boiler models")
	boilerModels := boiler.GetBoilerModels(m.backendModelsPath)

	fmt.Println("[resolver] get models with information")
	models := convert.GetModelsWithInformation(data.Config, boilerModels)

	fmt.Println("[resolver] generate file")
	switch data.Config.Resolver.Layout {
	case config.LayoutSingleFile:
		return m.generateSingleFile(data, models, boilerModels)
	case config.LayoutFollowSchema:
		return m.generatePerSchema(data, models, boilerModels)
	}
	fmt.Println("[resolver] generated files")

	return nil
}

func (m *Plugin) generateSingleFile(data *codegen.Data, models []*convert.Model, boilerModels []*boiler.BoilerModel) error {
	file := File{}

	file.imports = append(file.imports, Import{
		Alias:      ".",
		ImportPath: getGoImportFromFile(m.convertHelpersDir),
	})

	file.imports = append(file.imports, Import{
		Alias:      "dm",
		ImportPath: getGoImportFromFile(m.backendModelsPath),
	})
	file.imports = append(file.imports, Import{
		Alias:      "fm",
		ImportPath: getGoImportFromFile(m.frontendModelsPath),
	})

	if m.authImport != "" {
		file.imports = append(file.imports, Import{
			Alias:      "auth",
			ImportPath: m.authImport,
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
			if resolver.Model.BoilerModel.Name != "" {
				file.Resolvers = append(file.Resolvers, resolver)
			} else {
				fmt.Println("Skipping resolver since no model found: ", resolver.Object.Name, resolver.Field.GoFieldName)
			}
		}
	}

	resolverBuild := &ResolverBuild{
		File:         &file,
		PackageName:  data.Config.Resolver.Package,
		ResolverType: data.Config.Resolver.Type,
		HasRoot:      true,
	}

	return templates.Render(templates.Options{
		PackageName: data.Config.Resolver.Package,
		PackageDoc:  `// THIS CODE IS A STARTING POINT ONLY. IT WILL NOT BE UPDATED WITH SCHEMA CHANGES.`,
		Filename:    data.Config.Resolver.Filename,
		Data:        resolverBuild,
		Packages:    data.Config.Packages,
	})
}

func (m *Plugin) generatePerSchema(data *codegen.Data, models []*convert.Model, boilerModels []*boiler.BoilerModel) error {
	rewriter, err := NewRewriter(data.Config.Resolver.ImportPath())
	if err != nil {
		return err
	}

	files := map[string]*File{}

	for _, o := range data.Objects {
		if o.HasResolvers() {
			fn := gqlToResolverName(data.Config.Resolver.Dir(), o.Position.Src.Name)
			if files[fn] == nil {
				files[fn] = &File{}
			}

			rewriter.MarkStructCopied(templates.LcFirst(o.Name) + templates.UcFirst(data.Config.Resolver.Type))
			rewriter.GetMethodBody(data.Config.Resolver.Type, o.Name)
			files[fn].Objects = append(files[fn].Objects, o)
		}
		for _, f := range o.Fields {
			if !f.IsResolver {
				continue
			}

			structName := templates.LcFirst(o.Name) + templates.UcFirst(data.Config.Resolver.Type)
			implementation := strings.TrimSpace(rewriter.GetMethodBody(structName, f.GoFieldName))
			// enhanceResolverWithBools(f)
			if implementation == "" {
				implementation = `panic(fmt.Errorf("not implemented"))`
			}

			resolver := &Resolver{
				Object:         o,
				Field:          f,
				Implementation: implementation,
			}
			fn := gqlToResolverName(data.Config.Resolver.Dir(), f.Position.Src.Name)
			if files[fn] == nil {
				files[fn] = &File{}
			}

			files[fn].Resolvers = append(files[fn].Resolvers, resolver)
		}
	}

	for filename, file := range files {
		file.imports = rewriter.ExistingImports(filename)
		file.RemainingSource = rewriter.RemainingSource(filename)
	}

	for filename, file := range files {
		resolverBuild := &ResolverBuild{
			File:         file,
			PackageName:  data.Config.Resolver.Package,
			ResolverType: data.Config.Resolver.Type,
		}

		err := templates.Render(templates.Options{
			PackageName: data.Config.Resolver.Package,
			PackageDoc: `
				// This file will be automatically regenerated based on the schema, any resolver implementations
				// will be copied through when generating and any unknown code will be moved to the end.`,
			Filename: filename,
			Data:     resolverBuild,
			Packages: data.Config.Packages,
		})
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(data.Config.Resolver.Filename); os.IsNotExist(errors.Cause(err)) {
		err := templates.Render(templates.Options{
			PackageName: data.Config.Resolver.Package,
			PackageDoc: `
				// This file will not be regenerated automatically.
				//
				// It serves as dependency injection for your app, add any dependencies you require here.`,
			Template: `type {{.}} struct {}`,
			Filename: data.Config.Resolver.Filename,
			Data:     data.Config.Resolver.Type,
			Packages: data.Config.Packages,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type ResolverBuild struct {
	*File
	HasRoot      bool
	PackageName  string
	ResolverType string
}

type File struct {
	// These are separated because the type definition of the resolver object may live in a different file from the
	//resolver method implementations, for example when extending a type in a different graphql schema file
	Objects         []*codegen.Object
	Resolvers       []*Resolver
	imports         []Import
	RemainingSource string
}

func (f *File) Imports() string {
	for _, imp := range f.imports {
		if imp.Alias == "" {
			_, _ = templates.CurrentImports.Reserve(imp.ImportPath)
		} else {
			_, _ = templates.CurrentImports.Reserve(imp.ImportPath, imp.Alias)
		}
	}
	return ""
}

type Resolver struct {
	Object                    *codegen.Object
	Field                     *codegen.Field
	Implementation            string
	IsSingle                  bool
	IsList                    bool
	IsCreate                  bool
	IsUpdate                  bool
	IsDelete                  bool
	IsBatchCreate             bool
	IsBatchUpdate             bool
	IsBatchDelete             bool
	BoilerWhiteList           string
	ResolveOrganizationID     bool
	ResolveUserOrganizationID bool
	ResolveUserID             bool
	Model                     convert.Model
	InputModel                convert.Model
}

func gqlToResolverName(base string, gqlname string) string {
	gqlname = filepath.Base(gqlname)
	ext := filepath.Ext(gqlname)
	return filepath.Join(base, strings.TrimSuffix(gqlname, ext)+".resolvers.go")
}

func hasBoilerField(boilerFields []*boiler.BoilerField, fieldName string) bool {
	for _, boilerField := range boilerFields {
		if boilerField.Name == fieldName {
			return true
		}
	}
	return false
}

func enhanceResolver(r *Resolver, models []*convert.Model) {
	nameOfResolver := r.Field.GoFieldName

	// get model names + model convert information
	modelName, inputModelName := getModelNames(nameOfResolver, false)
	// modelPluralName, _ := getModelNames(nameOfResolver, true)

	model := findModel(models, modelName)
	inputModel := findModel(models, inputModelName)

	// save for later inside file
	r.Model = model
	r.InputModel = inputModel

	if r.Object.Name == "Mutation" {

		r.IsCreate = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Create")
		r.IsUpdate = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Update")
		r.IsDelete = containsPrefixAndPartAfterThatIsSingle(nameOfResolver, "Delete")

		r.IsBatchCreate = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Create")
		r.IsBatchUpdate = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Update")
		r.IsBatchDelete = containsPrefixAndPartAfterThatIsPlural(nameOfResolver, "Delete")
	} else if r.Object.Name == "Query" {

		r.IsList = pluralizer.IsPlural(nameOfResolver)
		r.IsSingle = !r.IsList
	} else {
		fmt.Println("[WARN] Only Query and Mutation are handled we don't recognize the following: ", r.Object.Name)
	}
}

func findModel(models []*convert.Model, modelName string) convert.Model {
	if modelName == "" {
		return convert.Model{}
	}

	for _, m := range models {
		if m.Name == modelName {
			return *m
		}
	}

	return convert.Model{}
}

var InputTypes = []string{"Create", "Update", "Delete"}

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

func getGoImportFromFile(dir string) string {
	longPath, err := filepath.Abs(dir)
	if err != nil {
		fmt.Println("error while trying to convert folder to gopath", err)
	}
	// src/Users/richardlindhout/go/src/gitlab.com/eyeontarget/app/backend/graphql_models
	return strings.TrimPrefix(pathRegex.FindString(longPath), "src/")
}
