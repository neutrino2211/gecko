package compiler

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/fatih/color"
	"github.com/neutrino2211/gecko/backends"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/go-option"
)

func streamPipe(std io.ReadCloser) {
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
	streamPipe(stdout)
	streamPipe(stderr)

	return nil
}

var parsedFiles = make(map[string]*tokens.File)

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
			// Cache the parsed file
			fullPath := dirImport.Path + "." + file.Name
			if _, exists := parsedFiles[fullPath]; !exists {
				parsedFiles[fullPath] = file
				sourceFile.Imports = append(sourceFile.Imports, file)
			}
			return file, true
		}
	}
	return nil, false
}

// ResolveModuleTypeFromDirectoryImports searches a specific module (directory import) for a type.
// This is called for module-qualified types like shapes.Circle.
func ResolveModuleTypeFromDirectoryImports(sourceFile *tokens.File, moduleName string, typeName string) (*tokens.File, bool) {
	for _, dirImport := range sourceFile.DirectoryImports {
		// Check if this directory import matches the module name
		// The module name is the last part of the import path (e.g., "shapes" from "import shapes")
		importModuleName := filepath.Base(dirImport.Path)
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
			// Cache the parsed file
			fullPath := dirImport.Path + "." + file.Name
			if _, exists := parsedFiles[fullPath]; !exists {
				parsedFiles[fullPath] = file
				sourceFile.Imports = append(sourceFile.Imports, file)
			}
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
			// Cache the parsed file
			fullPath := dirImport.Path + "." + file.Name
			if _, exists := parsedFiles[fullPath]; !exists {
				parsedFiles[fullPath] = file
				sourceFile.Imports = append(sourceFile.Imports, file)
			}
			return file, true
		}
	}
	return nil, false
}

// resolveImports finds and parses imported modules
// Resolution order: 1) Relative to source file, 2) Stdlib ($GECKO_HOME/stdlib), 3) Vendor (./vendor)
func resolveImports(sourceFile *tokens.File, baseDir string, cfg *config.CompileCfg) {
	geckoHome := getGeckoHome()

	// Determine stdlib path - prefer "std" over legacy "stdlib"
	stdPath := filepath.Join(geckoHome, "std")
	if _, err := os.Stat(stdPath); os.IsNotExist(err) {
		stdPath = filepath.Join(geckoHome, "stdlib")
	}

	// Get project root and deps path from project config or fallback to baseDir
	var projectRoot string
	var depsPath string

	if cfg != nil && cfg.Project != nil {
		projectRoot = cfg.Project.ProjectRoot
		depsPath = cfg.Project.GetDepsDir()
	} else {
		// Fallback: try to determine from CLI args
		projectRoot = baseDir
		if cfg != nil && cfg.Ctx != nil {
			if args := cfg.Ctx.Args(); args.Len() > 0 {
				if mainFile := args.First(); mainFile != "" {
					projectRoot = filepath.Dir(mainFile)
				}
			}
		}
		depsPath = filepath.Join(projectRoot, ".gecko", "deps")
	}

	// Legacy vendor path for backwards compatibility
	vendorPath := filepath.Join(projectRoot, "vendor")

	for _, entry := range sourceFile.Entries {
		if entry.Import == nil {
			continue
		}

		moduleName := entry.Import.ModuleName()
		fullPath := entry.Import.Package()

		// Skip if already parsed
		if _, ok := parsedFiles[fullPath]; ok {
			sourceFile.Imports = append(sourceFile.Imports, parsedFiles[fullPath])
			continue
		}

		// Convert dot notation to file path: std.collections.vec -> std/collections/vec
		pathComponents := entry.Import.Path
		relativePath := filepath.Join(pathComponents...)

		// For stdlib imports (starting with "std"), skip relative search
		// and go directly to stdlib path
		var searchPaths []string
		if len(pathComponents) > 0 && pathComponents[0] == "std" {
			// Strip "std" prefix for stdlib lookup
			stdRelativePath := filepath.Join(pathComponents[1:]...)
			// Search both std/ and stdlib/ directories
			searchPaths = []string{
				filepath.Join(geckoHome, "std"),    // $GECKO_HOME/std/
				filepath.Join(geckoHome, "stdlib"), // $GECKO_HOME/stdlib/
			}
			relativePath = stdRelativePath
		} else {
			// Non-std imports: relative -> project root -> deps -> vendor
			searchPaths = []string{
				baseDir,     // Relative to current file
				projectRoot, // Project root (gecko.toml location)
				depsPath,    // .gecko/deps/
				vendorPath,  // Legacy vendor/ path
			}
		}

		location := findModulePath(relativePath, searchPaths)

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
			continue // Module not found, will be handled as error later
		}

		// Parse the module
		moduleContents, err := os.ReadFile(location.FilePath)
		if err != nil {
			continue
		}

		moduleFile, parseErr := parser.Parser.ParseString(location.FilePath, string(moduleContents))
		if parseErr != nil {
			continue
		}

		moduleFile.Content = string(moduleContents)
		moduleFile.Path = location.FilePath
		moduleFile.Name = moduleName
		moduleFile.Config = cfg

		parsedFiles[fullPath] = moduleFile
		sourceFile.Imports = append(sourceFile.Imports, moduleFile)

		// Recursively resolve imports in the module
		moduleDir := filepath.Dir(location.FilePath)
		resolveImports(moduleFile, moduleDir, cfg)
	}
}

