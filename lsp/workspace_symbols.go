package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func appendWorkspaceSymbols(result *[]protocol.SymbolInformation, uri protocol.DocumentURI, symbols []protocol.DocumentSymbol, container, query string) {
	for _, symbol := range symbols {
		qualified := symbol.Name
		if container != "" {
			qualified = container + "::" + symbol.Name
		}
		if query == "" || strings.Contains(strings.ToLower(qualified), query) {
			*result = append(*result, protocol.SymbolInformation{
				Name: symbol.Name, Kind: symbol.Kind,
				Location:      protocol.Location{URI: uri, Range: symbol.SelectionRange},
				ContainerName: container,
			})
		}
		appendWorkspaceSymbols(result, uri, symbol.Children, qualified, query)
	}
}

func (s *Server) workspaceSymbols(ctx context.Context, query string) ([]protocol.SymbolInformation, error) {
	openFiles := s.documents.Snapshot()
	readSource := func(path string) ([]byte, error) {
		if content, ok := openFiles[filepath.Clean(path)]; ok {
			return []byte(content), nil
		}
		return os.ReadFile(path)
	}
	s.mu.Lock()
	roots := append([]string{}, s.workspaceRoots...)
	s.mu.Unlock()
	if len(roots) == 0 {
		for path := range openFiles {
			if filepath.Ext(path) != ".gecko" {
				continue
			}
			root, err := projectSourceRoot(path, readSource)
			if err == nil {
				roots = append(roots, root)
			} else {
				roots = append(roots, filepath.Dir(path))
			}
		}
	}
	paths := make(map[string]bool)
	for _, root := range roots {
		diskPaths, err := s.projectSourcePaths(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("listing workspace symbols: %w", err)
		}
		for _, path := range diskPaths {
			paths[path] = true
		}
	}
	for path := range openFiles {
		for _, root := range roots {
			if filepath.Ext(path) == ".gecko" && withinDirectory(root, path) {
				paths[filepath.Clean(path)] = true
				break
			}
		}
	}
	result := make([]protocol.SymbolInformation, 0)
	query = strings.ToLower(query)
	for path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := readSource(path)
		if err != nil {
			return nil, fmt.Errorf("reading workspace source %s: %w", path, err)
		}
		content := string(data)
		var file *tokens.File
		doc, open := s.documents.Get(pathToURI(path))
		if open && doc.Analysis != nil && doc.Analysis.MainFile != nil && doc.Analysis.MainFile.Content == content {
			file = doc.Analysis.MainFile
		} else {
			source, parseErr := s.documents.frontendSession.Syntax(path, content)
			if parseErr == nil {
				file = source.File
			} else if open && doc.Analysis != nil {
				file = doc.Analysis.MainFile
			}
		}
		if file == nil {
			continue
		}
		appendWorkspaceSymbols(&result, pathToURI(path), documentSymbols(file, content), "", query)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		if result[i].Location.URI != result[j].Location.URI {
			return result[i].Location.URI < result[j].Location.URI
		}
		return result[i].Location.Range.Start.Line < result[j].Location.Range.Start.Line
	})
	return result, nil
}

func (s *Server) handleWorkspaceSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.WorkspaceSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	result, err := s.workspaceSymbols(ctx, params.Query)
	if errors.Is(err, context.Canceled) {
		err = protocol.ErrRequestCancelled
	}
	return reply(ctx, result, err)
}
