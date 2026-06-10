// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

// SharedCompilePipelineOptions configures shared, backend-agnostic lowering setup.
type SharedCompilePipelineOptions struct {
	ProcessImports            bool
	MarkImportedModules       bool
	TrackLazyResolvedAsImport bool
}

// SharedCompilePipelineResult contains shared lowering artifacts.
type SharedCompilePipelineResult struct {
	RootScope    *ast.Ast
	ImportScopes []*ast.Ast
	SemanticInfo *semantic.Program
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func copySelectedSymbol(dst *ast.Ast, src *ast.Ast, name string) {
	if dst == nil || src == nil || name == "" {
		return
	}

	if variable, found := src.Variables[name]; found {
		dst.Variables[name] = variable
	}
	if cls, found := src.Classes[name]; found {
		dst.Classes[name] = cls
	}
	if trait, found := src.Traits[name]; found {
		dst.Traits[name] = trait
	}
	if method, found := src.Methods[name]; found {
		dst.Methods[name] = method
	}
}

func buildUseObjectMap(entries []*tokens.Entry) map[string][]string {
	useObjects := make(map[string][]string)
	for _, entry := range entries {
		if entry != nil && entry.Import != nil && len(entry.Import.Objects) > 0 {
			useObjects[entry.Import.ModuleName()] = entry.Import.Objects
		}
	}
	return useObjects
}

func newImportedScope(importedFile *tokens.File, scopeName string, markImported bool) *ast.Ast {
	scope := &ast.Ast{
		Scope:            scopeName,
		Parent:           nil, // Keep imported scope names stable: module__symbol.
		IsImportedModule: markImported,
		SourceFile:       importedFile.Path,
	}
	scope.Init(errors.NewErrorScope(importedFile.Name, importedFile.Path, importedFile.Content))
	scope.Config = importedFile.Config
	return scope
}

// PrepareSharedCompilePipeline performs backend-agnostic lowering preparation:
// root scope creation, lazy resolver wiring, import graph processing, and entry lowering.
func PrepareSharedCompilePipeline(b interfaces.BackendInterface, c *interfaces.BackendConfig, options SharedCompilePipelineOptions) *SharedCompilePipelineResult {
	result := &SharedCompilePipelineResult{
		ImportScopes: make([]*ast.Ast, 0),
		SemanticInfo: c.SemanticInfo,
	}

	rootScope := &ast.Ast{
		Scope:      c.SourceFile.PackageName,
		SourceFile: c.SourceFile.Path,
	}
	rootScope.Init(errors.NewErrorScope(c.SourceFile.Name, c.SourceFile.Path, c.SourceFile.Content))
	rootScope.Config = c.SourceFile.Config

	if c.SuggestionProvider != nil {
		rootScope.SuggestionProvider = func(typeName string) string {
			return c.SuggestionProvider(typeName)
		}
	}

	// Lazy-resolved module scopes are intentionally tracked separately from
	// eagerly processed imports to preserve current behavior.
	lazyResolvedFiles := make(map[string]*ast.Ast)
	getOrCreateLazyScope := func(resolvedFile *tokens.File) *ast.Ast {
		cacheKey := firstNonEmpty(resolvedFile.Path, resolvedFile.PackageName, resolvedFile.Name)
		if existingScope, ok := lazyResolvedFiles[cacheKey]; ok {
			return existingScope
		}

		scopeName := firstNonEmpty(resolvedFile.PackageName, resolvedFile.Name, resolvedFile.Path)
		resolvedScope := newImportedScope(resolvedFile, scopeName, options.MarkImportedModules)
		b.ProcessEntries(resolvedFile.Entries, resolvedScope)

		if scopeName != "" {
			rootScope.Children[scopeName] = resolvedScope
		}

		lazyResolvedFiles[cacheKey] = resolvedScope

		if options.TrackLazyResolvedAsImport {
			result.ImportScopes = append(result.ImportScopes, resolvedScope)
		}

		return resolvedScope
	}

	if c.LazyTypeResolver != nil {
		rootScope.LazyResolver = func(typeName string) (*ast.Ast, bool) {
			resolvedFile, found := c.LazyTypeResolver(typeName)
			if !found {
				return nil, false
			}
			resolvedScope := getOrCreateLazyScope(resolvedFile)
			if cls, ok := resolvedScope.Classes[typeName]; ok {
				return cls, true
			}
			return nil, false
		}
	}

	if c.LazyMethodResolver != nil {
		rootScope.LazyMethodResolver = func(methodName string) (*ast.Method, bool) {
			resolvedFile, found := c.LazyMethodResolver(methodName)
			if !found {
				return nil, false
			}
			resolvedScope := getOrCreateLazyScope(resolvedFile)
			if mth, ok := resolvedScope.Methods[methodName]; ok {
				return mth, true
			}
			return nil, false
		}
	}

	if c.LazyModuleTypeResolver != nil {
		rootScope.LazyModuleTypeResolver = func(moduleName string, typeName string) (*ast.Ast, bool) {
			resolvedFile, found := c.LazyModuleTypeResolver(moduleName, typeName)
			if !found {
				return nil, false
			}
			resolvedScope := getOrCreateLazyScope(resolvedFile)
			rootScope.Children[moduleName] = resolvedScope
			if cls, ok := resolvedScope.Classes[typeName]; ok {
				return cls, true
			}
			return nil, false
		}
	}

	if options.ProcessImports {
		useObjects := buildUseObjectMap(c.SourceFile.Entries)
		processedModules := make(map[string]*ast.Ast)

		var processImport func(importedFile *tokens.File, parentScope *ast.Ast) (*ast.Ast, bool)
		processImport = func(importedFile *tokens.File, parentScope *ast.Ast) (*ast.Ast, bool) {
			moduleKey := firstNonEmpty(importedFile.Name, importedFile.PackageName, importedFile.Path)
			processKey := firstNonEmpty(importedFile.Path, moduleKey)

			if existingScope, ok := processedModules[processKey]; ok {
				if parentScope != nil {
					parentScope.Children[moduleKey] = existingScope
				}
				return existingScope, true
			}

			importScope := newImportedScope(importedFile, moduleKey, options.MarkImportedModules)
			processedModules[processKey] = importScope
			rootScope.Children[moduleKey] = importScope

			if parentScope != nil {
				parentScope.Children[moduleKey] = importScope
			}

			localUseObjects := buildUseObjectMap(importedFile.Entries)
			for _, nestedImport := range importedFile.Imports {
				nestedScope, alreadyProcessed := processImport(nestedImport, importScope)
				if !alreadyProcessed {
					result.ImportScopes = append(result.ImportScopes, nestedScope)
				}

				nestedModuleKey := firstNonEmpty(nestedImport.Name, nestedImport.PackageName, nestedImport.Path)
				if objects, ok := localUseObjects[nestedModuleKey]; ok {
					for _, objName := range objects {
						copySelectedSymbol(importScope, nestedScope, objName)
					}
				}
			}

			b.ProcessEntries(importedFile.Entries, importScope)
			return importScope, false
		}

		for _, importedFile := range c.SourceFile.Imports {
			importScope, alreadyProcessed := processImport(importedFile, nil)
			if !alreadyProcessed {
				result.ImportScopes = append(result.ImportScopes, importScope)
			}

			moduleKey := firstNonEmpty(importedFile.Name, importedFile.PackageName, importedFile.Path)
			if objects, ok := useObjects[moduleKey]; ok {
				for _, objName := range objects {
					copySelectedSymbol(rootScope, importScope, objName)
				}
			}
		}
	}

	b.ProcessEntries(c.SourceFile.Entries, rootScope)
	result.RootScope = rootScope
	return result
}
