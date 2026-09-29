package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/neutrino2211/gecko/config"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func isProjectConfigURI(uri protocol.DocumentURI) bool {
	return filepath.Base(uriToPath(string(uri))) == "gecko.toml"
}

func isGeckoURI(uri protocol.DocumentURI) bool {
	return filepath.Ext(uriToPath(string(uri))) == ".gecko"
}

func (s *Server) activeTarget() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.targetOverride
}

func (s *Server) setTarget(target string) error {
	if _, err := config.ResolveCompileTarget(nil, target, runtime.GOARCH, runtime.GOOS); err != nil {
		return fmt.Errorf("setting Gecko target: %w", err)
	}
	s.mu.Lock()
	s.targetOverride = target
	s.mu.Unlock()
	return nil
}

func (s *Server) initializeProjectOptions(raw json.RawMessage) error {
	var params struct {
		RootURI          string `json:"rootUri"`
		RootPath         string `json:"rootPath"`
		WorkspaceFolders []struct {
			URI string `json:"uri"`
		} `json:"workspaceFolders"`
		InitializationOptions struct {
			Gecko struct {
				Target string `json:"target"`
			} `json:"gecko"`
		} `json:"initializationOptions"`
		Capabilities struct {
			Workspace struct {
				DidChangeWatchedFiles struct {
					DynamicRegistration bool `json:"dynamicRegistration"`
				} `json:"didChangeWatchedFiles"`
			} `json:"workspace"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return fmt.Errorf("reading initialization options: %w", err)
	}
	if err := s.setTarget(params.InitializationOptions.Gecko.Target); err != nil {
		return err
	}
	s.mu.Lock()
	s.watchConfig = params.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration
	s.workspaceRoots = nil
	s.sourcePaths = make(map[string]*sourceCacheEntry)
	s.sourceRevision++
	for _, folder := range params.WorkspaceFolders {
		if folder.URI != "" {
			s.workspaceRoots = append(s.workspaceRoots, filepath.Clean(uriToPath(folder.URI)))
		}
	}
	if len(s.workspaceRoots) == 0 {
		if params.RootURI != "" {
			s.workspaceRoots = []string{filepath.Clean(uriToPath(params.RootURI))}
		} else if params.RootPath != "" {
			s.workspaceRoots = []string{filepath.Clean(params.RootPath)}
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *Server) registerConfigWatcher(ctx context.Context) {
	s.mu.Lock()
	enabled := s.watchConfig
	s.mu.Unlock()
	if !enabled {
		return
	}
	go func() {
		watchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		params := protocol.RegistrationParams{Registrations: []protocol.Registration{{
			ID: "gecko-project-config", Method: "workspace/didChangeWatchedFiles",
			RegisterOptions: protocol.DidChangeWatchedFilesRegistrationOptions{
				Watchers: []protocol.FileSystemWatcher{{GlobPattern: "**/gecko.toml"}, {GlobPattern: "**/*.gecko"}},
			},
		}}}
		var result any
		if _, err := s.conn.Call(watchCtx, "client/registerCapability", params, &result); err != nil {
			log.Printf("Could not register gecko.toml watcher: %v", err)
		}
	}()
}

func (s *Server) projectDocuments(configPath string) []protocol.DocumentURI {
	directory := filepath.Dir(filepath.Clean(configPath))
	var result []protocol.DocumentURI
	for path := range s.documents.Snapshot() {
		if filepath.Ext(path) != ".gecko" {
			continue
		}
		if strings.HasPrefix(filepath.Clean(path), directory+string(filepath.Separator)) {
			result = append(result, pathToURI(path))
		}
	}
	return result
}

func (s *Server) refreshProjectConfig(ctx context.Context, configURI protocol.DocumentURI) {
	s.documents.InvalidateDependencies()
	s.invalidateSourcePaths()
	for _, uri := range s.projectDocuments(uriToPath(string(configURI))) {
		s.rebuildAnalysis(uri)
		s.queueDiagnostics(ctx, uri)
	}
	go s.scanWorkspaceDiagnostics(ctx)
}

func (s *Server) refreshDependents(ctx context.Context, uri protocol.DocumentURI) {
	for _, dependent := range s.documents.Dependents(uri) {
		s.rebuildAnalysis(dependent)
		s.queueDiagnostics(ctx, dependent)
	}
}

func (s *Server) documentOpened(ctx context.Context, uri protocol.DocumentURI) {
	if isProjectConfigURI(uri) {
		s.publishDiagnostics(ctx, uri)
		s.refreshProjectConfig(ctx, uri)
		return
	}
	if !isGeckoURI(uri) {
		return
	}
	s.rebuildAnalysis(uri)
	s.publishDiagnostics(ctx, uri)
	s.refreshDependents(ctx, uri)
}

func (s *Server) documentChanged(ctx context.Context, uri protocol.DocumentURI) {
	if isProjectConfigURI(uri) {
		s.queueDiagnostics(ctx, uri)
		s.refreshProjectConfig(ctx, uri)
		return
	}
	if !isGeckoURI(uri) {
		return
	}
	s.rebuildAnalysis(uri)
	s.queueDiagnostics(ctx, uri)
	s.refreshDependents(ctx, uri)
}

func (s *Server) documentSaved(ctx context.Context, uri protocol.DocumentURI) {
	if !isProjectConfigURI(uri) && !isGeckoURI(uri) {
		return
	}
	if isGeckoURI(uri) {
		s.invalidateSourcePaths()
		s.rebuildAnalysis(uri)
	}
	s.cancelDiagnostics(uri)
	s.publishDiagnostics(ctx, uri)
	if isProjectConfigURI(uri) {
		s.refreshProjectConfig(ctx, uri)
	} else {
		s.refreshDependents(ctx, uri)
		go s.scanWorkspaceDiagnostics(ctx)
	}
}

func (s *Server) documentClosed(ctx context.Context, uri protocol.DocumentURI, dependents []protocol.DocumentURI) {
	if isProjectConfigURI(uri) {
		s.refreshProjectConfig(ctx, uri)
		return
	}
	for _, dependent := range dependents {
		s.rebuildAnalysis(dependent)
		s.queueDiagnostics(ctx, dependent)
	}
}

func (s *Server) handleWatchedFiles(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeWatchedFilesParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	seen := make(map[protocol.DocumentURI]bool)
	refreshWorkspace := false
	for _, event := range params.Changes {
		if event == nil {
			continue
		}
		uri := protocol.DocumentURI(event.URI)
		if seen[uri] {
			continue
		}
		seen[uri] = true
		if isProjectConfigURI(uri) {
			if _, open := s.documents.Get(uri); open {
				s.queueDiagnostics(ctx, uri)
			}
			s.refreshProjectConfig(ctx, uri)
		} else if isGeckoURI(uri) {
			refreshWorkspace = true
			if event.Type == protocol.FileChangeTypeCreated {
				s.documents.InvalidateDependencies()
			}
			if event.Type == protocol.FileChangeTypeCreated || event.Type == protocol.FileChangeTypeDeleted {
				s.invalidateSourcePaths()
			}
			s.refreshDependents(ctx, uri)
		}
	}
	if refreshWorkspace {
		go s.scanWorkspaceDiagnostics(ctx)
	}
	return reply(ctx, nil, nil)
}

func (s *Server) handleConfigurationChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	raw, ok := params.Settings["gecko"]
	if !ok {
		return reply(ctx, nil, nil)
	}
	var settings struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return reply(ctx, nil, err)
	}
	oldTarget := s.activeTarget()
	if err := s.setTarget(settings.Target); err != nil {
		return reply(ctx, nil, err)
	}
	if settings.Target != oldTarget {
		for path := range s.documents.Snapshot() {
			uri := pathToURI(path)
			if isGeckoURI(uri) {
				s.rebuildAnalysis(uri)
				s.queueDiagnostics(ctx, uri)
			}
		}
		go s.scanWorkspaceDiagnostics(ctx)
	}
	return reply(ctx, nil, nil)
}
