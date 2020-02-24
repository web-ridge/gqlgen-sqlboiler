package resolvergen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	pluralize "github.com/gertd/go-pluralize"
	"github.com/pkg/errors"
	"github.com/web-ridge/gqlgen-sqlboiler/boiler"
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

func New(convertHelpersDir, backendModelsPath, frontendModelsPath string) plugin.Plugin {
	return &Plugin{convertHelpersDir: convertHelpersDir, backendModelsPath: backendModelsPath, frontendModelsPath: frontendModelsPath}
}

type Plugin struct {
	convertHelpersDir  string
	backendModelsPath  string
	frontendModelsPath string
}

var _ plugin.CodeGenerator = &Plugin{}

func (m *Plugin) Name() string {
	return "resolvergen"
}

func (m *Plugin) GenerateCode(data *codegen.Data) error {
	if !data.Config.Resolver.IsDefined() {
		return nil
	}

	switch data.Config.Resolver.Layout {
	case config.LayoutSingleFile:
		return m.generateSingleFile(data)
	case config.LayoutFollowSchema:
		return m.generatePerSchema(data)
	}

	return nil
}

func (m *Plugin) generateSingleFile(data *codegen.Data) error {
	file := File{}

	// if _, err := os.Stat(data.Config.Resolver.Filename); err == nil {
	// 	// file already exists and we dont support updating resolvers with layout = single so just return
	// 	return nil
	// }
	boilerTypeMap, _, _ := boiler.ParseBoilerFile(m.backendModelsPath)

	file.imports = append(file.imports, Import{
		Alias:      "helpers",
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
			enhanceResolver(resolver, boilerTypeMap)
			file.Resolvers = append(file.Resolvers, resolver)
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

func (m *Plugin) generatePerSchema(data *codegen.Data) error {
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
	Object                *codegen.Object
	Field                 *codegen.Field
	Implementation        string
	IsSingle              bool
	IsList                bool
	IsCreate              bool
	IsUpdate              bool
	IsDelete              bool
	IsBatchCreate         bool
	IsBatchUpdate         bool
	IsBatchDelete         bool
	ModelName             string
	PluralModelName       string
	HasOrganizationID     bool
	HasUserOrganizationID bool
	HasUserID             bool
	BoilerWhiteList       string
}

func gqlToResolverName(base string, gqlname string) string {
	gqlname = filepath.Base(gqlname)
	ext := filepath.Ext(gqlname)

	return filepath.Join(base, strings.TrimSuffix(gqlname, ext)+".resolvers.go")
}

func hasField(boilerTypeMap map[string]string, modelName, fieldName string) bool {
	k := modelName + "." + fieldName
	// fmt.Println("try to get boiler type from map with key: ", k)
	_, ok := boilerTypeMap[k]

	return ok
}

func enhanceResolver(r *Resolver, boilerTypeMap map[string]string) {
	nameOfResolver := r.Field.GoFieldName

	r.ModelName = getModelName(nameOfResolver)
	r.PluralModelName = getModelNamePlural(nameOfResolver)
	if hasField(boilerTypeMap, r.ModelName, "OrganizationID") {
		r.HasOrganizationID = true
		r.BoilerWhiteList += fmt.Sprintf(", dm.%vColumns.OrganizationID", r.ModelName)
	}
	if hasField(boilerTypeMap, r.ModelName, "UserOrganizationID") {
		r.HasUserOrganizationID = true
		r.BoilerWhiteList += fmt.Sprintf(", dm.%vColumns.UserOrganizationID", r.ModelName)
	}
	if hasField(boilerTypeMap, r.ModelName, "UserID") {
		r.HasUserID = true
		r.BoilerWhiteList += fmt.Sprintf(", dm.%vColumns.UserID", r.ModelName)
	}

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

var stripPrefixes = []string{"Create", "Update", "Delete"}

func getModelName(v string) string {
	for _, prefix := range stripPrefixes {
		v = strings.TrimPrefix(v, prefix)
	}
	return pluralizer.Singular(v)
}

func getModelNamePlural(v string) string {
	for _, prefix := range stripPrefixes {
		v = strings.TrimPrefix(v, prefix)
	}
	return pluralizer.Plural(v)
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
