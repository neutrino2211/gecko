// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type Server struct {
	conn            jsonrpc2.Conn
	documents       *DocumentStore
	mu              sync.Mutex
	diagnosticMu    sync.Mutex
	publishMu       sync.Mutex
	diagnostics     map[protocol.DocumentURI]map[protocol.DocumentURI][]protocol.Diagnostic
	diagnosticRuns  map[protocol.DocumentURI]uint64
	workspaceScanMu sync.Mutex
	timers          map[protocol.DocumentURI]*time.Timer
	targetOverride  string
	workspaceRoots  []string
	sourcePaths     map[string]*sourceCacheEntry
	sourceRevision  uint64
	watchConfig     bool
	shutdown        bool
	exiting         bool
}

func NewServer() *Server {
	return &Server{
		documents:      NewDocumentStore(),
		diagnostics:    make(map[protocol.DocumentURI]map[protocol.DocumentURI][]protocol.Diagnostic),
		diagnosticRuns: make(map[protocol.DocumentURI]uint64),
		timers:         make(map[protocol.DocumentURI]*time.Timer),
		sourcePaths:    make(map[string]*sourceCacheEntry),
	}
}

func (s *Server) Handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	log.Printf("Received: %s", req.Method())
	s.mu.Lock()
	shutdown := s.shutdown
	s.mu.Unlock()
	if shutdown && req.Method() != "exit" {
		return reply(ctx, nil, jsonrpc2.ErrInvalidRequest)
	}

	switch req.Method() {
	case "initialize":
		return s.handleInitialize(ctx, reply, req)
	case "initialized":
		s.registerConfigWatcher(ctx)
		go s.scanWorkspaceDiagnostics(ctx)
		return reply(ctx, nil, nil)
	case "shutdown":
		s.mu.Lock()
		s.shutdown = true
		s.mu.Unlock()
		return reply(ctx, nil, nil)
	case "exit":
		s.mu.Lock()
		s.exiting = true
		for _, timer := range s.timers {
			timer.Stop()
		}
		s.mu.Unlock()
		return s.conn.Close()
	case "textDocument/didOpen":
		return s.handleDidOpen(ctx, reply, req)
	case "textDocument/didChange":
		return s.handleDidChange(ctx, reply, req)
	case "textDocument/didClose":
		return s.handleDidClose(ctx, reply, req)
	case "textDocument/didSave":
		return s.handleDidSave(ctx, reply, req)
	case "textDocument/hover":
		return s.handleHover(ctx, reply, req)
	case "textDocument/definition":
		return s.handleDefinition(ctx, reply, req)
	case "textDocument/documentSymbol":
		return s.handleDocumentSymbol(ctx, reply, req)
	case "textDocument/semanticTokens/full":
		return s.handleSemanticTokens(ctx, reply, req)
	case "textDocument/inlayHint":
		return s.handleInlayHints(ctx, reply, req)
	case "textDocument/references":
		return s.handleReferences(ctx, reply, req)
	case "textDocument/prepareRename":
		return s.handlePrepareRename(ctx, reply, req)
	case "textDocument/rename":
		return s.handleRename(ctx, reply, req)
	case "textDocument/completion":
		return s.handleCompletion(ctx, reply, req)
	case "textDocument/signatureHelp":
		return s.handleSignatureHelp(ctx, reply, req)
	case "textDocument/codeAction":
		return s.handleCodeAction(ctx, reply, req)
	case "workspace/didChangeWatchedFiles":
		return s.handleWatchedFiles(ctx, reply, req)
	case "workspace/didChangeConfiguration":
		return s.handleConfigurationChange(ctx, reply, req)
	case "workspace/symbol":
		return s.handleWorkspaceSymbol(ctx, reply, req)
	default:
		log.Printf("Unhandled method: %s", req.Method())
		return jsonrpc2.MethodNotFoundHandler(ctx, reply, req)
	}
}

func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	if err := s.initializeProjectOptions(req.Params()); err != nil {
		return reply(ctx, nil, err)
	}
	result := struct {
		Capabilities struct {
			protocol.ServerCapabilities
			InlayHintProvider bool `json:"inlayHintProvider"`
		} `json:"capabilities"`
		ServerInfo *protocol.ServerInfo `json:"serverInfo,omitempty"`
	}{}
	result.Capabilities.InlayHintProvider = true
	result.Capabilities.ServerCapabilities = protocol.ServerCapabilities{
		TextDocumentSync: &protocol.TextDocumentSyncOptions{
			OpenClose: true,
			Change:    protocol.TextDocumentSyncKindFull,
			Save: &protocol.SaveOptions{
				IncludeText: true,
			},
		},
		HoverProvider:           true,
		DefinitionProvider:      true,
		DocumentSymbolProvider:  true,
		WorkspaceSymbolProvider: true,
		SemanticTokensProvider:  map[string]any{"legend": semanticTokenLegend(), "full": true},
		ReferencesProvider:      true,
		RenameProvider:          &protocol.RenameOptions{PrepareProvider: true},
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{".", ":"},
		},
		SignatureHelpProvider: &protocol.SignatureHelpOptions{
			TriggerCharacters:   []string{"(", ","},
			RetriggerCharacters: []string{","},
		},
		CodeActionProvider: true,
	}
	result.ServerInfo = &protocol.ServerInfo{
		Name:    "gecko-lsp",
		Version: "0.1.0",
	}

	return reply(ctx, result, nil)
}

func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	content := params.TextDocument.Text

	s.documents.Open(uri, content, params.TextDocument.Version)
	s.documentOpened(ctx, uri)

	return reply(ctx, nil, nil)
}

