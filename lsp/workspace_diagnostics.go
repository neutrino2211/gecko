package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/neutrino2211/gecko/frontend"
	"go.lsp.dev/protocol"
)

func (s *Server) beginDiagnosticRun(uri protocol.DocumentURI) uint64 {
	s.diagnosticMu.Lock()
	defer s.diagnosticMu.Unlock()
	s.diagnosticRuns[uri]++
	return s.diagnosticRuns[uri]
}

func (s *Server) workspaceContains(uri protocol.DocumentURI) bool {
	path := filepath.Clean(uriToPath(string(uri)))
	s.mu.Lock()
	roots := append([]string(nil), s.workspaceRoots...)
	s.mu.Unlock()
	for _, root := range roots {
		if withinDirectory(root, path) {
			return true
		}
	}
	return false
}

func (s *Server) publishDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found: %s", uri)
		return
	}
	s.checkDiagnostics(ctx, uri, doc.Content, uint32(doc.Version))
}

func (s *Server) checkDiagnostics(ctx context.Context, uri protocol.DocumentURI, content string, version uint32) {
	run := s.beginDiagnosticRun(uri)
	log.Printf("Publishing diagnostics for %s (%d bytes)", uri, len(content))
	target := s.activeTarget()
	var prepared *frontend.Result
	if doc, ok := s.documents.Get(uri); ok && doc.Content == content && doc.Analysis != nil {
		prepared = doc.Analysis.Frontend
	}
	results, err := runWorkspaceCheckWithFrontend(string(uri), content, s.documents.Snapshot(), target, prepared, s.documents.frontendSession)
	if err != nil {
		log.Printf("Compiler check failed: %v", err)
		return
	}
	if doc, open := s.documents.Get(uri); open {
		if doc.Version != int(version) || doc.Content != content {
			return
		}
	} else if version != 0 {
		return
	}
	if s.activeTarget() != target || ctx.Err() != nil {
		return
	}
	for other := range results {
		if other != uri && isGeckoURI(other) && s.workspaceContains(other) {
			delete(results, other)
		}
	}
	s.commitDiagnostics(ctx, uri, results, run, version)
}

func (s *Server) checkPathDiagnostics(ctx context.Context, path string) {
	uri := pathToURI(path)
	if doc, open := s.documents.Get(uri); open {
		s.checkDiagnostics(ctx, uri, doc.Content, uint32(doc.Version))
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.removeDiagnostics(ctx, uri)
		} else {
			log.Printf("Reading workspace source %s: %v", path, err)
		}
		return
	}
	s.checkDiagnostics(ctx, uri, string(content), 0)
}

func (s *Server) scanWorkspaceDiagnostics(ctx context.Context) {
	s.workspaceScanMu.Lock()
	defer s.workspaceScanMu.Unlock()
	s.mu.Lock()
	roots := append([]string(nil), s.workspaceRoots...)
	s.mu.Unlock()
	if len(roots) == 0 {
		return
	}
	paths := make(map[string]bool)
	for _, root := range roots {
		found, err := s.projectSourcePaths(ctx, root)
		if err != nil {
			log.Printf("Listing workspace sources in %s: %v", root, err)
			continue
		}
		for _, path := range found {
			paths[path] = true
		}
	}
	for path := range s.documents.Snapshot() {
		uri := pathToURI(path)
		if isGeckoURI(uri) && s.workspaceContains(uri) {
			paths[path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		if ctx.Err() != nil {
			return
		}
		s.checkPathDiagnostics(ctx, path)
	}
	s.diagnosticMu.Lock()
	var stale []protocol.DocumentURI
	for uri := range s.diagnostics {
		if isGeckoURI(uri) && s.workspaceContains(uri) && !paths[filepath.Clean(uriToPath(string(uri)))] {
			stale = append(stale, uri)
		}
	}
	s.diagnosticMu.Unlock()
	for _, uri := range stale {
		s.removeDiagnostics(ctx, uri)
	}
}

func (s *Server) clearDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
	if s.workspaceContains(uri) {
		path := uriToPath(string(uri))
		if _, err := os.Stat(path); err == nil {
			s.checkPathDiagnostics(ctx, path)
			return
		} else if !os.IsNotExist(err) {
			log.Printf("Checking closed workspace source %s: %v", path, err)
			return
		}
	}
	s.removeDiagnostics(ctx, uri)
}

func (s *Server) removeDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
	run := s.beginDiagnosticRun(uri)
	s.commitDiagnostics(ctx, uri, nil, run, 0)
}

func (s *Server) commitDiagnostics(ctx context.Context, owner protocol.DocumentURI, results map[protocol.DocumentURI][]protocol.Diagnostic, run uint64, version uint32) {
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	s.diagnosticMu.Lock()
	if s.diagnosticRuns[owner] != run {
		s.diagnosticMu.Unlock()
		return
	}
	affected := map[protocol.DocumentURI]bool{owner: true}
	for target := range s.diagnostics[owner] {
		affected[target] = true
	}
	for target := range results {
		affected[target] = true
	}
	if results == nil {
		delete(s.diagnostics, owner)
	} else {
		s.diagnostics[owner] = results
	}
	targets := make([]protocol.DocumentURI, 0, len(affected))
	for target := range affected {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
	merged := make(map[protocol.DocumentURI][]protocol.Diagnostic, len(targets))
	for _, target := range targets {
		merged[target] = s.mergedDiagnostics(target)
	}
	s.diagnosticMu.Unlock()
	for _, target := range targets {
		items := merged[target]
		if items == nil {
			items = []protocol.Diagnostic{}
		}
		params := protocol.PublishDiagnosticsParams{URI: target, Diagnostics: items}
		if target == owner {
			params.Version = version
		}
		if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", params); err != nil {
			log.Printf("Failed to publish diagnostics: %v", err)
		}
	}
}

func (s *Server) mergedDiagnostics(target protocol.DocumentURI) []protocol.Diagnostic {
	owners := make([]protocol.DocumentURI, 0, len(s.diagnostics))
	for owner := range s.diagnostics {
		owners = append(owners, owner)
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i] < owners[j] })
	var merged []protocol.Diagnostic
	for _, owner := range owners {
		for _, candidate := range s.diagnostics[owner][target] {
			duplicate := false
			for _, existing := range merged {
				if reflect.DeepEqual(existing, candidate) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				merged = append(merged, candidate)
			}
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		left, right := merged[i], merged[j]
		if left.Range.Start.Line != right.Range.Start.Line {
			return left.Range.Start.Line < right.Range.Start.Line
		}
		if left.Range.Start.Character != right.Range.Start.Character {
			return left.Range.Start.Character < right.Range.Start.Character
		}
		return left.Message < right.Message
	})
	return merged
}
