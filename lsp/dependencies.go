package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/frontend"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

type dependencyClosure struct {
	files       map[string]bool
	directories map[string]bool
}

func (closure *dependencyClosure) contains(path string) bool {
	if closure == nil {
		return false
	}
	path = filepath.Clean(path)
	if closure.files[path] {
		return true
	}
	for directory := range closure.directories {
		if withinDirectory(directory, path) {
			return true
		}
	}
	return false
}

func (ds *DocumentStore) InvalidateDependencies() {
	ds.mu.Lock()
	ds.dependencies = make(map[protocol.DocumentURI]*dependencyClosure)
	ds.dependencyRevision++
	ds.mu.Unlock()
}

func (ds *DocumentStore) Dependents(uri protocol.DocumentURI) []protocol.DocumentURI {
	ds.mu.RLock()
	docs := make([]Document, 0, len(ds.docs))
	contents := make(map[string]string, len(ds.docs))
	cached := make(map[protocol.DocumentURI]*dependencyClosure, len(ds.dependencies))
	for _, doc := range ds.docs {
		docs = append(docs, *doc)
		contents[filepath.Clean(uriToPath(string(doc.URI)))] = doc.Content
	}
	for key, closure := range ds.dependencies {
		cached[key] = closure
	}
	revision := ds.dependencyRevision
	ds.mu.RUnlock()
	readSource := func(path string) ([]byte, error) {
		if content, ok := contents[filepath.Clean(path)]; ok {
			return []byte(content), nil
		}
		return os.ReadFile(path)
	}
	target := filepath.Clean(uriToPath(string(uri)))
	result := make([]protocol.DocumentURI, 0)
	built := make(map[protocol.DocumentURI]*dependencyClosure)
	for _, doc := range docs {
		if doc.URI == uri || doc.Analysis == nil || doc.Analysis.MainFile == nil {
			continue
		}
		closure := cached[doc.URI]
		if closure == nil {
			closure = buildDependencyClosure(doc.Analysis.MainFile, uriToPath(string(doc.URI)), contents, readSource, ds.frontendSession)
			built[doc.URI] = closure
		}
		if closure != nil && closure.contains(target) {
			result = append(result, doc.URI)
		}
	}
	ds.mu.Lock()
	if ds.dependencyRevision == revision {
		for key, closure := range built {
			if current := ds.docs[key]; current != nil && closure != nil {
				ds.dependencies[key] = closure
			}
		}
		for key, closure := range ds.dependencies {
			if key == uri || closure.contains(target) {
				delete(ds.dependencies, key)
			}
		}
	}
	ds.mu.Unlock()
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func buildDependencyClosure(file *tokens.File, path string, openFiles map[string]string, readSource func(string) ([]byte, error), session *frontend.Session) *dependencyClosure {
	project, err := config.LoadProjectConfigWithReader(filepath.Dir(path), readSource)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	closure := &dependencyClosure{files: make(map[string]bool), directories: make(map[string]bool)}
	cfg := &config.CompileCfg{Project: project, ReadSource: readSource}
	visited := make(map[string]bool)
	closure.walk(file, path, openFiles, cfg, readSource, session, visited)
	return closure
}

func (closure *dependencyClosure) walk(file *tokens.File, path string, openFiles map[string]string, cfg *config.CompileCfg, readSource func(string) ([]byte, error), session *frontend.Session, visited map[string]bool) {
	path = filepath.Clean(path)
	if file == nil || visited[path] {
		return
	}
	visited[path] = true
	for _, entry := range file.Entries {
		if entry == nil || entry.Import == nil {
			continue
		}
		location := compiler.ResolveImportLocation(path, entry.Import.Package(), cfg)
		if location.IsDirectory {
			directory := filepath.Clean(location.DirPath)
			closure.directories[directory] = true
			closure.walkDirectory(directory, openFiles, cfg, readSource, session, visited)
		} else if location.FilePath != "" {
			closure.walkFile(location.FilePath, openFiles, cfg, readSource, session, visited)
		}
	}
}

func (closure *dependencyClosure) walkDirectory(directory string, openFiles map[string]string, cfg *config.CompileCfg, readSource func(string) ([]byte, error), session *frontend.Session, visited map[string]bool) {
	paths := make(map[string]bool)
	entries, err := os.ReadDir(directory)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".gecko" {
				paths[filepath.Join(directory, entry.Name())] = true
			}
		}
	}
	for path := range openFiles {
		if filepath.Dir(path) == directory && filepath.Ext(path) == ".gecko" {
			paths[path] = true
		}
	}
	for path := range paths {
		closure.walkFile(path, openFiles, cfg, readSource, session, visited)
	}
}

func (closure *dependencyClosure) walkFile(path string, openFiles map[string]string, cfg *config.CompileCfg, readSource func(string) ([]byte, error), session *frontend.Session, visited map[string]bool) {
	path = filepath.Clean(path)
	closure.files[path] = true
	if visited[path] {
		return
	}
	data, err := readSource(path)
	if err != nil {
		return
	}
	source, err := session.Syntax(path, string(data))
	if err != nil {
		return
	}
	closure.walk(source.File, path, openFiles, cfg, readSource, session, visited)
}
