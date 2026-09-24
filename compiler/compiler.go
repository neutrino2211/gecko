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
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/gecko/utils"
	"github.com/neutrino2211/go-option"
)

func Compile(file string, config *config.CompileCfg) string {
	fileOpt := option.SomePair(os.ReadFile(file))
	fileContents := fileOpt.Expect("Unable to read file '" + file + "'")
	compileErrorScope := errors.NewErrorScope("compile", file, string(fileContents))

	sourceFile, tokenError := parser.Parser.ParseString(file, string(fileContents))

	sourceFile.Content = string(fileContents)
	sourceFile.Path = file
	sourceFile.Config = config
	tokens.NormalizeWhereClauses(sourceFile)

	state := newCompileState()

	// Resolve imports
	baseDir := filepath.Dir(file)
	if baseDir == "" {
		baseDir = "."
	}
	resolveImports(sourceFile, baseDir, config, compileErrorScope, state)
	preloadDirectoryUseImports(sourceFile, state)
	emitLegacyInteropDeprecationWarnings(sourceFile)
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

	parseSyntaxError(tokenError, compileErrorScope)

	compilationBackend, ok := backends.Backends[backend]

	if !ok {
		message := "Backend '" + backend + "' not found."
		if backend == "llvm" {
			message += " The LLVM backend is deprecated and unavailable; use 'c'."
		}
		compileErrorScope.NewCompileTimeError("Backend Error", message, lexer.Position{Line: 1, Column: 1})
		return ""
	}

	if haveErrors() {
		return ""
	}

	semanticInfo := semantic.Analyze(sourceFile)
	emitSemanticDiagnostics(sourceFile, semanticInfo.Diagnostics())

	if haveErrors() {
		return ""
	}

	compilationBackend.Init()
	unsupportedCount := validateFeatures(sourceFile, compilationBackend, backend, compileErrorScope, config)

	if haveErrors() {
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

	// Create lazy type resolver for directory imports
	lazyResolver := func(typeName string) (*tokens.File, bool) {
		return resolveTypeFromDirectoryImports(sourceFile, typeName, state)
	}

	// Create lazy method resolver for directory imports
	lazyMethodResolver := func(methodName string) (*tokens.File, bool) {
		return resolveMethodFromDirectoryImports(sourceFile, methodName, state)
	}

	// Create lazy module type resolver for qualified types (e.g., shapes.Circle)
	lazyModuleTypeResolver := func(moduleName string, typeName string) (*tokens.File, bool) {
		return resolveModuleTypeFromDirectoryImports(sourceFile, moduleName, typeName, state)
	}

	// Initialize type registry for suggestions
	ResetTypeRegistry()
	registry := GetTypeRegistry()
	registry.ScanStdlib()
	registry.ScanProjectDirectory(filepath.Dir(file))

	// Create suggestion provider
	suggestionProvider := func(typeName string) string {
		return registry.FormatSuggestions(typeName)
	}

	cmd := compilationBackend.Compile(&interfaces.BackendConfig{
		OutName:                outName,
		File:                   file,
		Ctx:                    config.Ctx,
		SourceFile:             sourceFile,
		LazyTypeResolver:       lazyResolver,
		LazyMethodResolver:     lazyMethodResolver,
		LazyModuleTypeResolver: lazyModuleTypeResolver,
		SuggestionProvider:     suggestionProvider,
		SemanticInfo:           semanticInfo,
	})

	// Check for errors after codegen/type checking - bail early if any
	if haveErrors() {
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
