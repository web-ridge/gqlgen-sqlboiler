package gbgen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog/log"

	"golang.org/x/mod/modfile"
)

func getRootImportPath() string {
	importPath, err := rootImportPath()
	if err != nil {
		log.Err(err).Msg(
			"could not detect root import path %v")
		return ""
	}
	return importPath
}

func rootImportPath() (string, error) {
	projectPath, err := getWorkingPath()
	if err != nil {
		// TODO: adhering to your original error handling
		//  should consider doing something here rather than continuing
		//  since this step occurs during generation, panicing or fatal error should be okay
		return "", fmt.Errorf("error while getting working directory %w", err)
	}
	if hasGoMod(projectPath) {
		modulePath, err := getModulePath(projectPath)
		if err != nil {
			// TODO: adhering to your original error handling
			//  should consider doing something here rather than continuing
			//  since this step occurs during generation, panicing or fatal error should be okay
			return "", fmt.Errorf("error while getting module path %w", err)
		}
		return modulePath, nil
	}

	return gopathImport(projectPath), nil
}

// getWorkingPath gets the current working directory
func getWorkingPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return wd, nil
}

func hasGoMod(projectPath string) bool {
	filePath := path.Join(projectPath, "go.mod")
	return fileExists(filePath)
}

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getModulePath(projectPath string) (string, error) {
	filePath := path.Join(projectPath, "go.mod")
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error while trying to read go mods path %w", err)
	}

	modPath := modfile.ModulePath(file)
	if modPath == "" {
		return "", fmt.Errorf("could not determine mod path")
	}
	return modPath, nil
}

func gopathImport(dir string) string {
	return strings.TrimPrefix(pathRegex.FindString(dir), "src/")
}