func Compile(file string, config *config.CompileCfg) string {
	fileOpt := option.SomePair(os.ReadFile(file))
	fileContents := fileOpt.Expect("Unable to read file '" + file + "'")

	sourceFile, tokenError := parser.Parser.ParseString(file, string(fileContents))

	sourceFile.Content = string(fileContents)
	sourceFile.Path = file
	sourceFile.Config = config

	// Resolve imports
	baseDir := filepath.Dir(file)
	if baseDir == "" {
		baseDir = "."
	}
	resolveImports(sourceFile, baseDir, config)

	compileErrorScope := errors.NewErrorScope("compile", sourceFile.Name, string(fileContents))

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

	// Warn about experimental LLVM backend
	if backend == "llvm" {
		println(color.YellowString("Warning: LLVM backend is experimental and incomplete."))
		println(color.YellowString("  Missing features: generics, type inference, break/continue, intrinsics,"))
		println(color.YellowString("  drop traits, operator traits, type checking, narrowing, builtin traits."))
		println(color.YellowString("  For production use, prefer the C backend (--backend c)."))
		println()
	}

	// Validate that the file only uses features supported by the backend
	compilationBackend.Init()
	usedFeatures := backends.DetectFeatures(sourceFile)
	featureSet := compilationBackend.Features()

	for _, feature := range usedFeatures {
		if !featureSet.SupportsString(string(feature)) {
			compileErrorScope.NewCompileTimeError(
				"Unsupported Feature",
				"Feature '"+string(feature)+"' is not supported by the '"+backend+"' backend",
				lexer.Position{Line: 1, Column: 1},
			)
		}
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
		return ResolveTypeFromDirectoryImports(sourceFile, typeName)
	}

	// Create lazy method resolver for directory imports
	lazyMethodResolver := func(methodName string) (*tokens.File, bool) {
		return ResolveMethodFromDirectoryImports(sourceFile, methodName)
	}

	// Create lazy module type resolver for qualified types (e.g., shapes.Circle)
	lazyModuleTypeResolver := func(moduleName string, typeName string) (*tokens.File, bool) {
		return ResolveModuleTypeFromDirectoryImports(sourceFile, moduleName, typeName)
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
		compileErrorScope.NewCompileTimeError("Compilation Backend Error", "Error compiling for backend '"+backend+"' "+err.Error(), lexer.Position{})
	}

	return compiledName
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
	errors.ResetScopes()
	parsedFiles = make(map[string]*tokens.File)
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
