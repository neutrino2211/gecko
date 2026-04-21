package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type Server struct {
	conn      jsonrpc2.Conn
	documents *DocumentStore
}

func NewServer() *Server {
	return &Server{
		documents: NewDocumentStore(),
	}
}

func (s *Server) Handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	log.Printf("Received: %s", req.Method())

	switch req.Method() {
	case "initialize":
		return s.handleInitialize(ctx, reply, req)
	case "initialized":
		return reply(ctx, nil, nil)
	case "shutdown":
		return reply(ctx, nil, nil)
	case "exit":
		return reply(ctx, nil, nil)
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
	case "textDocument/completion":
		return s.handleCompletion(ctx, reply, req)
	case "textDocument/signatureHelp":
		return s.handleSignatureHelp(ctx, reply, req)
	case "textDocument/codeAction":
		return s.handleCodeAction(ctx, reply, req)
	default:
		log.Printf("Unhandled method: %s", req.Method())
		return reply(ctx, nil, nil)
	}
}

func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	result := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
				Save: &protocol.SaveOptions{
					IncludeText: true,
				},
			},
			HoverProvider:      true,
				DefinitionProvider: true,
				CompletionProvider: &protocol.CompletionOptions{
					TriggerCharacters: []string{".", ":"},
				},
				SignatureHelpProvider: &protocol.SignatureHelpOptions{
					TriggerCharacters:   []string{"(", ","},
					RetriggerCharacters: []string{","},
				},
				CodeActionProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "gecko-lsp",
			Version: "0.1.0",
		},
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

	s.documents.Open(uri, content)
	s.publishDiagnostics(ctx, uri)

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
		content := params.ContentChanges[0].Text
		log.Printf("Content length: %d bytes", len(content))
		s.documents.Update(uri, content)
		s.publishDiagnostics(ctx, uri)
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

	s.documents.Close(params.TextDocument.URI)
	return reply(ctx, nil, nil)
}

func (s *Server) handleDidSave(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	if params.Text != "" {
		s.documents.Update(uri, params.Text)
	}
	s.publishDiagnostics(ctx, uri)

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

	info := GetHoverInfo(doc.Content, line, col)
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

	location := GetDefinitionLocation(doc.Content, line, col, string(uri))
	if location == nil {
		log.Printf("No definition found")
		return reply(ctx, nil, nil)
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

	items := GetCompletions(doc.Content, uriToPath(string(uri)), line, col)
	log.Printf("Found %d completion items", len(items))

	return reply(ctx, items, nil)
}

func (s *Server) publishDiagnostics(ctx context.Context, uri protocol.DocumentURI) {
	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found: %s", uri)
		return
	}

	log.Printf("Publishing diagnostics for %s (%d bytes)", uri, len(doc.Content))
	diagnostics, err := RunCompilerCheck(string(uri), doc.Content)
	if err != nil {
		log.Printf("Compiler check failed: %v", err)
	}

	// Ensure we always send a valid array (empty array clears diagnostics)
	if diagnostics == nil {
		diagnostics = []protocol.Diagnostic{}
	}
	log.Printf("Found %d diagnostics", len(diagnostics))

	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Version:     uint32(doc.Version),
		Diagnostics: diagnostics,
	}

	if err := s.conn.Notify(ctx, "textDocument/publishDiagnostics", params); err != nil {
		log.Printf("Failed to publish diagnostics: %v", err)
	} else {
		log.Printf("Successfully published diagnostics")
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

	result := GetSignatureHelp(doc.Content, uriToPath(string(uri)), line, col)
	if result == nil {
		log.Printf("No signature help found")
		return reply(ctx, nil, nil)
	}

	log.Printf("Found signature help with %d signatures", len(result.Signatures))
	return reply(ctx, result, nil)
}

func (s *Server) handleCodeAction(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("codeAction unmarshal error: %v", err)
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	log.Printf("CodeAction request for %s, range %v", uri, params.Range)

	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found for codeAction: %s", uri)
		return reply(ctx, []protocol.CodeAction{}, nil)
	}

	actions := GetCodeActions(doc.Content, uriToPath(string(uri)), params.Range, params.Context.Diagnostics)
	log.Printf("Found %d code actions", len(actions))

	return reply(ctx, actions, nil)
}
