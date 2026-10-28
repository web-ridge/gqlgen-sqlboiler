package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTemplateFileDotImportsOnlyWhatItUses(t *testing.T) {
	support := DotImport{Path: "example.com/app/support", Names: []string{"ContactSearchToMods"}}
	tests := []struct {
		name     string
		template string
		want     bool
	}{
		{"calls a support function", "func ContactFilterToMods() { ContactSearchToMods() }", true},
		{"calls nothing from support", "func ContactFilterToMods() {}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "generated.go")
			_ = WriteTemplateFile(file, Options{PackageName: "fmconv", Template: "package fmconv\n" + tt.template, DotImports: []DotImport{support}})
			written, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(written), `. "example.com/app/support"`); got != tt.want {
				t.Errorf("dot import present = %v, want %v:\n%s", got, tt.want, written)
			}
		})
	}
}
