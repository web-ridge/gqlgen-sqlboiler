/*
All code below copied from to prevent not allowed error
https://github.com/99designs/gqlgen/internal/code

Made issue to make these helper public
https://github.com/99designs/gqlgen/issues/1058


*/

package gqlgen_sqlboiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

var gopaths []string //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	gopaths = filepath.SplitList(build.Default.GOPATH)
	for i, p := range gopaths {
		gopaths[i] = filepath.ToSlash(filepath.Join(p, "src"))
	}
}

// NameForDir manually looks for package stanzas in files located in the given directory. This can be
// much faster than having to consult go list, because we already know exactly where to look.
func NameForDir(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return SanitizePackageName(filepath.Base(dir))
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return SanitizePackageName(filepath.Base(dir))
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if !strings.HasSuffix(strings.ToLower(file.Name()), ".go") {
			continue
		}

		filename := filepath.Join(dir, file.Name())
		if src, err := parser.ParseFile(fset, filename, nil, parser.PackageClauseOnly); err == nil {
			return src.Name.Name
		}
	}

	return SanitizePackageName(filepath.Base(dir))
}

var invalidPackageNameChar = regexp.MustCompile(`[^\w]`) //nolint:gochecknoglobals

func SanitizePackageName(pkg string) string {
	return invalidPackageNameChar.ReplaceAllLiteralString(filepath.Base(pkg), "_")
}

// take a string in the form github.com/package/blah.Type and split it into package and type
func PkgAndType(name string) (string, string) {
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return "", name
	}

	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

type Rewriter struct {
	pkg    *packages.Package
	files  map[string]string
	copied map[ast.Decl]bool
}

func NewRewriter(importPath string) (*Rewriter, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypes,
	}, importPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("package not found for importPath: %s", importPath)
	}

	return &Rewriter{
		pkg:    pkgs[0],
		files:  map[string]string{},
		copied: map[ast.Decl]bool{},
	}, nil
}

func (r *Rewriter) getSource(start, end token.Pos) string {
	startPos := r.pkg.Fset.Position(start)
	endPos := r.pkg.Fset.Position(end)

	if startPos.Filename != endPos.Filename {
		panic("cant get source spanning multiple files")
	}

	file := r.getFile(startPos.Filename)
	return file[startPos.Offset:endPos.Offset]
}

func (r *Rewriter) getFile(filename string) string {
	if _, ok := r.files[filename]; !ok {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			panic(fmt.Errorf("unable to load file, already exists: %s", err.Error()))
		}

		r.files[filename] = string(b)
	}

	return r.files[filename]
}

func (r *Rewriter) GetMethodBody(structname string, methodname string) string {
	for _, f := range r.pkg.Syntax {
		for _, d := range f.Decls {
			d, isFunc := d.(*ast.FuncDecl)
			if !isFunc {
				continue
			}
			if d.Name.Name != methodname {
				continue
			}
			if d.Recv == nil || len(d.Recv.List) == 0 {
				continue
			}
			recv := d.Recv.List[0].Type
			if star, isStar := recv.(*ast.StarExpr); isStar {
				recv = star.X
			}
			ident, ok := recv.(*ast.Ident)
			if !ok {
				continue
			}

			if ident.Name != structname {
				continue
			}

			r.copied[d] = true

			return r.getSource(d.Body.Pos()+1, d.Body.End()-1)
		}
	}

	return ""
}

func (r *Rewriter) MarkStructCopied(name string) {
	for _, f := range r.pkg.Syntax {
		for _, d := range f.Decls {
			d, isGen := d.(*ast.GenDecl)
			if !isGen {
				continue
			}
			if d.Tok != token.TYPE || len(d.Specs) == 0 {
				continue
			}

			spec, isTypeSpec := d.Specs[0].(*ast.TypeSpec)
			if !isTypeSpec {
				continue
			}

			if spec.Name.Name != name {
				continue
			}

			r.copied[d] = true
		}
	}
}

func (r *Rewriter) ExistingImports(filename string) []Import {
	filename, err := filepath.Abs(filename)
	if err != nil {
		panic(err)
	}
	for _, f := range r.pkg.Syntax {
		pos := r.pkg.Fset.Position(f.Pos())

		if filename != pos.Filename {
			continue
		}

		var imps []Import
		for _, i := range f.Imports {
			name := ""
			if i.Name != nil {
				name = i.Name.Name
			}
			path, err := strconv.Unquote(i.Path.Value)
			if err != nil {
				panic(err)
			}
			imps = append(imps, Import{name, path})
		}
		return imps
	}
	return nil
}

func (r *Rewriter) RemainingSource(filename string) string {
	filename, err := filepath.Abs(filename)
	if err != nil {
		panic(err)
	}
	for _, f := range r.pkg.Syntax {
		pos := r.pkg.Fset.Position(f.Pos())

		if filename != pos.Filename {
			continue
		}

		var buf bytes.Buffer

		for _, d := range f.Decls {
			if r.copied[d] {
				continue
			}

			if d, isGen := d.(*ast.GenDecl); isGen && d.Tok == token.IMPORT {
				continue
			}

			buf.WriteString(r.getSource(d.Pos(), d.End()))
			buf.WriteString("\n")
		}

		return strings.TrimSpace(buf.String())
	}
	return ""
}

type Import struct {
	Alias      string
	ImportPath string
}
