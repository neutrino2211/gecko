// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md

package compiler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
)

// findTypeInDirectory searches a directory for a type definition.
// Returns the parsed file if the type is found.
func findTypeInDirectory(dirPath string, typeName string, cfg *config.CompileCfg) (*tokens.File, bool) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, false
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".gecko" {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		contents, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		file, parseErr := parser.Parser.ParseString(filePath, string(contents))
		if parseErr != nil {
			continue
		}

		// Check if this file contains the type we're looking for
		for _, e := range file.Entries {
			if e.Class != nil && e.Class.Name == typeName {
				file.Content = string(contents)
				file.Path = filePath
				file.Name = e.Class.Name
				file.Config = cfg
				return file, true
			}
			if e.Trait != nil && e.Trait.Name == typeName {
				file.Content = string(contents)
				file.Path = filePath
				file.Name = e.Trait.Name
				file.Config = cfg
				return file, true
			}
		}
	}

	return nil, false
}

// ResolveTypeFromDirectoryImports searches directory imports for a type.
// This is called when a type cannot be found in the current scope.
func ResolveTypeFromDirectoryImports(sourceFile *tokens.File, typeName string) (*tokens.File, bool) {
	return resolveTypeFromDirectoryImports(sourceFile, typeName, newCompileState())
}

func resolveTypeFromDirectoryImports(sourceFile *tokens.File, typeName string, state *compileState) (*tokens.File, bool) {
	for _, dirImport := range sourceFile.DirectoryImports {
		// If use objects specified, only allow those types
		if len(dirImport.UseObjects) > 0 {
			found := false
			for _, obj := range dirImport.UseObjects {
				if obj == typeName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Search the directory for the type
		if file, ok := findTypeInDirectory(dirImport.DirPath, typeName, sourceFile.Config); ok {
			if state != nil {
				cacheKey := "dir-type|" + dirImport.Path + "|" + typeName
				state.importCache[cacheKey] = file
			}
			appendImportIfMissing(sourceFile, file)
			return file, true
		}
	}
	return nil, false
}

// ResolveModuleTypeFromDirectoryImports searches a specific module (directory import) for a type.
// This is called for module-qualified types like shapes.Circle.
func ResolveModuleTypeFromDirectoryImports(sourceFile *tokens.File, moduleName string, typeName string) (*tokens.File, bool) {
	return resolveModuleTypeFromDirectoryImports(sourceFile, moduleName, typeName, newCompileState())
}

func resolveModuleTypeFromDirectoryImports(sourceFile *tokens.File, moduleName string, typeName string, state *compileState) (*tokens.File, bool) {
	for _, dirImport := range sourceFile.DirectoryImports {
		// Check if this directory import matches the module name
		// The module name is the last part of the import path (e.g., "shapes" from "import shapes")
		importModuleName := moduleNameFromImportPath(dirImport.Path)
		if importModuleName != moduleName {
			continue
		}

		// If use objects specified, only allow those types
		if len(dirImport.UseObjects) > 0 {
			found := false
			for _, obj := range dirImport.UseObjects {
				if obj == typeName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Search the directory for the type
		if file, ok := findTypeInDirectory(dirImport.DirPath, typeName, sourceFile.Config); ok {
			if state != nil {
				cacheKey := "dir-module-type|" + dirImport.Path + "|" + typeName
				state.importCache[cacheKey] = file
			}
			appendImportIfMissing(sourceFile, file)
			return file, true
		}
	}
	return nil, false
}

// findMethodInDirectory searches a directory for a method definition.
// Returns the parsed file if the method is found.
func findMethodInDirectory(dirPath string, methodName string, cfg *config.CompileCfg) (*tokens.File, bool) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, false
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".gecko" {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		contents, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		file, parseErr := parser.Parser.ParseString(filePath, string(contents))
		if parseErr != nil {
			continue
		}

		// Check if this file contains the method we're looking for
		for _, e := range file.Entries {
			if e.Method != nil && e.Method.Name == methodName {
				file.Content = string(contents)
				file.Path = filePath
				file.Name = e.Method.Name
				file.Config = cfg
				return file, true
			}
		}
	}

	return nil, false
}

// ResolveMethodFromDirectoryImports searches directory imports for a method.
// This is called when a method cannot be found in the current scope.
func ResolveMethodFromDirectoryImports(sourceFile *tokens.File, methodName string) (*tokens.File, bool) {
	return resolveMethodFromDirectoryImports(sourceFile, methodName, newCompileState())
}

func resolveMethodFromDirectoryImports(sourceFile *tokens.File, methodName string, state *compileState) (*tokens.File, bool) {
	for _, dirImport := range sourceFile.DirectoryImports {
		// If use objects specified, only allow those methods
		if len(dirImport.UseObjects) > 0 {
			found := false
			for _, obj := range dirImport.UseObjects {
				if obj == methodName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Search the directory for the method
		if file, ok := findMethodInDirectory(dirImport.DirPath, methodName, sourceFile.Config); ok {
			if state != nil {
				cacheKey := "dir-method|" + dirImport.Path + "|" + methodName
				state.importCache[cacheKey] = file
			}
			appendImportIfMissing(sourceFile, file)
			return file, true
		}
	}
	return nil, false
}

// preloadDirectoryUseImports eagerly resolves explicit `import dir use { ... }` symbols
// so semantic analysis can type-check those references without backend-only lazy hooks.
func preloadDirectoryUseImports(sourceFile *tokens.File, state *compileState) {
	if sourceFile == nil {
		return
	}
	for _, dirImport := range sourceFile.DirectoryImports {
		if dirImport == nil || len(dirImport.UseObjects) == 0 {
			continue
		}
		for _, obj := range dirImport.UseObjects {
			if strings.TrimSpace(obj) == "" {
				continue
			}
			if _, ok := resolveMethodFromDirectoryImports(sourceFile, obj, state); ok {
				continue
			}
			_, _ = resolveTypeFromDirectoryImports(sourceFile, obj, state)
		}
	}
}
