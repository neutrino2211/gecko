package main

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/neutrino2211/gecko/analysis"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestInlayHintsUseInferredTypesAndRange(t *testing.T) {
	content := "package test\nfunc main(): void {\n    let count = 42\n    let explicit: int32 = 1\n    if true { let ready = true }\n}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing source: %v", err)
	}
	doc := &Document{URI: pathToURI(path), Content: content, Analysis: ctx}
	hints := inlayHints(doc, protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 5, Character: 0}})
	if len(hints) != 2 || hints[0].Label != ": int32" || hints[1].Label != ": bool" {
		t.Fatalf("inferred type hints: %#v", hints)
	}
	narrow := inlayHints(doc, protocol.Range{Start: protocol.Position{Line: 4}, End: protocol.Position{Line: 4, Character: 40}})
	if len(narrow) != 1 || narrow[0].Label != ": bool" {
		t.Fatalf("range-filtered hints: %#v", narrow)
	}
}

func TestInlayHintsProtocol(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	t.Cleanup(func() {
		if err := clientSide.Close(); err != nil {
			t.Errorf("closing client pipe: %v", err)
		}
		if err := serverSide.Close(); err != nil {
			t.Errorf("closing server pipe: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := NewServer()
	uri := pathToURI(filepath.Join(t.TempDir(), "main.gecko"))
	server.documents.Open(uri, "package test\nfunc main(): void { let value = 7 }\n", 1)
	server.rebuildAnalysis(uri)
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	var initialized map[string]any
	if _, err := client.Call(ctx, "initialize", map[string]any{"capabilities": map[string]any{}}, &initialized); err != nil {
		t.Fatalf("initializing LSP: %v", err)
	}
	capabilities, ok := initialized["capabilities"].(map[string]any)
	if !ok || capabilities["inlayHintProvider"] != true {
		t.Fatalf("missing inlay hint capability: %#v", initialized)
	}
	var hints []inlayHint
	_, err := client.Call(ctx, "textDocument/inlayHint", inlayHintParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Range:        protocol.Range{Start: protocol.Position{}, End: protocol.Position{Line: 2}},
	}, &hints)
	if err != nil || len(hints) != 1 || hints[0].Label != ": int32" {
		t.Fatalf("inlay hint response: %#v, %v", hints, err)
	}
}