func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("didChange unmarshal error: %v", err)
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	log.Printf("didChange for %s, %d changes", uri, len(params.ContentChanges))

	if len(params.ContentChanges) > 0 {
		content := params.ContentChanges[len(params.ContentChanges)-1].Text
		log.Printf("Content length: %d bytes", len(content))
		if s.documents.Update(uri, content, params.TextDocument.Version) {
			s.documentChanged(ctx, uri)
		}
	} else {
		log.Printf("No content changes received")
	}

	return reply(ctx, nil, nil)
}

func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	dependents := s.documents.Dependents(uri)
	s.documents.Close(uri)
	s.cancelDiagnostics(uri)
	s.clearDiagnostics(ctx, uri)
	s.documentClosed(ctx, uri, dependents)
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidSave(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(req.Params(), &raw); err != nil {
		return reply(ctx, nil, err)
	}
	if _, ok := raw["text"]; ok {
		s.documents.Save(uri, params.Text)
	}
	s.documentSaved(ctx, uri)

	return reply(ctx, nil, nil)
}

func (s *Server) handleHover(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.HoverParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("hover unmarshal error: %v", err)
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	line := int(params.Position.Line)
	col := int(params.Position.Character)

	log.Printf("Hover request at %s:%d:%d", uri, line, col)

	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found for hover: %s", uri)
		return reply(ctx, nil, nil)
	}
	col = byteColumn(doc.Content, params.Position)

	info := GetHoverInfo(doc.Analysis, doc.Content, line, col)
	if info == nil {
		log.Printf("No symbol found at position")
		return reply(ctx, nil, nil)
	}

	log.Printf("Found symbol: %s (%s)", info.Name, info.Type)

	// Format hover content
	var content string
	if info.DocComment != "" {
		content = fmt.Sprintf("```gecko\n%s\n```\n\n%s", info.Type, info.DocComment)
	} else {
		content = fmt.Sprintf("```gecko\n%s\n```", info.Type)
	}

	hover := protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: content,
		},
	}

	return reply(ctx, hover, nil)
}

func (s *Server) handleDefinition(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DefinitionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("definition unmarshal error: %v", err)
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	line := int(params.Position.Line)
	col := int(params.Position.Character)

	log.Printf("Definition request at %s:%d:%d", uri, line, col)

	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found for definition: %s", uri)
		return reply(ctx, nil, nil)
	}
	if location := semanticDefinition(doc.Analysis, uriToPath(string(uri)), doc.Content, params.Position); location != nil {
		return reply(ctx, location, nil)
	}
	col = byteColumn(doc.Content, params.Position)

	location := GetDefinitionLocation(doc.Analysis, doc.Content, line, col, string(uri))
	if location == nil {
		log.Printf("No definition found")
		return reply(ctx, nil, nil)
	}
	if targetContent, ok := s.documents.Snapshot()[uriToPath(string(location.URI))]; ok {
		location.Range.Start = utf16Position(targetContent, location.Range.Start)
		location.Range.End = utf16Position(targetContent, location.Range.End)
	} else if location.URI == uri {
		location.Range.Start = utf16Position(doc.Content, location.Range.Start)
		location.Range.End = utf16Position(doc.Content, location.Range.End)
	}

	log.Printf("Found definition at %s:%d:%d", location.URI, location.Range.Start.Line, location.Range.Start.Character)
	return reply(ctx, location, nil)
}

func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("completion unmarshal error: %v", err)
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	line := int(params.Position.Line)
	col := int(params.Position.Character)

	log.Printf("Completion request at %s:%d:%d", uri, line, col)

	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found for completion: %s", uri)
		return reply(ctx, nil, nil)
	}
	col = byteColumn(doc.Content, params.Position)

	items := GetCompletions(doc.Analysis, doc.Content, uriToPath(string(uri)), line, col)
	log.Printf("Found %d completion items", len(items))

	return reply(ctx, items, nil)
}

func (s *Server) rebuildAnalysis(uri protocol.DocumentURI) {
	doc, ok := s.documents.Get(uri)
	if !ok {
		return
	}
	doc.RebuildAnalysisWithSession(s.documents.Snapshot(), s.documents.frontendSession)
	s.documents.SetAnalysis(doc)
}

func (s *Server) queueDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timer := s.timers[uri]; timer != nil {
		timer.Stop()
	}
	var timer *time.Timer
	timer = time.AfterFunc(150*time.Millisecond, func() {
		s.mu.Lock()
		if s.timers[uri] != timer {
			s.mu.Unlock()
			return
		}
		delete(s.timers, uri)
		s.mu.Unlock()
		s.publishDiagnostics(ctx, uri)
	})
	s.timers[uri] = timer
}

func (s *Server) cancelDiagnostics(uri protocol.DocumentURI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timer := s.timers[uri]; timer != nil {
		timer.Stop()
		delete(s.timers, uri)
	}
}

func (s *Server) handleSignatureHelp(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.SignatureHelpParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("signatureHelp unmarshal error: %v", err)
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	line := int(params.Position.Line)
	col := int(params.Position.Character)

	log.Printf("SignatureHelp request at %s:%d:%d", uri, line, col)

	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found for signatureHelp: %s", uri)
		return reply(ctx, nil, nil)
	}
	col = byteColumn(doc.Content, params.Position)

	result := GetSignatureHelp(doc.Analysis, doc.Content, uriToPath(string(uri)), line, col)
	if result == nil {
		log.Printf("No signature help found")
		return reply(ctx, nil, nil)
	}

	log.Printf("Found signature help with %d signatures", len(result.Signatures))
	return reply(ctx, result, nil)
}
