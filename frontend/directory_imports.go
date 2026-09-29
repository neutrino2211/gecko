package frontend

import (
	"os"
	"path/filepath"

	"github.com/neutrino2211/gecko/tokens"
)

func ResolveTypeFromDirectoryImports(file *tokens.File, name string) (*tokens.File, bool) {
	return resolveTypeFromDirectoryImports(file, name, newCompileState())
}

func ResolveModuleTypeFromDirectoryImports(file *tokens.File, module, name string) (*tokens.File, bool) {
	return resolveModuleTypeFromDirectoryImports(file, module, name, newCompileState())
}

func ResolveMethodFromDirectoryImports(file *tokens.File, name string) (*tokens.File, bool) {
	return resolveMethodFromDirectoryImports(file, name, newCompileState())
}

func resolveTypeFromDirectoryImports(file *tokens.File, name string, state *compileState) (*tokens.File, bool) {
	return resolveDirectorySymbol(file, "", name, false, state)
}

func resolveModuleTypeFromDirectoryImports(file *tokens.File, module, name string, state *compileState) (*tokens.File, bool) {
	return resolveDirectorySymbol(file, module, name, false, state)
}

func resolveMethodFromDirectoryImports(file *tokens.File, name string, state *compileState) (*tokens.File, bool) {
	return resolveDirectorySymbol(file, "", name, true, state)
}

func directoryAllows(imported *tokens.DirectoryImport, module, name string) bool {
	actualModule := imported.Alias
	if actualModule == "" {
		actualModule = moduleNameFromImportPath(imported.Path)
	}
	if module != "" && actualModule != module {
		return false
	}
	if len(imported.UseObjects) == 0 {
		return true
	}
	for _, selected := range imported.UseObjects {
		if selected == name {
			return true
		}
	}
	return false
}

func resolveDirectorySymbol(file *tokens.File, module, name string, method bool, state *compileState) (*tokens.File, bool) {
	if file == nil {
		return nil, false
	}
	for _, imported := range file.DirectoryImports {
		if imported == nil || !directoryAllows(imported, module, name) {
			continue
		}
		moduleName := imported.Alias
		if moduleName == "" {
			moduleName = moduleNameFromImportPath(imported.Path)
		}
		key := "directory\x00" + normalizePath(file.Path) + "\x00" + file.Name + "\x00" + normalizePath(imported.DirPath) + "\x00" + moduleName + "\x00" + name
		if method {
			key += "\x00method"
		}
		if cached := state.importCache[key]; cached != nil {
			appendImportIfMissing(file, cached)
			return cached, true
		}
		loaded := findDirectorySymbol(imported.DirPath, name, method, file, state)
		if loaded == nil {
			continue
		}
		for _, existing := range file.Imports {
			if normalizePath(existing.Path) == normalizePath(loaded.Path) && existing.Name == moduleName {
				state.importCache[key] = existing
				return existing, true
			}
		}
		copy := *loaded
		copy.Name = moduleName
		copy.Alias = imported.Alias
		state.importCache[key] = &copy
		appendImportIfMissing(file, &copy)
		scope := state.newErrorScope("import", copy.Path, copy.Content)
		resolveImports(&copy, filepath.Dir(copy.Path), file.Config, scope, state)
		if !state.preloading {
			preloadDirectoryReferences(&copy, state)
		}
		return &copy, true
	}
	return nil, false
}

func findDirectorySymbol(directory, name string, method bool, owner *tokens.File, state *compileState) *tokens.File {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".gecko" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		file, err := state.loadFile(path, owner.Config)
		if err != nil {
			continue
		}
		for _, entry := range file.Entries {
			if method && entry.Method != nil && entry.Method.Name == name {
				return file
			}
			if !method && (entry.Class != nil && entry.Class.Name == name || entry.Trait != nil && entry.Trait.Name == name || entry.Enum != nil && entry.Enum.Name == name) {
				return file
			}
		}
	}
	return nil
}

func preloadDirectoryReferences(file *tokens.File, state *compileState) {
	if file == nil || state.preloading {
		return
	}
	state.preloading = true
	defer func() { state.preloading = false }()
	visited := make(map[*tokens.File]bool)
	var preload func(*tokens.File)
	preload = func(current *tokens.File) {
		if current == nil || visited[current] {
			return
		}
		visited[current] = true
		for _, imported := range current.DirectoryImports {
			if imported == nil {
				continue
			}
			for _, name := range imported.UseObjects {
				if name == "" {
					continue
				}
				if _, ok := resolveMethodFromDirectoryImports(current, name, state); !ok {
					resolveTypeFromDirectoryImports(current, name, state)
				}
			}
		}
		for _, ref := range directoryReferences(current) {
			resolveDirectorySymbol(current, ref.module, ref.name, ref.method, state)
		}
		for _, imported := range current.Imports {
			preload(imported)
		}
	}
	preload(file)
}
