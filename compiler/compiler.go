// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md

package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/backends"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/frontend"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/gecko/utils"
)

type Compilation struct {
	Artifact string
	Scopes   []*errors.ErrorScope
}

func Compile(file string, config *config.CompileCfg) string {
	return CompileWithDiagnostics(file, config).Artifact
}

func CompileWithDiagnostics(file string, config *config.CompileCfg) Compilation {
	fileContents, err := readSource(file, config)
	if err != nil {
		diagnostics := &errors.Collector{}
		scope := diagnostics.NewScope("compile", file, "")
		scope.NewCompileTimeError("Read Error", fmt.Sprintf("Unable to read file '%s': %v", file, err), lexer.Position{Line: 1, Column: 1})
		return Compilation{Scopes: diagnostics.Scopes()}
	}
	return CompileSourceWithDiagnostics(file, string(fileContents), config)
}

func CompileSource(file string, content string, config *config.CompileCfg) string {
	return CompileSourceWithDiagnostics(file, content, config).Artifact
}

func CompileSourceWithDiagnostics(file string, content string, config *config.CompileCfg) Compilation {
	return CompilePreparedWithDiagnostics(frontend.Analyze(file, content, config), config)
}

func CompilePrepared(result *frontend.Result, cfg *config.CompileCfg) string {
	return CompilePreparedWithDiagnostics(result, cfg).Artifact
}

func CompilePreparedWithDiagnostics(result *frontend.Result, cfg *config.CompileCfg) Compilation {
	diagnostics := &errors.Collector{}
	artifact := compileFrontend(result.CloneForBackend(cfg), cfg, diagnostics)
	return Compilation{Artifact: artifact, Scopes: diagnostics.Scopes()}
}

func compileFrontend(result *frontend.View, config *config.CompileCfg, diagnostics *errors.Collector) string {
	sourceFile := result.File
	if sourceFile == nil {
		scope := diagnostics.NewScope("compile", result.Path, result.Content)
		parseSyntaxError(result.ParseError, scope)
		return ""
	}
	file := sourceFile.Path
	compileErrorScope := diagnostics.NewScope("compile", file, sourceFile.Content)
	for _, scope := range result.Scopes {
		diagnostics.Add(scope)
	}
	emitLegacyInteropDeprecationWarnings(sourceFile, diagnostics)
	collectNativeLinkMetadata(sourceFile)

	ts := strconv.Itoa(int(time.Now().UnixNano()))

	buildDir := os.TempDir() + "/gecko/build/" + ts

	outName := buildDir + "/" + file + ".c"
	compiledName := buildDir + "/" + file + ".o"

	// Check for @backend attribute in the file, fall back to CLI flag
	backend := sourceFile.GetBackend()
	if backend == "" {
		backend = config.Ctx.String("backend")
	}

	parseSyntaxError(result.ParseError, compileErrorScope)

	compilationBackend, ok := backends.Backends[backend]

	if !ok {
		message := "Backend '" + backend + "' not found."
		if backend == "llvm" {
			message += " The LLVM backend is deprecated and unavailable; use 'c'."
		}
		compileErrorScope.NewCompileTimeError("Backend Error", message, lexer.Position{Line: 1, Column: 1})
		return ""
	}

	if diagnostics.HasErrors() {
		return ""
	}

	semanticInfo := result.Program
	emitSemanticDiagnostics(sourceFile, semanticInfo.Diagnostics(), diagnostics)

	if diagnostics.HasErrors() {
		return ""
	}

	tokens.UnsafeBlockBindNames, tokens.UnsafeBlockErrorNames = semanticInfo.UnsafeBlockNames()
	compilationBackend.Init()
	unsupportedCount := validateFeatures(sourceFile, compilationBackend, backend, compileErrorScope, config)

	if diagnostics.HasErrors() {
		return ""
	}

	// If unsupported features exist, that means we reported errors above.
	// Stop here if the LLVM backend has unsupported features detected in the import closure.
	if unsupportedCount > 0 {
		return ""
	}

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		compileErrorScope.NewCompileTimeError(
			"Build Directory Error",
			"Failed to create build directory '"+buildDir+"': "+err.Error(),
			lexer.Position{},
		)
		return ""
	}

	var registry *TypeRegistry
	suggestionProvider := func(typeName string) string {
		if registry == nil {
			registry = &TypeRegistry{types: make(map[string][]TypeLocation)}
			registry.ScanStdlib()
			registry.ScanProjectDirectory(filepath.Dir(file))
		}
		return registry.FormatSuggestions(typeName)
	}

	cmd := compilationBackend.Compile(&interfaces.BackendConfig{
		OutName:                outName,
		File:                   file,
		Ctx:                    config.Ctx,
		SourceFile:             sourceFile,
		LazyTypeResolver:       result.ResolveType,
		LazyMethodResolver:     result.ResolveMethod,
		LazyModuleTypeResolver: result.ResolveModuleType,
		SuggestionProvider:     suggestionProvider,
		SemanticInfo:           semanticInfo,
		Diagnostics:            diagnostics,
	})

	// Check for errors after codegen/type checking - bail early if any
	if diagnostics.HasErrors() {
		return ""
	}

	// For check-only mode, stop after type checking (don't run C compiler)
	if config.CheckOnly {
		return ""
	}

	var err error = nil

	if cmd != nil {
		err = utils.StreamCommand(cmd)
	}

	if err != nil {
		msg := "Error compiling for backend '" + backend + "' " + err.Error()
		compileErrorScope.NewCompileTimeError("Compilation Backend Error", msg, lexer.Position{})
		return ""
	}

	artifactPath := expectedArtifactPath(backend, file, outName, compiledName, config)
	if artifactPath == "" {
		return ""
	}

	if _, statErr := os.Stat(artifactPath); statErr != nil {
		compileErrorScope.NewCompileTimeError(
			"Compilation Backend Error",
			"Expected backend artifact '"+artifactPath+"' was not generated: "+statErr.Error(),
			lexer.Position{},
		)
		return ""
	}

	return artifactPath
}

