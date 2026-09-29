package main

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestServerPublishesVersionedDiagnosticsAndClearsOnClose(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	t.Cleanup(func() {
		if err := clientSide.Close(); err != nil {
			t.Errorf("closing client pipe: %v", err)
		}
		if err := serverSide.Close(); err != nil {
			t.Errorf("closing server pipe: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server := NewServer()
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	published := make(chan protocol.PublishDiagnosticsParams, 8)
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if req.Method() == "textDocument/publishDiagnostics" {
			var params protocol.PublishDiagnosticsParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return err
			}
			published <- params
		}
		return reply(ctx, nil, nil)
	})
	var initialized protocol.InitializeResult
	if _, err := client.Call(ctx, "initialize", map[string]any{"capabilities": map[string]any{}}, &initialized); err != nil {
		t.Fatalf("initializing LSP: %v", err)
	}
	uri := pathToURI(t.TempDir() + "/main.gecko")
	open := protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{
		URI: uri, LanguageID: "gecko", Version: 7,
		Text: "package test\ntrait Child: MissingParent {\n    func value(self): int32\n}\n",
	}}
	if err := client.Notify(ctx, "textDocument/didOpen", open); err != nil {
		t.Fatalf("opening document: %v", err)
	}
	first := waitPublished(t, published)
	if first.URI != uri || first.Version != 7 || !hasDiagnosticContaining(first.Diagnostics, "MissingParent") {
		t.Fatalf("incorrect open diagnostics: %#v", first)
	}
	change := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri},
			Version:                8,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: "package test\n"}},
	}
	if err := client.Notify(ctx, "textDocument/didChange", change); err != nil {
		t.Fatalf("changing document: %v", err)
	}
	second := waitPublished(t, published)
	if second.URI != uri || second.Version != 8 || len(second.Diagnostics) != 0 {
		t.Fatalf("incorrect changed diagnostics: %#v", second)
	}
	if err := client.Notify(ctx, "textDocument/didClose", protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	}); err != nil {
		t.Fatalf("closing document: %v", err)
	}
	closed := waitPublished(t, published)
	if closed.URI != uri || len(closed.Diagnostics) != 0 {
		t.Fatalf("diagnostics not cleared on close: %#v", closed)
	}
}

func waitPublished(t *testing.T, published <-chan protocol.PublishDiagnosticsParams) protocol.PublishDiagnosticsParams {
	t.Helper()
	select {
	case params := <-published:
		return params
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for diagnostics")
		return protocol.PublishDiagnosticsParams{}
	}
}

func TestServerRejectsUnknownAndPostShutdownRequests(t *testing.T) {
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
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	var result any
	if _, err := client.Call(ctx, "unknownMethod", nil, &result); err == nil {
		t.Fatal("unknown method returned success")
	}
	if _, err := client.Call(ctx, "shutdown", nil, &result); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	if _, err := client.Call(ctx, "textDocument/hover", nil, &result); err == nil {
		t.Fatal("request after shutdown returned success")
	}
}
