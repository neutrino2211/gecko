package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestWorkspaceDiagnosticsSurviveFileSwitches(t *testing.T) {
	root := t.TempDir()
	sources := map[string]string{
		"webview.gecko":       "package webview\nimport fs\ndeclare external func open_view(): int32\n",
		"fs.gecko":            "package fs\ndeclare external func open_file(): int32\ndeclare external func close_file(): int32\n",
		"sqlite_native.gecko": "package sqlite_native\ndeclare external func open_db(): int32\ndeclare external func close_db(): int32\n",
	}
	for name, content := range sources {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	serverSide, clientSide := net.Pipe()
	t.Cleanup(func() {
		if err := clientSide.Close(); err != nil {
			t.Errorf("closing client pipe: %v", err)
		}
		if err := serverSide.Close(); err != nil {
			t.Errorf("closing server pipe: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	server := NewServer()
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	published := make(chan protocol.PublishDiagnosticsParams, 64)
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
	if _, err := client.Call(ctx, "initialize", map[string]any{
		"rootUri": string(pathToURI(root)), "capabilities": map[string]any{},
	}, &initialized); err != nil {
		t.Fatalf("initializing LSP: %v", err)
	}
	if err := client.Notify(ctx, "initialized", map[string]any{}); err != nil {
		t.Fatalf("notifying initialized: %v", err)
	}
	wanted := map[protocol.DocumentURI]int{
		pathToURI(filepath.Join(root, "webview.gecko")):       1,
		pathToURI(filepath.Join(root, "fs.gecko")):            2,
		pathToURI(filepath.Join(root, "sqlite_native.gecko")): 2,
	}
	seen := make(map[protocol.DocumentURI]bool)
	for len(seen) < len(wanted) {
		params := waitPublished(t, published)
		count, ok := wanted[params.URI]
		if !ok {
			continue
		}
		if countDeprecations(params.Diagnostics) == count {
			seen[params.URI] = true
		}
	}
	fsURI := pathToURI(filepath.Join(root, "fs.gecko"))
	if err := client.Notify(ctx, "textDocument/didOpen", protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: fsURI, LanguageID: "gecko", Version: 1, Text: sources["fs.gecko"]},
	}); err != nil {
		t.Fatalf("opening file: %v", err)
	}
	if params := waitPublished(t, published); params.URI != fsURI || countDeprecations(params.Diagnostics) != 2 {
		t.Fatalf("opening file lost warnings: %#v", params)
	}
	if err := client.Notify(ctx, "textDocument/didChange", protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: fsURI}, Version: 2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: "package fs\n"}},
	}); err != nil {
		t.Fatalf("changing file: %v", err)
	}
	if params := waitPublished(t, published); params.URI != fsURI || countDeprecations(params.Diagnostics) != 0 {
		t.Fatalf("editing file kept stale warnings: %#v", params)
	}
	if err := client.Notify(ctx, "textDocument/didClose", protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: fsURI},
	}); err != nil {
		t.Fatalf("closing file: %v", err)
	}
	if params := waitPublished(t, published); params.URI != fsURI || countDeprecations(params.Diagnostics) != 2 {
		t.Fatalf("closing file cleared warnings: %#v", params)
	}
	server.diagnosticMu.Lock()
	sqlite := server.mergedDiagnostics(pathToURI(filepath.Join(root, "sqlite_native.gecko")))
	server.diagnosticMu.Unlock()
	if countDeprecations(sqlite) != 2 {
		t.Fatalf("unrelated file warnings were lost: %#v", sqlite)
	}
	newPath := filepath.Join(root, "new_bridge.gecko")
	if err := os.WriteFile(newPath, []byte("package new_bridge\ndeclare external func native(): int32\n"), 0o644); err != nil {
		t.Fatalf("writing new source: %v", err)
	}
	if err := client.Notify(ctx, "workspace/didChangeWatchedFiles", protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: pathToURI(newPath), Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("notifying new source: %v", err)
	}
	for {
		params := waitPublished(t, published)
		if params.URI == pathToURI(newPath) {
			if countDeprecations(params.Diagnostics) != 1 {
				t.Fatalf("new source warnings missing: %#v", params)
			}
			break
		}
	}
}

func countDeprecations(diagnostics []protocol.Diagnostic) int {
	count := 0
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
			count++
		}
	}
	return count
}
