package backends

import (
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type CBackend struct {
	impls    interfaces.BackendCodegenImplementations
	features *FeatureSet
}

func (b *CBackend) Init() {
	b.features = NewCFeatureSet()
	b.impls = &cbackend.CBackendImplementation{Backend: b}
	cbackend.CurrentBackend = b
	cbackend.Methods = make(map[string]*ast.Method)
}

func (b *CBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	BackendProcessEntries(b, scope, entries)
}

// generateGenericInstantiations generates C code for all pending generic instantiations
func (b *CBackend) generateGenericInstantiations(scope *ast.Ast) {
	impl := b.impls.(*cbackend.CBackendImplementation)

	// Generate class instantiations (struct + methods)
	for _, inst := range cbackend.Generics.ClassInstantiations {
		classToken, ok := cbackend.Generics.GenericClasses[inst.Name]
		if !ok {
			continue
		}
		impl.GenerateClassDef(scope, classToken, inst.FullName, inst.TypeArgs)

		// Build method name prefix with origin module if present
		methodPrefix := inst.FullName
		if inst.OriginModule != "" {
			methodPrefix = inst.OriginModule + "__" + inst.FullName
		}

		// Also generate methods for this class instantiation
		for _, f := range classToken.Fields {
			if f.Method != nil {
				methodName := methodPrefix + "__" + f.Method.Name
				impl.GenerateClassMethodDef(scope, classToken, f.Method, methodName, inst.FullName, inst.TypeArgs)
			}
		}

		// Generate methods from attached impl blocks
		for _, implBlock := range classToken.Implementations {
			if implBlock.GetFor() != "" {
				// Trait impl (impl<T> Trait for Class<T>)
				// Generate trait methods with proper naming
				impl.GenerateGenericTraitImpl(scope, classToken, implBlock, inst)
			} else {
				// Inherent impl (impl<T> Class<T> { ... })
				for _, f := range implBlock.GetFields() {
					m := f.ToMethodToken()
					methodName := methodPrefix + "__" + m.Name
					impl.GenerateClassMethodDef(scope, classToken, m, methodName, inst.FullName, inst.TypeArgs)
				}
			}
		}
	}

	// Generate method instantiations
	for _, inst := range cbackend.Generics.MethodInstantiations {
		methodToken, ok := cbackend.Generics.GenericMethods[inst.Name]
		if !ok {
			continue
		}

		// Validate trait constraints
		for i, param := range methodToken.TypeParams {
			if param.Trait != "" && i < len(inst.TypeArgs) {
				concreteType := inst.TypeArgs[i]
				// Remove pointer suffix for class lookup
				baseType := concreteType
				if len(baseType) > 0 && baseType[len(baseType)-1] == '*' {
					baseType = baseType[:len(baseType)-1]
				}
				// Also remove "struct " prefix if present
				if strings.HasPrefix(baseType, "struct ") {
					baseType = baseType[7:]
				}

				// Look up the class and check if it implements the trait
				classOpt := scope.ResolveClass(baseType)
				if !classOpt.IsNil() {
					class := classOpt.Unwrap()
					_, hasTrait := class.Traits[param.Trait]
					if !hasTrait {
						scope.ErrorScope.NewCompileTimeError(
							"Trait Constraint Error",
							"Type '"+baseType+"' does not implement trait '"+param.Trait+"' required by type parameter '"+param.Name+"'",
							lexer.Position{},
						)
					}
				}
			}
		}

		impl.GenerateMethodDef(scope, methodToken, inst.FullName, inst.TypeArgs)
	}

	// Clear for next compilation
	cbackend.ResetGenerics()
}

func (b *CBackend) GetImpls() interfaces.BackendCodegenImplementations {
	return b.impls
}

func (b *CBackend) Features() interfaces.FeatureChecker {
	return b.features
}

func (b *CBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	file := &ast.Ast{
		Scope: c.SourceFile.PackageName,
	}

	file.Init(errors.NewErrorScope(c.SourceFile.Name, c.SourceFile.Path, c.SourceFile.Content))
	file.Config = c.SourceFile.Config

	// Set up type suggestion provider for helpful error messages
	if c.SuggestionProvider != nil {
		file.SuggestionProvider = func(typeName string) string {
			return c.SuggestionProvider(typeName)
		}
	}

	// Track all import scopes for merging (including lazy-resolved ones)
	importScopes := make([]*ast.Ast, 0)

	// Track files already processed via lazy resolution to avoid duplication
	lazyResolvedFiles := make(map[string]*ast.Ast)

	// Helper to get or create a scope from a lazy-resolved file
	getOrCreateLazyScope := func(resolvedFile *tokens.File) *ast.Ast {
		if existingScope, ok := lazyResolvedFiles[resolvedFile.Path]; ok {
			return existingScope
		}

		// Create a scope using the file's package name
		resolvedScope := &ast.Ast{
			Scope:            resolvedFile.PackageName,
			Parent:           nil,
			IsImportedModule: true, // Mark as imported module for scoped typedef names
		}
		resolvedScope.Init(errors.NewErrorScope(resolvedFile.Name, resolvedFile.Path, resolvedFile.Content))
		resolvedScope.Config = resolvedFile.Config

		// Process the resolved file's entries
		b.ProcessEntries(resolvedFile.Entries, resolvedScope)

		// Add as child of main file for symbol resolution
		file.Children[resolvedFile.PackageName] = resolvedScope

		// Track for C code merging
		importScopes = append(importScopes, resolvedScope)

		// Cache for future lazy resolutions from the same file
		lazyResolvedFiles[resolvedFile.Path] = resolvedScope

		return resolvedScope
	}

	// Set up lazy type resolver for directory imports
	if c.LazyTypeResolver != nil {
		file.LazyResolver = func(typeName string) (*ast.Ast, bool) {
			resolvedFile, found := c.LazyTypeResolver(typeName)
			if !found {
				return nil, false
			}

			resolvedScope := getOrCreateLazyScope(resolvedFile)

			// Find and return the requested class
			if cls, ok := resolvedScope.Classes[typeName]; ok {
				return cls, true
			}

			return nil, false
		}
	}

	// Set up lazy method resolver for directory imports
	if c.LazyMethodResolver != nil {
		file.LazyMethodResolver = func(methodName string) (*ast.Method, bool) {
			resolvedFile, found := c.LazyMethodResolver(methodName)
			if !found {
				return nil, false
			}

			resolvedScope := getOrCreateLazyScope(resolvedFile)

			// Find and return the requested method
			if mth, ok := resolvedScope.Methods[methodName]; ok {
				return mth, true
			}

			return nil, false
		}
	}

	// Set up lazy module type resolver for qualified types (e.g., shapes.Circle)
	if c.LazyModuleTypeResolver != nil {
		file.LazyModuleTypeResolver = func(moduleName string, typeName string) (*ast.Ast, bool) {
			resolvedFile, found := c.LazyModuleTypeResolver(moduleName, typeName)
			if !found {
				return nil, false
			}

			resolvedScope := getOrCreateLazyScope(resolvedFile)

			// Register the scope as a child module for future lookups
			file.Children[moduleName] = resolvedScope

			// Find and return the requested class
			if cls, ok := resolvedScope.Classes[typeName]; ok {
				return cls, true
			}

			return nil, false
		}
	}

	// Build a map of module name -> use objects for quick lookup
	useObjects := make(map[string][]string)
	for _, entry := range c.SourceFile.Entries {
		if entry.Import != nil && len(entry.Import.Objects) > 0 {
			useObjects[entry.Import.ModuleName()] = entry.Import.Objects
		}
	}

	// Track processed modules to avoid duplicates
	processedModules := make(map[string]*ast.Ast)

	// Helper to recursively process imports
	// Returns (scope, alreadyProcessed)
	var processImport func(importedFile *tokens.File, parentScope *ast.Ast) (*ast.Ast, bool)
	processImport = func(importedFile *tokens.File, parentScope *ast.Ast) (*ast.Ast, bool) {
		if existingScope, ok := processedModules[importedFile.PackageName]; ok {
			// Already processed - just link to parent
			if parentScope != nil {
				parentScope.Children[importedFile.PackageName] = existingScope
			}
			return existingScope, true
		}

		importScope := &ast.Ast{
			Scope:            importedFile.PackageName,
			Parent:           nil, // No parent so names are module__symbol, not main__module__symbol
			IsImportedModule: true, // Mark as imported module for scoped typedef names
		}
		importScope.Init(errors.NewErrorScope(importedFile.Name, importedFile.Path, importedFile.Content))
		importScope.Config = importedFile.Config

		processedModules[importedFile.PackageName] = importScope
		file.Children[importedFile.PackageName] = importScope

		if parentScope != nil {
			parentScope.Children[importedFile.PackageName] = importScope
		}

		// Build use objects map for this imported file
		localUseObjects := make(map[string][]string)
		for _, entry := range importedFile.Entries {
			if entry.Import != nil && len(entry.Import.Objects) > 0 {
				localUseObjects[entry.Import.ModuleName()] = entry.Import.Objects
			}
		}

		// Recursively process nested imports BEFORE processing entries
		for _, nestedImport := range importedFile.Imports {
			nestedScope, alreadyProcessed := processImport(nestedImport, importScope)
			if !alreadyProcessed {
				importScopes = append(importScopes, nestedScope)
			}

			// Copy symbols from nested import's 'use { ... }' into this file's scope
			if objects, ok := localUseObjects[nestedImport.PackageName]; ok {
				for _, objName := range objects {
					if cls, found := nestedScope.Classes[objName]; found {
						importScope.Classes[objName] = cls
					}
					if trait, found := nestedScope.Traits[objName]; found {
						importScope.Traits[objName] = trait
					}
					if method, found := nestedScope.Methods[objName]; found {
						importScope.Methods[objName] = method
					}
				}
			}
		}

		b.ProcessEntries(importedFile.Entries, importScope)
		return importScope, false
	}

	// Process imported modules first (as root-level scopes for proper naming)
	for _, importedFile := range c.SourceFile.Imports {
		importScope, alreadyProcessed := processImport(importedFile, nil)
		if !alreadyProcessed {
			importScopes = append(importScopes, importScope)
		}

		// Copy symbols specified in 'use { ... }' into the main file's scope
		if objects, ok := useObjects[importedFile.PackageName]; ok {
			for _, objName := range objects {
				// Copy classes
				if cls, found := importScope.Classes[objName]; found {
					file.Classes[objName] = cls
				}
				// Copy traits
				if trait, found := importScope.Traits[objName]; found {
					file.Traits[objName] = trait
				}
				// Copy methods (free functions)
				if method, found := importScope.Methods[objName]; found {
					file.Methods[objName] = method
				}
			}
		}
	}

	b.ProcessEntries(c.SourceFile.Entries, file)

	// Generate all pending generic instantiations
	b.generateGenericInstantiations(file)

	// Check for errors from imported modules
	for _, importScope := range importScopes {
		if importScope.ErrorScope.HasErrors() {
			for _, err := range importScope.ErrorScope.CompileTimeErrors {
				println(err.GetError())
			}
			return nil
		}
	}

	// Check for errors from the main file - bail early if any
	if file.ErrorScope.HasErrors() {
		for _, err := range file.ErrorScope.CompileTimeErrors {
			println(err.GetError())
		}
		return nil
	}

	info := cbackend.CGetScopeInformation(file)

	// Collect all import info in dependency order (importScopes is already ordered correctly)
	// Then prepend the combined import info to main's info
	var allImportTypeDefs, allImportTypes, allImportDecls, allImportGlobals, allImportFuncs, allImportIncludes []string
	var allImportStructDefs []*cbackend.StructDefinition

	for _, importScope := range importScopes {
		importInfo := cbackend.CGetScopeInformation(importScope)
		allImportTypeDefs = append(allImportTypeDefs, importInfo.TypeDefs...)
		allImportTypes = append(allImportTypes, importInfo.Types...)
		allImportStructDefs = append(allImportStructDefs, importInfo.StructDefs...)
		allImportDecls = append(allImportDecls, importInfo.Declarations...)
		allImportGlobals = append(allImportGlobals, importInfo.Globals...)
		allImportFuncs = append(allImportFuncs, importInfo.Functions...)
		allImportIncludes = append(allImportIncludes, importInfo.Includes...)
	}

	// Prepend combined import info so imports come before main
	info.TypeDefs = append(allImportTypeDefs, info.TypeDefs...)
	info.Types = append(allImportTypes, info.Types...)
	info.StructDefs = append(allImportStructDefs, info.StructDefs...)
	info.Declarations = append(allImportDecls, info.Declarations...)
	info.Globals = append(allImportGlobals, info.Globals...)
	info.Functions = append(allImportFuncs, info.Functions...)
	info.Includes = append(allImportIncludes, info.Includes...)

	// Check for circular value dependencies (infinite size cycles)
	cycles := cbackend.DetectCircularValueDependencies(info.StructDefs)
	for _, cycle := range cycles {
		if len(cycle.Types) > 0 {
			// Build list of type names in the cycle
			typeNames := make([]string, len(cycle.Types))
			for i, t := range cycle.Types {
				typeNames[i] = t.Name
			}
			cycleDesc := strings.Join(typeNames, " -> ") + " -> " + typeNames[0]

			file.ErrorScope.NewCompileTimeError(
				"Circular Type Dependency",
				"Types have circular value dependencies causing infinite size: "+cycleDesc+
					". Use pointers to break the cycle.",
				cycle.Types[0].Pos,
			)
		}
	}

	// Topologically sort struct definitions so dependencies come first
	info.StructDefs = cbackend.TopologicalSortStructs(info.StructDefs)

	// Generate C code
	cCode := generateCCode(info)

	if c.Ctx.Bool("print-ir") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + cCode)
	}

	// Write output file with .c extension
	// c.OutName already contains the build directory path (e.g., /tmp/gecko/build/xxx/file.ll)
	// Replace .ll extension with .c
	outFile := c.OutName
	if strings.HasSuffix(outFile, ".ll") {
		outFile = outFile[:len(outFile)-3] + ".c"
	} else if idx := strings.LastIndex(outFile, "."); idx != -1 {
		outFile = outFile[:idx] + ".c"
	} else {
		outFile = outFile + ".c"
	}

	// Ensure the parent directory exists
	if idx := strings.LastIndex(outFile, "/"); idx != -1 {
		parentDir := outFile[:idx]
		os.MkdirAll(parentDir, 0o755)
	}

	err := os.WriteFile(outFile, []byte(cCode), 0o755)
	if err != nil {
		file.ErrorScope.NewCompileTimeError("File write error", "Error writing C file: "+err.Error(), lexer.Position{})
		return nil
	}

	if c.Ctx.Bool("ir-only") {
		// Copy to same directory as source file (not current dir, to avoid Go finding .c files)
		sourceDir := "."
		if idx := strings.LastIndex(c.File, "/"); idx != -1 {
			sourceDir = c.File[:idx]
		}
		destFile := sourceDir + "/" + c.File[strings.LastIndex(c.File, "/")+1:] + ".c"

		cmd := exec.Command("cp", outFile, destFile)
		streamErr := streamCommand(cmd)

		if streamErr != nil {
			file.ErrorScope.NewCompileTimeError("C copy", "Error copying C file "+streamErr.Error(), lexer.Position{})
		}

		return nil
	}

	// Compile with gcc
	gccArgs := []string{"-c", "-ffreestanding", "-nostdlib"}

	// Add architecture-specific flags
	if file.Config.Arch == "arm64" && file.Config.Platform == "darwin" {
		gccArgs = append(gccArgs, "-target", "arm64-apple-darwin")
	} else if file.Config.Vendor != "" {
		gccArgs = append(gccArgs, "-target", file.Config.Arch+"-"+file.Config.Vendor+"-"+file.Config.Platform)
	}

	// Output object file
	objFile := outFile[:len(outFile)-2] + ".o"
	gccArgs = append(gccArgs, "-o", objFile)
	gccArgs = append(gccArgs, outFile)

	cmd := exec.Command("gcc", gccArgs...)

	return cmd
}

