// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
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
	cbackend.ResetTreeshakeAnalysis()
	sharedResult := PrepareSharedCompilePipeline(b, c, SharedCompilePipelineOptions{
		ProcessImports:            true,
		MarkImportedModules:       true,
		TrackLazyResolvedAsImport: true,
	})
	file := sharedResult.RootScope
	importScopes := sharedResult.ImportScopes

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
	var allImportTypeDefs, allImportTypes, allImportDecls, allImportGlobals, allImportFuncs, allImportIncludes, allImportObjects, allImportExternalRoots []string
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
		allImportExternalRoots = append(allImportExternalRoots, importInfo.ExternalRootSymbols...)
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
	info.ExternalRootSymbols = append(allImportExternalRoots, info.ExternalRootSymbols...)

	// Append project-level native headers from gecko.toml.
	if file.Config != nil && file.Config.Project != nil {
		info.Includes = append(info.Includes, file.Config.Project.GetNativeHeadersForTarget(file.Config.TargetKey)...)
	}

	// Normalize include/library/object lists while preserving declaration order.
	info.Includes = dedupeStrings(info.Includes)
	info.CImportLibraries = dedupeStrings(info.CImportLibraries)
	info.CImportObjects = dedupeStrings(info.CImportObjects)
	info.ExternalRootSymbols = dedupeStrings(info.ExternalRootSymbols)

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

	// Treeshake safety gate: disable for this invocation when dynamic calls
	// make static reachability unsafe in v1.
	if file.Config != nil && file.Config.Treeshake {
		warnings := cbackend.GetTreeshakeDynamicCallWarnings()
		if len(warnings) > 0 {
			file.Config.Treeshake = false
			cbackend.LastTreeshakeAutoDisabled = true
			cbackend.LastTreeshakeDisableWarnings = warnings

			fmt.Fprintln(os.Stderr, "warning: treeshake disabled for this build due to dynamic-call patterns:")
			for _, w := range warnings {
				location := fmt.Sprintf("%s:%d:%d", w.File, w.Line, w.Column)
				if strings.TrimSpace(w.File) == "" {
					location = fmt.Sprintf("<unknown>:%d:%d", w.Line, w.Column)
				}
				fmt.Fprintf(os.Stderr, "  %s: %s\n", location, w.Reason)
			}
		}
	}

	// Generate C code
	treeshakeEnabled := file.Config != nil && file.Config.Treeshake
	cCode := generateCCode(info, treeshakeEnabled)

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
	if file.Config != nil && file.Config.Treeshake {
		gccArgs = append(gccArgs, "-ffunction-sections", "-fdata-sections")
	}

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

// generateCCode creates the final C source code from scope information.
func generateCCode(info *cbackend.CScopeInformation, treeshakeEnabled bool) string {
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

	// Keep Gecko `external func` definitions alive under section GC.
	if treeshakeEnabled && len(info.ExternalRootSymbols) > 0 {
		sb.WriteString("/* Treeshake roots for Gecko external functions */\n")
		sb.WriteString("static void (* const __gecko_external_roots[])(void) __attribute__((used)) = {\n")
		for _, symbol := range info.ExternalRootSymbols {
			sb.WriteString("    (void (*)(void))" + symbol + ",\n")
		}
		sb.WriteString("};\n\n")
		sb.WriteString("/* Ensure external roots remain reachable when linker GC is enabled. */\n")
		sb.WriteString("static void __attribute__((constructor)) __gecko_keep_external_roots(void) {\n")
		sb.WriteString("    volatile const void* root = (const void*)__gecko_external_roots;\n")
		sb.WriteString("    (void)root;\n")
		sb.WriteString("}\n\n")
	}

	return sb.String()
}
