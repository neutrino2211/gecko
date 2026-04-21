package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/neutrino2211/gecko/parser"
)

// StdlibExport represents a single exported symbol from the stdlib
type StdlibExport struct {
	Name       string // Symbol name (e.g., "Vec", "Box", "String")
	Kind       string // "class", "trait", "func", "const", "enum"
	ModulePath string // Full import path (e.g., "std.collections.vec")
	UsePath    string // Import with use clause (e.g., "std.collections.vec use { Vec }")
}

// StdlibIndex holds the indexed stdlib exports
type StdlibIndex struct {
	exports     []StdlibExport
	byName      map[string][]StdlibExport // Lookup by symbol name
	initialized bool
	mu          sync.RWMutex
}

var globalStdlibIndex = &StdlibIndex{
	byName: make(map[string][]StdlibExport),
}

// GetStdlibIndex returns the global stdlib index, initializing if needed
func GetStdlibIndex() *StdlibIndex {
	globalStdlibIndex.mu.RLock()
	if globalStdlibIndex.initialized {
		globalStdlibIndex.mu.RUnlock()
		return globalStdlibIndex
	}
	globalStdlibIndex.mu.RUnlock()

	globalStdlibIndex.mu.Lock()
	defer globalStdlibIndex.mu.Unlock()

	if !globalStdlibIndex.initialized {
		globalStdlibIndex.buildIndex()
		globalStdlibIndex.initialized = true
	}

	return globalStdlibIndex
}

// FindByName returns all exports matching the given name
func (idx *StdlibIndex) FindByName(name string) []StdlibExport {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byName[name]
}

// FindByPrefix returns all exports whose names start with the given prefix
func (idx *StdlibIndex) FindByPrefix(prefix string) []StdlibExport {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []StdlibExport
	for _, export := range idx.exports {
		if strings.HasPrefix(export.Name, prefix) {
			results = append(results, export)
		}
	}
	return results
}

// AllExports returns all indexed exports
func (idx *StdlibIndex) AllExports() []StdlibExport {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.exports
}

// buildIndex scans the stdlib directory and indexes all exports
func (idx *StdlibIndex) buildIndex() {
	stdlibPath := getStdlibPath()
	if stdlibPath == "" {
		return
	}

	filepath.Walk(stdlibPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".gecko") {
			return nil
		}
		// Skip mod.gecko files (they just re-export)
		if strings.HasSuffix(path, "mod.gecko") {
			return nil
		}

		idx.indexFile(path, stdlibPath)
		return nil
	})
}

// indexFile parses a single file and extracts its public exports
func (idx *StdlibIndex) indexFile(filePath, stdlibRoot string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	file, err := parser.Parser.ParseString(filePath, string(content))
	if err != nil || file == nil {
		return
	}

	// Convert file path to module path
	// e.g., /path/to/stdlib/collections/vec.gecko -> std.collections.vec
	relPath, err := filepath.Rel(stdlibRoot, filePath)
	if err != nil {
		return
	}
	relPath = strings.TrimSuffix(relPath, ".gecko")
	relPath = strings.ReplaceAll(relPath, string(filepath.Separator), ".")
	modulePath := "std." + relPath

	for _, entry := range file.Entries {
		if entry.Class != nil && isPublicVisibility(entry.Class.Visibility) {
			idx.addExport(entry.Class.Name, "class", modulePath)
		}
		if entry.Trait != nil && isPublicVisibility(entry.Trait.Visibility) {
			idx.addExport(entry.Trait.Name, "trait", modulePath)
		}
		if entry.Method != nil && isPublicVisibility(entry.Method.Visibility) {
			idx.addExport(entry.Method.Name, "func", modulePath)
		}
		// Field with const mutability is a constant
		if entry.Field != nil && entry.Field.Mutability == "const" && isPublicVisibility(entry.Field.Visibility) {
			idx.addExport(entry.Field.Name, "const", modulePath)
		}
		// Enums don't have visibility in Gecko, treat as public
		if entry.Enum != nil {
			idx.addExport(entry.Enum.Name, "enum", modulePath)
		}
	}
}

func (idx *StdlibIndex) addExport(name, kind, modulePath string) {
	export := StdlibExport{
		Name:       name,
		Kind:       kind,
		ModulePath: modulePath,
		UsePath:    modulePath + " use { " + name + " }",
	}
	idx.exports = append(idx.exports, export)
	idx.byName[name] = append(idx.byName[name], export)
}

func isPublicVisibility(vis string) bool {
	return vis == "public" || vis == "external"
}

// getStdlibPath returns the path to the stdlib directory
func getStdlibPath() string {
	// Check GECKO_HOME environment variable
	if geckoHome := os.Getenv("GECKO_HOME"); geckoHome != "" {
		stdlibPath := filepath.Join(geckoHome, "stdlib")
		if info, err := os.Stat(stdlibPath); err == nil && info.IsDir() {
			return stdlibPath
		}
	}

	// Check relative to executable
	if execPath, err := os.Executable(); err == nil {
		stdlibPath := filepath.Join(filepath.Dir(execPath), "stdlib")
		if info, err := os.Stat(stdlibPath); err == nil && info.IsDir() {
			return stdlibPath
		}
	}

	// Check current working directory (for development)
	if cwd, err := os.Getwd(); err == nil {
		stdlibPath := filepath.Join(cwd, "stdlib")
		if info, err := os.Stat(stdlibPath); err == nil && info.IsDir() {
			return stdlibPath
		}
		// Also check parent directory
		stdlibPath = filepath.Join(filepath.Dir(cwd), "stdlib")
		if info, err := os.Stat(stdlibPath); err == nil && info.IsDir() {
			return stdlibPath
		}
	}

	return ""
}
