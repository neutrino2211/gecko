// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md

package compiler

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/fatih/color"
	"github.com/neutrino2211/gecko/backends"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	llvmbackend "github.com/neutrino2211/gecko/backends/llvm_backend"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/go-option"
)

func streamPipe(std io.ReadCloser) {
	defer std.Close()
	buf := bufio.NewReader(std) // Notice that this is not in a loop
	for {

		line, _, err := buf.ReadLine()
		if err != nil {
			break
		}
		fmt.Println(string(line))
	}
}

func streamCommand(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		streamPipe(stdout)
	}()

	go func() {
		defer wg.Done()
		streamPipe(stderr)
	}()

	wg.Wait()
	return cmd.Wait()
}

type compileState struct {
	importCache map[string]*tokens.File
	fileCache   map[string]*tokens.File
}

func newCompileState() *compileState {
	return &compileState{
		importCache: make(map[string]*tokens.File),
		fileCache:   make(map[string]*tokens.File),
	}
}

func normalizePath(path string) string {
	if absPath, err := filepath.Abs(path); err == nil {
		return filepath.Clean(absPath)
	}
	return filepath.Clean(path)
}

func importCacheKey(importerDir, importPath string) string {
	return normalizePath(importerDir) + "|" + importPath
}

func splitImportPath(importPath string) []string {
	if importPath == "" {
		return nil
	}
	parts := strings.Split(importPath, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func moduleNameFromImportPath(importPath string) string {
	parts := splitImportPath(importPath)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func appendImportIfMissing(sourceFile *tokens.File, imported *tokens.File) {
	if sourceFile == nil || imported == nil {
		return
	}

	importedPath := normalizePath(imported.Path)
	for _, existing := range sourceFile.Imports {
		if normalizePath(existing.Path) == importedPath {
			return
		}
	}
	sourceFile.Imports = append(sourceFile.Imports, imported)
}

// getGeckoHome returns the path to the Gecko home directory (parent of stdlib).
// Resolution order: $GECKO_HOME env var -> system paths -> current directory
func getGeckoHome() string {
	// Helper to check if a path has std library
	hasStd := func(path string) bool {
		// Check for new "std" path first, fall back to legacy "stdlib"
		if _, err := os.Stat(filepath.Join(path, "std")); err == nil {
			return true
		}
		if _, err := os.Stat(filepath.Join(path, "stdlib")); err == nil {
			return true
		}
		return false
	}

	if home := os.Getenv("GECKO_HOME"); home != "" && hasStd(home) {
		return home
	}

	// Check system paths based on OS
	switch runtime.GOOS {
	case "darwin", "linux":
		if hasStd("/usr/local/lib/gecko") {
			return "/usr/local/lib/gecko"
		}
		if home := os.Getenv("HOME"); home != "" {
			userPath := filepath.Join(home, ".gecko")
			if hasStd(userPath) {
				return userPath
			}
		}
	case "windows":
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			winPath := filepath.Join(appData, "gecko")
			if hasStd(winPath) {
				return winPath
			}
		}
	}

	// Fallback: use current working directory
	if wd, err := os.Getwd(); err == nil && hasStd(wd) {
		return wd
	}
	return "."
}

// ModuleLocation represents where a module was found
type ModuleLocation struct {
	FilePath    string // Path to the .gecko file (empty if directory)
	DirPath     string // Path to directory (empty if file)
	IsDirectory bool   // True if this is a directory import
}

func resolveProjectPaths(baseDir string, cfg *config.CompileCfg) (string, string) {
	if cfg != nil && cfg.Project != nil {
		return cfg.Project.ProjectRoot, cfg.Project.GetDepsDir()
	}

	if projectCfg, err := config.LoadProjectConfig(baseDir); err == nil && projectCfg != nil {
		return projectCfg.ProjectRoot, projectCfg.GetDepsDir()
	}

	projectRoot := baseDir
	if cfg != nil && cfg.Ctx != nil {
		if args := cfg.Ctx.Args(); args.Len() > 0 {
			if mainFile := args.First(); mainFile != "" {
				projectRoot = filepath.Dir(mainFile)
			}
		}
	}

	return projectRoot, filepath.Join(projectRoot, ".gecko", "deps")
}

func resolveImportLocation(baseDir string, pathComponents []string, cfg *config.CompileCfg) ModuleLocation {
	if len(pathComponents) == 0 {
		return ModuleLocation{}
	}

	geckoHome := getGeckoHome()
	projectRoot, depsPath := resolveProjectPaths(baseDir, cfg)
	vendorPath := filepath.Join(projectRoot, "vendor")

	relativePath := filepath.Join(pathComponents...)

	var searchPaths []string
	if pathComponents[0] == "std" {
		relativePath = filepath.Join(pathComponents[1:]...)
		searchPaths = []string{
			filepath.Join(geckoHome, "std"),
			filepath.Join(geckoHome, "stdlib"),
		}
	} else {
		searchPaths = []string{
			baseDir,
			projectRoot,
			depsPath,
			vendorPath,
		}
	}

	return findModulePath(relativePath, searchPaths)
}

// ResolveImportLocation resolves a dot-notation import path using compiler resolution rules.
func ResolveImportLocation(importerFilePath string, importPath string, cfg *config.CompileCfg) ModuleLocation {
	baseDir := filepath.Dir(importerFilePath)
	if baseDir == "" {
		baseDir = "."
	}
	return resolveImportLocation(baseDir, splitImportPath(importPath), cfg)
}

// findModulePath searches for a module in the given search paths.
// For each search path, it tries: direct file (path.gecko), directory module (path/mod.gecko), or directory.
func findModulePath(relativePath string, searchPaths []string) ModuleLocation {
	for _, searchPath := range searchPaths {
		// Try direct file first
		filePath := filepath.Join(searchPath, relativePath+".gecko")
		if _, err := os.Stat(filePath); err == nil {
			return ModuleLocation{FilePath: filePath}
		}

		// Try directory with mod.gecko
		modPath := filepath.Join(searchPath, relativePath, "mod.gecko")
		if _, err := os.Stat(modPath); err == nil {
			return ModuleLocation{FilePath: modPath}
		}

		// Try as directory (for lazy resolution)
		dirPath := filepath.Join(searchPath, relativePath)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			return ModuleLocation{DirPath: dirPath, IsDirectory: true}
		}
	}
	return ModuleLocation{}
}

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

// resolveImports finds and parses imported modules
// Resolution order: relative to importer, project root, deps, vendor, and stdlib for `std.*`.
func resolveImports(sourceFile *tokens.File, baseDir string, cfg *config.CompileCfg, importErrorScope *errors.ErrorScope, state *compileState) {
	for _, entry := range sourceFile.Entries {
		if entry.Import == nil {
			continue
		}

		moduleName := entry.Import.ModuleName()
		fullPath := entry.Import.Package()
		cacheKey := importCacheKey(baseDir, fullPath)

		// Skip if already resolved for this importer+path.
		if importedFile, ok := state.importCache[cacheKey]; ok {
			appendImportIfMissing(sourceFile, importedFile)
			continue
		}

		location := resolveImportLocation(baseDir, entry.Import.Path, cfg)

		// Handle directory imports (lazy resolution)
		if location.IsDirectory {
			dirImport := &tokens.DirectoryImport{
				Path:       fullPath,
				DirPath:    location.DirPath,
				UseObjects: entry.Import.Objects,
			}
			sourceFile.DirectoryImports = append(sourceFile.DirectoryImports, dirImport)
			continue
		}

		if location.FilePath == "" {
			if importErrorScope != nil {
				importErrorScope.NewCompileTimeError(
					"Import Resolution Error",
					"Unable to resolve import '"+fullPath+"'",
					entry.Import.Pos,
				)
			}
			continue // Module not found, will be handled as error later
		}

		normalizedFilePath := normalizePath(location.FilePath)
		moduleFile, ok := state.fileCache[normalizedFilePath]
		if !ok {
			moduleContents, err := os.ReadFile(location.FilePath)
			if err != nil {
				if importErrorScope != nil {
					importErrorScope.NewCompileTimeError(
						"Import Read Error",
						"Unable to read module '"+fullPath+"': "+err.Error(),
						entry.Import.Pos,
					)
				}
				continue
			}

			parsedModule, parseErr := parser.Parser.ParseString(location.FilePath, string(moduleContents))
			if parseErr != nil {
				if importErrorScope != nil {
					importErrorScope.NewCompileTimeError(
						"Import Parse Error",
						"Unable to parse module '"+fullPath+"': "+parseErr.Error(),
						entry.Import.Pos,
					)
				}
				continue
			}

			parsedModule.Content = string(moduleContents)
			parsedModule.Path = location.FilePath
			parsedModule.Config = cfg

			moduleFile = parsedModule
			state.fileCache[normalizedFilePath] = moduleFile
		}

		moduleFile.Name = moduleName
		moduleFile.Config = cfg

		state.importCache[cacheKey] = moduleFile
		appendImportIfMissing(sourceFile, moduleFile)

		// Recursively resolve imports in the module
		moduleDir := filepath.Dir(location.FilePath)
		moduleImportScope := errors.NewErrorScope("import", moduleFile.Path, moduleFile.Content)
		resolveImports(moduleFile, moduleDir, cfg, moduleImportScope, state)
	}
}

func Compile(file string, config *config.CompileCfg) string {
	fileOpt := option.SomePair(os.ReadFile(file))
	fileContents := fileOpt.Expect("Unable to read file '" + file + "'")
	compileErrorScope := errors.NewErrorScope("compile", file, string(fileContents))

	sourceFile, tokenError := parser.Parser.ParseString(file, string(fileContents))

	sourceFile.Content = string(fileContents)
	sourceFile.Path = file
	sourceFile.Config = config

	state := newCompileState()

	// Resolve imports
	baseDir := filepath.Dir(file)
	if baseDir == "" {
		baseDir = "."
	}
	resolveImports(sourceFile, baseDir, config, compileErrorScope, state)

	ts := strconv.Itoa(int(time.Now().UnixNano()))

	buildDir := os.TempDir() + "/gecko/build/" + ts

	outName := buildDir + "/" + file + ".ll"
	compiledName := buildDir + "/" + file + ".o"

	// Check for @backend attribute in the file, fall back to CLI flag
	backend := sourceFile.GetBackend()
	if backend == "" {
		backend = config.Ctx.String("backend")
	}

	if tokenError != nil {
		errorMsg := tokenError.Error()
		var line, column int = 1, 1

		// Try to extract position from Participle error format: "filename:line:col: message"
		// or just "line:col: message"
		parts := strings.SplitN(errorMsg, ":", 4)
		if len(parts) >= 3 {
			// Check if first part is a number (line) or filename
			if l, err := strconv.Atoi(parts[0]); err == nil {
				line = l
				if c, err := strconv.Atoi(parts[1]); err == nil {
					column = c
				}
			} else if len(parts) >= 4 {
				// Format is "filename:line:col: message"
				if l, err := strconv.Atoi(parts[1]); err == nil {
					line = l
					if c, err := strconv.Atoi(parts[2]); err == nil {
						column = c
					}
				}
			}
		}

		compileErrorScope.NewCompileTimeError(
			"Syntax Error",
			errorMsg,
			lexer.Position{
				Line:   line,
				Column: column,
			},
		)
	}

	compilationBackend, ok := backends.Backends[backend]

	if !ok {
		println(color.RedString("Backend '" + backend + "' not found."))
		os.Exit(0)
	}

	// Validate that the file only uses features supported by the backend
	compilationBackend.Init()
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

	// Warn about experimental LLVM backend with feature-set-derived unsupported usage.
	if backend == "llvm" {
		println(color.YellowString("Warning: LLVM backend is experimental and incomplete."))
		if len(unsupportedFeatures) > 0 {
			featureNames := make([]string, 0, len(unsupportedFeatures))
			for _, feature := range unsupportedFeatures {
				featureNames = append(featureNames, string(feature))
			}
			println(color.YellowString("  Unsupported in effective import closure: " + strings.Join(featureNames, ", ") + "."))
		} else {
			println(color.YellowString("  No unsupported features were detected in the effective import closure by feature gating."))
		}
		println(color.YellowString("  For production use, prefer the C backend (--backend c)."))
		println()
	}

	for _, feature := range unsupportedFeatures {
		msg := "Feature '" + string(feature) + "' is not supported by the '" + backend + "' backend"
		if origins, ok := featureSources[feature]; ok && len(origins) > 0 {
			msg += " (used in: " + strings.Join(origins, ", ") + ")"
		}
		if backend == "llvm" {
			msg += " (prefer '--backend c' for this file)"
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
			importBackend = config.Ctx.String("backend")
		}

		if importBackend != backend && !featureSet.CanImportFrom(importBackend) {
			compileErrorScope.NewCompileTimeError(
				"Incompatible Import",
				"Cannot import '"+importedFile.Name+"' (backend: "+importBackend+") into file using '"+backend+"' backend. These backends are not compatible.",
				lexer.Position{Line: 1, Column: 1},
			)
		}
	}

	if haveErrors() {
		return ""
	}

	os.MkdirAll(buildDir, 0o755)

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
		err = streamCommand(cmd)
	}

	if err != nil {
		msg := "Error compiling for backend '" + backend + "' " + err.Error()
		if backend == "llvm" && strings.Contains(err.Error(), "\"llc\"") && strings.Contains(err.Error(), "executable file not found") {
			msg = "LLVM toolchain error: llc not found in PATH"
		}
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

func expectedArtifactPath(backend, sourceFile, irPath, objPath string, cfg *config.CompileCfg) string {
	irOnly := cfg != nil && cfg.Ctx != nil && cfg.Ctx.Bool("ir-only")

	switch backend {
	case "llvm":
		if irOnly {
			return irPath
		}
		return objPath
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

func haveErrors() bool {
	for _, e := range errors.GetAllScopes() {
		if e.HasErrors() {
			return true
		}
	}

	return false
}

// DiagnosticMessage represents an error or warning for external consumers (like LSP)
type DiagnosticMessage struct {
	Line    int
	Column  int
	Message string
	Title   string
}

// ResetErrorScopes clears all error scopes (useful for LSP between checks)
func ResetErrorScopes() {
	ResetCompilationState()
}

// ResetCompilationState clears all compiler/backend global state.
// Useful for long-lived processes (e.g. LSP) between checks.
func ResetCompilationState() {
	errors.ResetScopes()
	ResetTypeRegistry()
	cbackend.ResetState()
	llvmbackend.ResetState()
	hooks.ResetHookRegistry()
}

// GetAllErrors returns all errors from all scopes
func GetAllErrors() []DiagnosticMessage {
	var result []DiagnosticMessage
	for _, scope := range errors.GetAllScopes() {
		for _, err := range scope.CompileTimeErrors {
			result = append(result, DiagnosticMessage{
				Line:    err.Pos.Line,
				Column:  err.Pos.Column,
				Message: err.Title + ": " + err.Message,
				Title:   err.Title,
			})
		}
	}
	return result
}

// GetAllWarnings returns all warnings from all scopes
func GetAllWarnings() []DiagnosticMessage {
	var result []DiagnosticMessage
	for _, scope := range errors.GetAllScopes() {
		for _, warn := range scope.CompileTimeWarnings {
			result = append(result, DiagnosticMessage{
				Line:    warn.Pos.Line,
				Column:  warn.Pos.Column,
				Message: warn.Title + ": " + warn.Message,
				Title:   warn.Title,
			})
		}
	}
	return result
}

func PrintErrorSummary() bool {
	var warnings, errCount int = 0, 0
	var bold, boldYellow, boldRed *color.Color = color.New(color.Bold), color.New(color.Bold, color.FgHiYellow), color.New(color.Bold, color.FgHiRed)
	for _, e := range errors.GetAllScopes() {
		if e.HasWarnings() {
			for _, e := range e.CompileTimeWarnings {
				fmt.Println(e.GetWarning())
			}
		}

		if e.HasErrors() {
			for _, e := range e.CompileTimeErrors {
				fmt.Println(e.GetError())
			}
		}

		fmt.Println(e.GetSummary() + "\n")

		errCount += len(e.CompileTimeErrors)
		warnings += len(e.CompileTimeWarnings)
	}

	fmt.Print(
		bold.Sprint("\nTotal of ") +
			boldYellow.Sprintf("%d warnings", warnings) +
			bold.Sprint(" and ") +
			boldRed.Sprintf("%d errors", errCount) +
			bold.Sprint(" generated\n"),
	)

	return errCount > 0
}
