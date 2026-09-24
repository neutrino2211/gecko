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
	"github.com/neutrino2211/gecko/utils"
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
	cbackend.CScopeDataMap = &cbackend.CScopeData{}
	cbackend.CProgramValues = &cbackend.CValuesMap{}
	cbackend.MethodSignatures = make(map[string]*cbackend.MethodSignature)
	cbackend.TraitDefinitions = make(map[string]*tokens.Trait)
	cbackend.TraitDefinitionOrigins = make(map[string]string)
	cbackend.EnumToCType = make(map[string]string)
	cbackend.MethodReturnTypes = make(map[string]*tokens.TypeRef)
	cbackend.Generics = cbackend.NewGenericRegistry()
	cbackend.CurrentMonomorphContext = nil
	cbackend.CurrentTypeState = nil
	cbackend.SetSemanticProgram(nil)
	cbackend.InitTypeParameterChecker()

}

func (b *CBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	BackendProcessEntries(b, scope, entries)
}

func (b *CBackend) GetImpls() interfaces.BackendCodegenImplementations {
	return b.impls
}

func (b *CBackend) Features() interfaces.FeatureChecker {
	return b.features
}

func (b *CBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	cbackend.SetSemanticProgram(c.SemanticInfo)
	defer cbackend.SetSemanticProgram(nil)
	cbackend.ResetUnsafeHandlerCoverage()

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
	info.Includes = utils.DedupeStrings(info.Includes)
	info.CImportLibraries = utils.DedupeStrings(info.CImportLibraries)
	info.CImportObjects = utils.DedupeStrings(info.CImportObjects)
	info.ExternalRootSymbols = utils.DedupeStrings(info.ExternalRootSymbols)

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
	treeshakeEnabled := file.Config != nil && file.Config.Treeshake
	if treeshakeEnabled {
		warnings := cbackend.GetTreeshakeDynamicCallWarnings()
		if len(warnings) > 0 {
			treeshakeEnabled = false
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
	cCode := generateCCode(info, treeshakeEnabled)

	if c.Ctx.Bool("print-ir") || c.Ctx.Bool("print-expanded") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + cCode)
	}

	outFile := writeCFile(c, cCode, file)
	if outFile == "" {
		return nil
	}

	if c.Ctx.Bool("ir-only") {
		return nil
	}

	// Compile with gcc
	gccArgs := buildGCCArgs(c.SourceFile, info, outFile)
	cmd := exec.Command("gcc", gccArgs...)

	return cmd
}

// writeCFile writes the C source code to disk and returns the output file path.
// Returns empty string on failure.
func writeCFile(c *interfaces.BackendConfig, cCode string, scope *ast.Ast) string {
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
		scope.ErrorScope.NewCompileTimeError("File write error", "Error writing C file: "+err.Error(), lexer.Position{})
		return ""
	}

	if c.Ctx.Bool("ir-only") {
		// Copy generated C into project-local artifact directory when available.
		destFile := c.File + ".c"
		if c.SourceFile.Config != nil && c.SourceFile.Config.Project != nil {
			destFile = c.SourceFile.Config.Project.GetArtifactPath(c.File, ".c")
		}
		_ = os.MkdirAll(filepath.Dir(destFile), 0o755)

		cmd := exec.Command("cp", outFile, destFile)
		streamErr := utils.StreamCommand(cmd)

		if streamErr != nil {
			scope.ErrorScope.NewCompileTimeError("C copy", "Error copying C file "+streamErr.Error(), lexer.Position{})
		}
	}

	return outFile
}

// buildGCCArgs builds the gcc command line arguments for compiling the generated C file.
func buildGCCArgs(file *tokens.File, info *cbackend.CScopeInformation, outFile string) []string {
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

	return gccArgs
}