// generateCCode creates the final C source code from scope information
func generateCCode(info *cbackend.CScopeInformation) string {
	var sb strings.Builder

	// Header comment
	sb.WriteString("/* Generated by Gecko C Backend */\n\n")

	// Include standard headers
	sb.WriteString("#include <stdint.h>\n")
	sb.WriteString("#include <stddef.h>\n")

	// Write user-specified includes from cimport statements
	if len(info.Includes) > 0 {
		for _, include := range info.Includes {
			sb.WriteString(include)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	// Write typedef declarations (external types)
	if len(info.TypeDefs) > 0 {
		sb.WriteString("/* External type declarations */\n")
		for _, typeDef := range info.TypeDefs {
			sb.WriteString(typeDef)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Write type definitions (structs) - use StructDefs if available (sorted), else Types
	if len(info.StructDefs) > 0 {
		sb.WriteString("/* Type definitions */\n")
		for _, structDef := range info.StructDefs {
			sb.WriteString(structDef.Code)
			sb.WriteString("\n")
		}
	} else if len(info.Types) > 0 {
		sb.WriteString("/* Type definitions */\n")
		for _, typeDef := range info.Types {
			sb.WriteString(typeDef)
			sb.WriteString("\n")
		}
	}

	// Write extern declarations
	if len(info.Declarations) > 0 {
		sb.WriteString("/* External declarations */\n")
		for _, decl := range info.Declarations {
			sb.WriteString(decl)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Write global variables
	if len(info.Globals) > 0 {
		sb.WriteString("/* Global variables */\n")
		for _, global := range info.Globals {
			sb.WriteString(global)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Write function definitions
	if len(info.Functions) > 0 {
		sb.WriteString("/* Function definitions */\n")
		for _, fn := range info.Functions {
			sb.WriteString(fn)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