func validateFeatures(sourceFile *tokens.File, compilationBackend interfaces.BackendInterface, backend string, compileErrorScope *errors.ErrorScope, cfg *config.CompileCfg) int {
	featureSet := compilationBackend.Features()
	unsupportedSet := make(map[backends.Feature]struct{})
	featureSources := make(map[backends.Feature][]string)

	fileDisplayName := func(file *tokens.File) string {
		if file == nil {
			return "<unknown>"
		}
		if file.Path != "" {
			return file.Path
		}
		if file.Name != "" {
			return file.Name
		}
		return "<anonymous>"
	}

	appendUniqueString := func(values []string, candidate string) []string {
		for _, v := range values {
			if v == candidate {
				return values
			}
		}
		return append(values, candidate)
	}

	visitedFiles := make(map[string]bool)
	filesInClosure := make([]*tokens.File, 0)
	var walkImportClosure func(file *tokens.File)
	walkImportClosure = func(file *tokens.File) {
		if file == nil {
			return
		}
		key := file.Path
		if key == "" {
			key = file.Name
		}
		if key == "" {
			key = fmt.Sprintf("%p", file)
		}
		if visitedFiles[key] {
			return
		}
		visitedFiles[key] = true
		filesInClosure = append(filesInClosure, file)
		for _, imported := range file.Imports {
			walkImportClosure(imported)
		}
	}
	walkImportClosure(sourceFile)

	for _, fileInClosure := range filesInClosure {
		usedFeatures := backends.DetectFeatures(fileInClosure)
		for _, feature := range usedFeatures {
			if !featureSet.SupportsString(string(feature)) {
				unsupportedSet[feature] = struct{}{}
				featureSources[feature] = appendUniqueString(featureSources[feature], fileDisplayName(fileInClosure))
			}
		}
	}

	unsupportedFeatures := make([]backends.Feature, 0, len(unsupportedSet))
	for feature := range unsupportedSet {
		unsupportedFeatures = append(unsupportedFeatures, feature)
	}
	sort.Slice(unsupportedFeatures, func(i, j int) bool {
		return unsupportedFeatures[i] < unsupportedFeatures[j]
	})

	for _, feature := range unsupportedFeatures {
		msg := "Feature '" + string(feature) + "' is not supported by the '" + backend + "' backend"
		if origins, ok := featureSources[feature]; ok && len(origins) > 0 {
			msg += " (used in: " + strings.Join(origins, ", ") + ")"
		}
		compileErrorScope.NewCompileTimeError(
			"Unsupported Feature",
			msg,
			lexer.Position{Line: 1, Column: 1},
		)
	}

	// Validate import compatibility
	for _, importedFile := range sourceFile.Imports {
		importBackend := importedFile.GetBackend()
		if importBackend == "" {
			importBackend = cfg.Ctx.String("backend")
		}

		if importBackend != backend && !featureSet.CanImportFrom(importBackend) {
			compileErrorScope.NewCompileTimeError(
				"Incompatible Import",
				"Cannot import '"+importedFile.Name+"' (backend: "+importBackend+") into file using '"+backend+"' backend. These backends are not compatible.",
				lexer.Position{Line: 1, Column: 1},
			)
		}
	}

	return len(unsupportedFeatures)
}

func expectedArtifactPath(backend, sourceFile, irPath, objPath string, cfg *config.CompileCfg) string {
	irOnly := cfg != nil && cfg.Ctx != nil && cfg.Ctx.Bool("ir-only")

	switch backend {
	case "c":
		if irOnly {
			if cfg != nil && cfg.Project != nil {
				return cfg.Project.GetArtifactPath(sourceFile, ".c")
			}
			return sourceFile + ".c"
		}
		return objPath
	default:
		if irOnly {
			return irPath
		}
		return objPath
	}
}
