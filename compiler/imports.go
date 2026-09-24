// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md

package compiler

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
)

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
		if normalizePath(existing.Path) == importedPath && existing.Alias == imported.Alias {
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
		if entry.Import.Alias != "" {
			cacheKey += "|as:" + entry.Import.Alias
		}

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
				Alias:      entry.Import.Alias,
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
		cachedFile, ok := state.fileCache[normalizedFilePath]
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
			tokens.NormalizeWhereClauses(parsedModule)

			cachedFile = parsedModule
			state.fileCache[normalizedFilePath] = cachedFile
		}

		// When an alias is used, shallow-copy the cached file so the alias
		// name does not clobber the shared module identity for other importers.
		moduleFile := cachedFile
		if entry.Import.Alias != "" && cachedFile.Name != entry.Import.Alias {
			copiedFile := *cachedFile
			moduleFile = &copiedFile
		}

		moduleFile.Name = moduleName
		moduleFile.Alias = entry.Import.Alias
		moduleFile.Config = cfg

		state.importCache[cacheKey] = moduleFile
		appendImportIfMissing(sourceFile, moduleFile)

		// Recursively resolve imports in the module
		moduleDir := filepath.Dir(location.FilePath)
		moduleImportScope := errors.NewErrorScope("import", moduleFile.Path, moduleFile.Content)
		resolveImports(moduleFile, moduleDir, cfg, moduleImportScope, state)
	}
}
