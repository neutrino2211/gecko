package compiler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/parser"
)

// TypeLocation represents where a type is defined
type TypeLocation struct {
	TypeName   string // e.g., "HashMap"
	ModulePath string // e.g., "std.collections.hash"
	FilePath   string // e.g., "/path/to/stdlib/collections/hash.gecko"
	IsPublic   bool   // Whether the type is exported
}

// TypeRegistry tracks available types across all modules
type TypeRegistry struct {
	types map[string][]TypeLocation // type name -> locations (may be in multiple modules)
}

// Global registry instance
var globalTypeRegistry *TypeRegistry

// GetTypeRegistry returns the global type registry, initializing if needed
func GetTypeRegistry() *TypeRegistry {
	if globalTypeRegistry == nil {
		globalTypeRegistry = &TypeRegistry{
			types: make(map[string][]TypeLocation),
		}
	}
	return globalTypeRegistry
}

// ResetTypeRegistry clears the registry (useful between compilations)
func ResetTypeRegistry() {
	globalTypeRegistry = nil
}

// Register adds a type to the registry
func (r *TypeRegistry) Register(loc TypeLocation) {
	r.types[loc.TypeName] = append(r.types[loc.TypeName], loc)
}

// Find returns all locations where a type is defined
func (r *TypeRegistry) Find(typeName string) []TypeLocation {
	return r.types[typeName]
}

// ScanStdlib scans the stdlib directory for type definitions
func (r *TypeRegistry) ScanStdlib() {
	geckoHome := getGeckoHome()
	stdlibPath := filepath.Join(geckoHome, "stdlib")

	r.scanDirectory(stdlibPath, "std")
}

// ScanProjectDirectory scans a project directory for type definitions
func (r *TypeRegistry) ScanProjectDirectory(projectDir string) {
	r.scanDirectory(projectDir, "")
}

// scanDirectory recursively scans a directory for .gecko files
func (r *TypeRegistry) scanDirectory(dir string, modulePrefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Recurse into subdirectory
			subPrefix := entry.Name()
			if modulePrefix != "" {
				subPrefix = modulePrefix + "." + entry.Name()
			}
			r.scanDirectory(path, subPrefix)
			continue
		}

		if filepath.Ext(entry.Name()) != ".gecko" {
			continue
		}

		// Parse the file to find type definitions
		contents, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		file, parseErr := parser.Parser.ParseString(path, string(contents))
		if parseErr != nil {
			continue
		}

		// Build module path from prefix and package name
		modulePath := modulePrefix
		if file.PackageName != "" {
			if modulePrefix != "" {
				// For stdlib: std + package "vec" = std.vec
				modulePath = modulePrefix + "." + file.PackageName
			} else {
				modulePath = file.PackageName
			}
		}

		// Find all type definitions
		for _, e := range file.Entries {
			if e.Class != nil {
				r.Register(TypeLocation{
					TypeName:   e.Class.Name,
					ModulePath: modulePath,
					FilePath:   path,
					IsPublic:   e.Class.Visibility == "public",
				})
			}
			if e.Trait != nil {
				r.Register(TypeLocation{
					TypeName:   e.Trait.Name,
					ModulePath: modulePath,
					FilePath:   path,
					IsPublic:   e.Trait.Visibility == "public",
				})
			}
		}
	}
}

// FormatSuggestions returns a formatted suggestion string for a missing type
func (r *TypeRegistry) FormatSuggestions(typeName string) string {
	locations := r.Find(typeName)
	if len(locations) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\nhelp: `" + typeName + "` was found in the following modules:\n")

	publicCount := 0
	var firstPublic TypeLocation
	for _, loc := range locations {
		visibility := ""
		if !loc.IsPublic {
			visibility = " (not public)"
		} else {
			publicCount++
			if publicCount == 1 {
				firstPublic = loc
			}
		}
		sb.WriteString("  - " + loc.ModulePath + visibility + "\n")
	}

	if publicCount == 1 {
		sb.WriteString("\nConsider adding: import " + firstPublic.ModulePath + " use { " + typeName + " }")
	}

	return sb.String()
}
