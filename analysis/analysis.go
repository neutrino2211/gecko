// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package analysis

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

// AnalysisContext holds parsed files and resolved symbols for analysis
type AnalysisContext struct {
	MainFile      *tokens.File
	ImportedFiles map[string]*tokens.File
	RootScope     *ast.Ast
	FilePath      string
	SemanticGraph *semantic.Program
}

// NewAnalysisContext creates an analysis context for a file
func NewAnalysisContext(filePath string, content string) (*AnalysisContext, error) {
	file, err := parser.Parser.ParseString(filePath, content)
	if err != nil {
		return nil, err
	}
	file.ComputeRanges()
	file.Path = filePath
	file.Content = content

	ctx := &AnalysisContext{
		MainFile:      file,
		ImportedFiles: make(map[string]*tokens.File),
		FilePath:      filePath,
	}

	// Build AST scope for symbol resolution
	ctx.RootScope = &ast.Ast{
		Scope:      file.PackageName,
		SourceFile: filePath,
	}
	errorScope := errors.NewErrorScope("analysis", file.PackageName, content)
	ctx.RootScope.Init(errorScope)

	// Resolve imports
	ctx.resolveImports()

	// Build shared semantic graph used by compiler backends and LSP helpers.
	ctx.SemanticGraph = semantic.Analyze(ctx.MainFile)

	// Register symbols in scope
	ctx.registerSymbols()

	return ctx, nil
}

// resolveImports parses imported modules
func (ctx *AnalysisContext) resolveImports() {
	baseDir := filepath.Dir(ctx.FilePath)
	geckoHome := getGeckoHome()
	stdlibPath := filepath.Join(geckoHome, "stdlib")

	for _, entry := range ctx.MainFile.Entries {
		if entry.Import == nil {
			continue
		}

		moduleName := entry.Import.ModuleName()
		pathComponents := entry.Import.Path
		relativePath := filepath.Join(pathComponents...)

		var searchPaths []string
		if len(pathComponents) > 0 && pathComponents[0] == "std" {
			stdRelativePath := filepath.Join(pathComponents[1:]...)
			searchPaths = []string{stdlibPath}
			relativePath = stdRelativePath
		} else {
			searchPaths = []string{baseDir}
		}

		for _, searchPath := range searchPaths {
			candidates := []string{
				filepath.Join(searchPath, relativePath+".gecko"),
				filepath.Join(searchPath, relativePath, "mod.gecko"),
			}

			for _, candidate := range candidates {
				if moduleContent, err := os.ReadFile(candidate); err == nil {
					moduleFile, parseErr := parser.Parser.ParseString(candidate, string(moduleContent))
					if parseErr == nil {
						moduleFile.ComputeRanges()
						moduleFile.Path = candidate
						moduleFile.Content = string(moduleContent)
						moduleFile.Name = moduleName
						ctx.ImportedFiles[moduleName] = moduleFile
						ctx.MainFile.Imports = append(ctx.MainFile.Imports, moduleFile)
					}
					break
				}
			}
		}
	}
}

// registerSymbols adds all symbols to the AST scope
func (ctx *AnalysisContext) registerSymbols() {
	ctx.registerFileSymbols(ctx.MainFile, ctx.RootScope)

	for moduleName, moduleFile := range ctx.ImportedFiles {
		moduleScope := &ast.Ast{
			Scope:      moduleName,
			Parent:     ctx.RootScope,
			SourceFile: moduleFile.Path,
		}
		moduleScope.Init(ctx.RootScope.ErrorScope)
		ctx.RootScope.Children[moduleName] = moduleScope
		ctx.registerFileSymbols(moduleFile, moduleScope)
	}
}

