// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"os"
	"os/exec"
	"path/filepath"
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

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
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

	// Generate class instantiations (struct + methods).
	// Use an index-based queue so nested instantiations requested during codegen
	// (e.g., Option<Vec<int32>> discovered while generating Vec<int32>) are
	// processed in the same compilation pass.
	for i := 0; i < len(cbackend.Generics.ClassInstantiations); i++ {
		inst := cbackend.Generics.ClassInstantiations[i]
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

	// Generate method instantiations. Use queue semantics for the same reason as
	// class instantiations: generation may request additional method monomorphs.
	for i := 0; i < len(cbackend.Generics.MethodInstantiations); i++ {
		inst := cbackend.Generics.MethodInstantiations[i]
		methodToken, ok := cbackend.Generics.GenericMethods[inst.Name]
		if !ok {
			continue
		}

		// Validate trait constraints
		for i, param := range methodToken.TypeParams {
			if len(param.AllTraits()) > 0 && i < len(inst.TypeArgs) {
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
					for _, requiredTrait := range param.AllTraits() {
						hasTrait := false
						for implementedTrait := range class.Traits {
							if cbackend.TraitMatchesOrExtends(implementedTrait, requiredTrait) {
								hasTrait = true
								break
							}
						}
						if !hasTrait {
							scope.ErrorScope.NewCompileTimeError(
								"Trait Constraint Error",
								"Type '"+baseType+"' does not implement trait '"+requiredTrait+"' required by type parameter '"+param.Name+"'",
								lexer.Position{},
							)
						}
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
		Scope:      c.SourceFile.PackageName,
		SourceFile: c.SourceFile.Path,
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
			SourceFile:       resolvedFile.Path,
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
		moduleKey := importedFile.Name
		if moduleKey == "" {
			moduleKey = importedFile.PackageName
		}
		if moduleKey == "" {
			moduleKey = importedFile.Path
		}

		processKey := importedFile.Path
		if processKey == "" {
			processKey = moduleKey
		}

		if existingScope, ok := processedModules[processKey]; ok {
			// Already processed - just link to parent
			if parentScope != nil {
				parentScope.Children[moduleKey] = existingScope
			}
			return existingScope, true
		}

		importScope := &ast.Ast{
			Scope:            moduleKey,
			Parent:           nil,  // No parent so names are module__symbol, not main__module__symbol
			IsImportedModule: true, // Mark as imported module for scoped typedef names
			SourceFile:       importedFile.Path,
		}
		importScope.Init(errors.NewErrorScope(importedFile.Name, importedFile.Path, importedFile.Content))
		importScope.Config = importedFile.Config

		processedModules[processKey] = importScope
		file.Children[moduleKey] = importScope

		if parentScope != nil {
			parentScope.Children[moduleKey] = importScope
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
			nestedModuleKey := nestedImport.Name
			if nestedModuleKey == "" {
				nestedModuleKey = nestedImport.PackageName
			}
			if objects, ok := localUseObjects[nestedModuleKey]; ok {
				for _, objName := range objects {
					if variable, found := nestedScope.Variables[objName]; found {
						importScope.Variables[objName] = variable
					}
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
		moduleKey := importedFile.Name
		if moduleKey == "" {
			moduleKey = importedFile.PackageName
		}
		if objects, ok := useObjects[moduleKey]; ok {
			for _, objName := range objects {
				// Copy variables/constants.
				if variable, found := importScope.Variables[objName]; found {
					file.Variables[objName] = variable
				}
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
	var allImportTypeDefs, allImportTypes, allImportDecls, allImportGlobals, allImportFuncs, allImportIncludes, allImportObjects []string
	var allImportStructDefs []*cbackend.StructDefinition
	var allCImportLibraries []string

	for _, importScope := range importScopes {
		importInfo := cbackend.CGetScopeInformation(importScope)
		allImportTypeDefs = append(allImportTypeDefs, importInfo.TypeDefs...)
		allImportTypes = append(allImportTypes, importInfo.Types...)
		allImportStructDefs = append(allImportStructDefs, importInfo.StructDefs...)
		allImportDecls = append(allImportDecls, importInfo.Declarations...)
		allImportGlobals = append(allImportGlobals, importInfo.Globals...)
		allImportFuncs = append(allImportFuncs, importInfo.Functions...)
		allImportIncludes = append(allImportIncludes, importInfo.Includes...)
		allCImportLibraries = append(allCImportLibraries, importInfo.CImportLibraries...)
		allImportObjects = append(allImportObjects, importInfo.CImportObjects...)
	}

	// Prepend combined import info so imports come before main
	info.TypeDefs = append(allImportTypeDefs, info.TypeDefs...)
	info.Types = append(allImportTypes, info.Types...)
	info.StructDefs = append(allImportStructDefs, info.StructDefs...)
	info.Declarations = append(allImportDecls, info.Declarations...)
	info.Globals = append(allImportGlobals, info.Globals...)
	info.Functions = append(allImportFuncs, info.Functions...)
	info.Includes = append(allImportIncludes, info.Includes...)
	info.CImportLibraries = append(allCImportLibraries, info.CImportLibraries...)
	info.CImportObjects = append(allImportObjects, info.CImportObjects...)

	// Append project-level native headers from gecko.toml.
	if file.Config != nil && file.Config.Project != nil {
		info.Includes = append(info.Includes, file.Config.Project.GetNativeHeadersForTarget(file.Config.TargetKey)...)
	}

	// Normalize include/library/object lists while preserving declaration order.
	info.Includes = dedupeStrings(info.Includes)
	info.CImportLibraries = dedupeStrings(info.CImportLibraries)
	info.CImportObjects = dedupeStrings(info.CImportObjects)

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

	// Store CImportLibraries for access by build command
	cbackend.LastCImportLibraries = info.CImportLibraries
	cbackend.LastCImportObjects = info.CImportObjects

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
		// Copy generated C into project-local artifact directory when available.
		destFile := c.File + ".c"
		if c.SourceFile.Config != nil && c.SourceFile.Config.Project != nil {
			destFile = c.SourceFile.Config.Project.GetArtifactPath(c.File, ".c")
		}
		_ = os.MkdirAll(filepath.Dir(destFile), 0o755)

		cmd := exec.Command("cp", outFile, destFile)
		streamErr := streamCommand(cmd)

		if streamErr != nil {
			file.ErrorScope.NewCompileTimeError("C copy", "Error copying C file "+streamErr.Error(), lexer.Position{})
		}

		return nil
	}

	// Compile with gcc
	gccArgs := []string{"-c"}

	// Only add freestanding flags if no project config or explicit target
	if file.Config.Project == nil || (file.Config.Vendor != "" || file.Config.Arch != "") {
		gccArgs = append(gccArgs, "-ffreestanding", "-nostdlib")
	}

	// Add architecture-specific flags
	if file.Config.Arch == "arm64" && file.Config.Platform == "darwin" {
		gccArgs = append(gccArgs, "-target", "arm64-apple-darwin")
	} else if file.Config.Vendor != "" {
		gccArgs = append(gccArgs, "-target", file.Config.Arch+"-"+file.Config.Vendor+"-"+file.Config.Platform)
	}

	// Add user-specified CFlags from config
	if len(file.Config.CFlags) > 0 {
		gccArgs = append(gccArgs, file.Config.CFlags...)
	}

	// Add CFlags from project config (includes pkg-config)
	if file.Config.Project != nil {
		if cflags, err := file.Config.Project.GetCFlagsForTarget(file.Config.TargetKey); err == nil {
			gccArgs = append(gccArgs, cflags...)
		}
	}

	// Add pkg-config --cflags for cimport libraries (for include paths)
	if len(info.CImportLibraries) > 0 {
		// Deduplicate libraries
		libSet := make(map[string]bool)
		for _, lib := range info.CImportLibraries {
			libSet[lib] = true
		}
		for lib := range libSet {
			pkgCmd := exec.Command("pkg-config", "--cflags", lib)
			if output, err := pkgCmd.Output(); err == nil {
				flags := strings.Fields(strings.TrimSpace(string(output)))
				gccArgs = append(gccArgs, flags...)
			}
		}
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

	// Gecko runtime hooks used by lowered try diagnostics.
	sb.WriteString("/* Gecko runtime hooks */\n")
	sb.WriteString("typedef void (*__gecko_try_error_handler_t)(const char* file, int32_t line, int32_t column, const char* function_name, const char* expr);\n")
	sb.WriteString("static __gecko_try_error_handler_t __gecko_try_error_handler = 0;\n")
	sb.WriteString("void __gecko_set_try_error_handler(__gecko_try_error_handler_t handler) {\n")
	sb.WriteString("    __gecko_try_error_handler = handler;\n")
	sb.WriteString("}\n")
	sb.WriteString("static void __gecko_try_fail(const char* file, int32_t line, int32_t column, const char* function_name, const char* expr) {\n")
	sb.WriteString("    if (__gecko_try_error_handler) {\n")
	sb.WriteString("        __gecko_try_error_handler(file, line, column, function_name, expr);\n")
	sb.WriteString("    }\n")
	sb.WriteString("    __builtin_trap();\n")
	sb.WriteString("}\n\n")

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
