package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/semantic"
	"go.lsp.dev/protocol"
)

type symbolDeclaration struct {
	path   string
	offset int
}

func declarationFor(graph *semantic.Program, id int64) (symbolDeclaration, bool) {
	for _, occurrence := range graph.OccurrencesFor(id, true) {
		if occurrence.Declaration {
			return symbolDeclaration{path: filepath.Clean(occurrence.FilePath), offset: occurrence.Start}, true
		}
	}
	return symbolDeclaration{}, false
}

func hasIdentifier(content, name string) bool {
	for offset := 0; offset < len(content); {
		if content[offset] == '/' && offset+1 < len(content) && content[offset+1] == '/' {
			for offset < len(content) && content[offset] != '\n' {
				offset++
			}
			continue
		}
		if content[offset] == '"' || content[offset] == '`' {
			quote := content[offset]
			offset++
			for offset < len(content) && content[offset] != quote {
				if content[offset] == '\\' && quote == '"' {
					offset++
				}
				offset++
			}
			offset++
			continue
		}
		if !identifierStart(content[offset]) {
			offset++
			continue
		}
		start := offset
		for offset < len(content) && identifierPart(content[offset]) {
			offset++
		}
		if content[start:offset] == name {
			return true
		}
	}
	return false
}

func projectSourceRoot(path string, readSource func(string) ([]byte, error)) (string, error) {
	project, err := config.LoadProjectConfigWithReader(filepath.Dir(path), readSource)
	if err == nil {
		return project.ProjectRoot, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return filepath.Dir(path), nil
	}
	return "", fmt.Errorf("finding project sources: %w", err)
}

func withinDirectory(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !filepath.IsAbs(relative) && (len(relative) < 3 || relative[:3] != ".."+string(filepath.Separator))
}

func withinAnyDirectory(roots []string, path string) bool {
	for _, root := range roots {
		if withinDirectory(root, path) {
			return true
		}
	}
	return false
}

func (s *Server) analysisForSource(path, content string, readSource func(string) ([]byte, error)) (*analysis.AnalysisContext, error) {
	if doc, open := s.documents.Get(pathToURI(path)); open && doc.Analysis != nil && doc.Analysis.MainFile != nil && doc.Analysis.MainFile.Content == content {
		return doc.Analysis, nil
	}
	return analysis.NewAnalysisContextWithSession(path, content, readSource, s.documents.frontendSession)
}