// registerFileSymbols registers symbols from a file into a scope
func (ctx *AnalysisContext) registerFileSymbols(file *tokens.File, scope *ast.Ast) {
	for _, entry := range file.Entries {
		if entry.Class != nil {
			classScope := &ast.Ast{
				Scope:        entry.Class.Name,
				Parent:       scope,
				Visibility:   entry.Class.Visibility,
				OriginModule: file.PackageName,
				SourceFile:   file.Path,
			}
			classScope.Init(scope.ErrorScope)

			// Register class fields
			for _, field := range entry.Class.Fields {
				if field.Field != nil {
					classScope.Variables[field.Field.Name] = ast.Variable{
						Name:      field.Field.Name,
						IsPointer: field.Field.Type != nil && field.Field.Type.Pointer,
						IsConst:   field.Field.Mutability == "const" || (field.Field.Type != nil && field.Field.Type.Const && !field.Field.Type.Pointer),
						Parent:    classScope,
					}
				}
				if field.Method != nil {
					classScope.Methods[field.Method.Name] = &ast.Method{
						Name:       field.Method.Name,
						Visibility: field.Method.Visibility,
						Parent:     classScope,
						Type:       getReturnType(field.Method.Type),
						Unsafe:     tokens.HasAttribute(field.Method.Attributes, "unsafe"),
					}
				}
			}

			scope.Classes[entry.Class.Name] = classScope
		}

		if entry.Trait != nil {
			methods := []*ast.Method{}
			for _, field := range entry.Trait.Fields {
				methods = append(methods, &ast.Method{
					Name:       field.Name,
					Visibility: "public",
					Type:       getReturnType(field.Type),
				})
			}
			scope.Traits[entry.Trait.Name] = &methods
		}

		if entry.Method != nil {
			scope.Methods[entry.Method.Name] = &ast.Method{
				Name:       entry.Method.Name,
				Visibility: entry.Method.Visibility,
				Parent:     scope,
				Type:       getReturnType(entry.Method.Type),
				Unsafe:     tokens.HasAttribute(entry.Method.Attributes, "unsafe"),
			}
		}

		if entry.Field != nil {
			scope.Variables[entry.Field.Name] = ast.Variable{
				Name:      entry.Field.Name,
				IsPointer: entry.Field.Type != nil && entry.Field.Type.Pointer,
				IsConst:   entry.Field.Mutability == "const",
				Parent:    scope,
			}
		}

		// Register implementation methods
		if entry.Implementation != nil && entry.Implementation.GetFor() != "" {
			if classScope, ok := scope.Classes[entry.Implementation.GetFor()]; ok {
				traitName := entry.Implementation.GetName()
				methods := []*ast.Method{}
				for _, field := range entry.Implementation.GetFields() {
					method := &ast.Method{
						Name:       field.Name,
						Visibility: "public",
						Parent:     classScope,
						Type:       getReturnType(field.Type),
					}
					methods = append(methods, method)
				}
				classScope.Traits[traitName] = &methods
			}
		}
	}
}

func getReturnType(t *tokens.TypeRef) string {
	if t == nil {
		return "void"
	}
	return FormatTypeRef(t)
}

// getGeckoHome returns the Gecko home directory (same logic as compiler)
func getGeckoHome() string {
	hasStdlib := func(path string) bool {
		_, err := os.Stat(filepath.Join(path, "stdlib"))
		return err == nil
	}

	if home := os.Getenv("GECKO_HOME"); home != "" && hasStdlib(home) {
		return home
	}

	switch runtime.GOOS {
	case "darwin", "linux":
		if hasStdlib("/usr/local/lib/gecko") {
			return "/usr/local/lib/gecko"
		}
		if home := os.Getenv("HOME"); home != "" {
			userPath := filepath.Join(home, ".gecko")
			if hasStdlib(userPath) {
				return userPath
			}
		}
	case "windows":
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			winPath := filepath.Join(appData, "gecko")
			if hasStdlib(winPath) {
				return winPath
			}
		}
	}

	if wd, err := os.Getwd(); err == nil && hasStdlib(wd) {
		return wd
	}
	return "."
}
