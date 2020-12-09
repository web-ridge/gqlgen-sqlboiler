package templates

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"text/template"

	"golang.org/x/tools/imports"

	gqlgenTemplates "github.com/99designs/gqlgen/codegen/templates"
)

type Options struct {
	// PackageName is a helper that specifies the package header declaration.
	// In other words, when you write the template you don't need to specify `package X`
	// at the top of the file. By providing PackageName in the Options, the Render
	// function will do that for you.
	PackageName string
	// Template is a string of the entire template that
	// will be parsed and rendered. If it's empty,
	// the plugin processor will look for .gotpl files
	// in the same directory of where you wrote the plugin.
	Template string

	// Data will be passed to the template execution.
	Data interface{}
}

func WriteTemplateFile(fileName string, cfg Options) error {
	content, contentError := GetTemplateContent(cfg)
	importFixedContent, importsError := imports.Process(fileName, []byte(content), nil)
	writeError := ioutil.WriteFile(fileName, importFixedContent, 0o600)

	if contentError != nil || writeError != nil || importsError != nil {
		return fmt.Errorf("errors while writing template to %v writeError: %v, contentError: %v, importError: %v", fileName, writeError, contentError, importsError)
	}

	return nil
}

func GetTemplateContent(cfg Options) (string, error) {
	tpl, err := template.New("").Funcs(template.FuncMap{
		"go":      gqlgenTemplates.ToGo,
		"lcFirst": gqlgenTemplates.LcFirst,
		"ucFirst": gqlgenTemplates.UcFirst,
	}).Parse(cfg.Template)
	if err != nil {
		return "", fmt.Errorf("parse: %v", err)
	}

	var content bytes.Buffer
	err = tpl.Execute(&content, cfg.Data)
	if err != nil {
		return "", fmt.Errorf("execute: %v", err)
	}

	contentBytes := content.Bytes()
	formattedContent, err := format.Source(contentBytes)
	if err != nil {
		return string(contentBytes), fmt.Errorf("formatting: %v", err)
	}

	return string(formattedContent), nil
}