func (s *Server) workspaceReferences(ctx context.Context, doc *Document, position protocol.Position, includeDeclaration, strict bool) ([]protocol.Location, map[string]string, string, error) {
	path := filepath.Clean(uriToPath(string(doc.URI)))
	occurrence, symbol, ok := symbolAt(doc.Analysis, path, doc.Content, position)
	if !ok || symbol.Local {
		return nil, nil, "", nil
	}
	declaration, ok := declarationFor(doc.Analysis.SemanticGraph, occurrence.SymbolID)
	if !ok {
		return nil, nil, "", nil
	}
	openFiles := s.documents.Snapshot()
	readSource := func(candidate string) ([]byte, error) {
		if content, ok := openFiles[filepath.Clean(candidate)]; ok {
			return []byte(content), nil
		}
		return os.ReadFile(candidate)
	}
	root, err := projectSourceRoot(path, readSource)
	if err != nil {
		return nil, nil, "", err
	}
	root = filepath.Clean(root)
	s.mu.Lock()
	workspaceRoots := append([]string{}, s.workspaceRoots...)
	s.mu.Unlock()
	roots := []string{root}
	seenRoots := map[string]bool{root: true}
	for _, candidate := range workspaceRoots {
		candidate = filepath.Clean(candidate)
		if !seenRoots[candidate] {
			roots = append(roots, candidate)
			seenRoots[candidate] = true
		}
	}
	if strict && !withinAnyDirectory(roots, declaration.path) {
		return nil, nil, root, fmt.Errorf("cannot rename symbol declared outside workspace roots")
	}
	paths := make(map[string]bool)
	for _, sourceRoot := range roots {
		diskPaths, err := s.projectSourcePaths(ctx, sourceRoot)
		if err != nil {
			return nil, nil, root, fmt.Errorf("listing project sources in %s: %w", sourceRoot, err)
		}
		for _, candidate := range diskPaths {
			paths[candidate] = true
		}
	}
	for candidate := range openFiles {
		if filepath.Ext(candidate) == ".gecko" && withinAnyDirectory(roots, candidate) {
			paths[filepath.Clean(candidate)] = true
		}
	}
	locations := make(map[protocol.Location]bool)
	sources := make(map[string]string)
	for candidate := range paths {
		if err := ctx.Err(); err != nil {
			return nil, nil, root, err
		}
		data, err := readSource(candidate)
		if err != nil {
			return nil, nil, root, fmt.Errorf("reading %s: %w", candidate, err)
		}
		content := string(data)
		if !hasIdentifier(content, symbol.Name) {
			continue
		}
		sources[candidate] = content
		fileAnalysis, analyzeErr := s.analysisForSource(candidate, content, readSource)
		if analyzeErr != nil {
			if strict {
				return nil, nil, root, fmt.Errorf("analyzing %s: %w", candidate, analyzeErr)
			}
			continue
		}
		if strict && !completeLocalIndex(fileAnalysis.SemanticGraph, candidate, content, symbol.Name) {
			return nil, nil, root, fmt.Errorf("cannot rename %q while %s has unresolved uses", symbol.Name, candidate)
		}
		for _, found := range fileAnalysis.SemanticGraph.OccurrencesInFile(candidate) {
			if found.Declaration && !includeDeclaration {
				continue
			}
			key, ok := declarationFor(fileAnalysis.SemanticGraph, found.SymbolID)
			if !ok || key != declaration {
				continue
			}
			locations[protocol.Location{URI: pathToURI(candidate), Range: protocol.Range{
				Start: sourcePosition(content, found.Start), End: sourcePosition(content, found.End),
			}}] = true
		}
	}
	result := make([]protocol.Location, 0, len(locations))
	for location := range locations {
		result = append(result, location)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].URI != result[j].URI {
			return result[i].URI < result[j].URI
		}
		if result[i].Range.Start.Line != result[j].Range.Start.Line {
			return result[i].Range.Start.Line < result[j].Range.Start.Line
		}
		return result[i].Range.Start.Character < result[j].Range.Start.Character
	})
	return result, sources, root, nil
}

func (s *Server) renameWorkspaceSymbol(ctx context.Context, doc *Document, position protocol.Position, newName string) (*protocol.WorkspaceEdit, error) {
	if !validRename(newName) {
		return nil, fmt.Errorf("invalid Gecko identifier %q", newName)
	}
	_, symbol, ok := symbolAt(doc.Analysis, uriToPath(string(doc.URI)), doc.Content, position)
	if !ok || symbol.Local {
		return nil, nil
	}
	if symbol.Kind != semantic.SymbolFunction && symbol.Kind != semantic.SymbolClass && symbol.Kind != semantic.SymbolTrait && symbol.Kind != semantic.SymbolField && symbol.Kind != semantic.SymbolEnum && symbol.Kind != semantic.SymbolEnumCase {
		return nil, fmt.Errorf("workspace rename is not yet available for %s symbols", symbol.Kind)
	}
	locations, sources, _, err := s.workspaceReferences(ctx, doc, position, true, true)
	if err != nil {
		return nil, err
	}
	if len(locations) == 0 {
		return nil, fmt.Errorf("cannot find declaration for %q", symbol.Name)
	}
	if symbol.Name == newName {
		return &protocol.WorkspaceEdit{Changes: make(map[protocol.DocumentURI][]protocol.TextEdit)}, nil
	}
	for _, location := range locations {
		path := uriToPath(string(location.URI))
		if hasIdentifier(sources[path], newName) {
			return nil, fmt.Errorf("%q already appears in %s", newName, path)
		}
	}
	edit := &protocol.WorkspaceEdit{Changes: make(map[protocol.DocumentURI][]protocol.TextEdit)}
	for _, location := range locations {
		edit.Changes[location.URI] = append(edit.Changes[location.URI], protocol.TextEdit{Range: location.Range, NewText: newName})
	}
	return edit, nil
}
