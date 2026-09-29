package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestRapidEditsPublishOnlyLatestVersion(t *testing.T) {
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
	server.conn.Go(ctx, server.requestHandler())
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
	uri := pathToURI(filepath.Join(t.TempDir(), "main.gecko"))
	if err := client.Notify(ctx, "textDocument/didOpen", protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri, Version: 1, Text: "package main\n"},
	}); err != nil {
		t.Fatalf("opening document: %v", err)
	}
	if first := waitPublished(t, published); first.Version != 1 {
		t.Fatalf("initial diagnostics version: %#v", first)
	}
	for version, content := range []string{
		"package main\ntrait Child: MissingParent {}\n",
		"package main\ntrait Child: OtherMissingParent {}\n",
		"package main\n",
	} {
		if err := client.Notify(ctx, "textDocument/didChange", protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}, Version: int32(version + 2),
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: content}},
		}); err != nil {
			t.Fatalf("changing document: %v", err)
		}
	}
	latest := waitPublished(t, published)
	if latest.Version != 4 || len(latest.Diagnostics) != 0 {
		t.Fatalf("published stale diagnostics: %#v", latest)
	}
	select {
	case extra := <-published:
		t.Fatalf("published another edit after the latest version: %#v", extra)
	case <-time.After(350 * time.Millisecond):
	}
}

func TestWorkspaceSymbolCancellation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.gecko")
	if err := os.WriteFile(path, []byte("package main\nclass Ready {}\n"), 0o644); err != nil {
		t.Fatalf("writing source: %v", err)
	}
	server := NewServer()
	server.workspaceRoots = []string{root}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := server.workspaceSymbols(cancelled, "Ready"); !errors.Is(err, context.Canceled) {
		t.Fatalf("workspace scan ignored cancellation: %v", err)
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
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.requestHandler())
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	if err := client.Notify(ctx, "$/cancelRequest", map[string]any{"id": 123}); err != nil {
		t.Fatalf("sending cancellation: %v", err)
	}
	var symbols []protocol.SymbolInformation
	if _, err := client.Call(ctx, "workspace/symbol", protocol.WorkspaceSymbolParams{Query: "Ready"}, &symbols); err != nil || len(symbols) != 1 {
		t.Fatalf("workspace request after cancellation: %#v, %v", symbols, err)
	}
}

func TestCancelNotificationStopsPendingWorkspaceRequest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.gecko")
	if err := os.WriteFile(path, []byte("package main\nclass Ready {}\n"), 0o644); err != nil {
		t.Fatalf("writing source: %v", err)
	}
	server := NewServer()
	server.workspaceRoots = []string{root}
	handler := server.requestHandler()
	call, err := jsonrpc2.NewCall(jsonrpc2.NewStringID("pending"), "workspace/symbol", protocol.WorkspaceSymbolParams{Query: "Ready"})
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	replies := make(chan error, 1)
	server.mu.Lock()
	locked := true
	defer func() {
		if locked {
			server.mu.Unlock()
		}
	}()
	if err := handler(context.Background(), func(_ context.Context, _ any, err error) error {
		replies <- err
		return nil
	}, call); err != nil {
		t.Fatalf("dispatching workspace request: %v", err)
	}
	cancelRequest, err := jsonrpc2.NewNotification("$/cancelRequest", protocol.CancelParams{ID: "pending"})
	if err != nil {
		t.Fatalf("creating cancellation: %v", err)
	}
	if err := handler(context.Background(), func(context.Context, any, error) error { return nil }, cancelRequest); err != nil {
		t.Fatalf("dispatching cancellation: %v", err)
	}
	server.mu.Unlock()
	locked = false
	select {
	case err := <-replies:
		var rpcErr *jsonrpc2.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != protocol.CodeRequestCancelled {
			t.Fatalf("pending request was not cancelled: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pending request did not respond to cancellation")
	}
}

func TestFrontendAnalysisDoesNotWaitForBackendCheck(t *testing.T) {
	server := NewServer()
	uri := pathToURI(filepath.Join(t.TempDir(), "main.gecko"))
	server.documents.Open(uri, "package main\nclass Ready {}\n", 1)
	compilerCheckMu.Lock()
	defer compilerCheckMu.Unlock()
	done := make(chan struct{}, 1)
	go func() {
		server.rebuildAnalysis(uri)
		done <- struct{}{}
	}()
	select {
	case <-done:
		if doc, ok := server.documents.Get(uri); !ok || doc.Analysis == nil {
			t.Fatal("frontend analysis did not finish while backend lock was held")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("frontend analysis waited for backend lock")
	}
}

func TestUTF16PositionsAroundUnicodeText(t *testing.T) {
	content := "package main\nfunc main(): void {\n    let text: string = \"😀\"\n    let value: int32 = 1\n    value = value + 1\n}\n"
	start := strings.Index(content, "value")
	position := sourcePosition(content, start)
	if position.Line != 3 || position.Character != 8 || sourceOffset(content, position) != start {
		t.Fatalf("incorrect UTF-16 source position: %#v", position)
	}
	emoji := strings.Index(content, "😀")
	if got := sourcePosition(content, emoji+len("😀")); got.Line != 2 || got.Character != 26 {
		t.Fatalf("emoji did not occupy two UTF-16 units: %#v", got)
	}
	uri := pathToURI(filepath.Join(t.TempDir(), "main.gecko"))
	server := NewServer()
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("failed to analyze source containing Unicode text")
	}
	location := semanticDefinition(doc.Analysis, uriToPath(string(uri)), content, protocol.Position{Line: 4, Character: 5})
	if location == nil || location.Range.Start != position {
		t.Fatalf("definition shifted after Unicode text: %#v", location)
	}
}

func TestProtocolDiagnosticsForOwnershipAndUnsafeSyntax(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("finding repository root: %v", err)
	}
	t.Setenv("GECKO_HOME", root)
	serverSide, clientSide := net.Pipe()
	t.Cleanup(func() {
		if err := clientSide.Close(); err != nil {
			t.Errorf("closing client pipe: %v", err)
		}
		if err := serverSide.Close(); err != nil {
			t.Errorf("closing server pipe: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	server := NewServer()
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	published := make(chan protocol.PublishDiagnosticsParams, 32)
	server.conn.Go(ctx, server.requestHandler())
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
	for _, fixture := range []struct {
		name    string
		message string
	}{
		{name: "unsafe_alloc_context_error", message: "unsafe"},
		{name: "borrow_move_error", message: "move"},
		{name: "unsafe_setup"},
	} {
		path := filepath.Join(root, "test_sources", "compile_tests", fixture.name, "main.gecko")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", fixture.name, err)
		}
		uri := pathToURI(path)
		if err := client.Notify(ctx, "textDocument/didOpen", protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: uri, Version: 1, Text: string(content)},
		}); err != nil {
			t.Fatalf("opening %s: %v", fixture.name, err)
		}
		for {
			params := waitPublished(t, published)
			if params.URI != uri {
				continue
			}
			if fixture.message == "" {
				if len(params.Diagnostics) != 0 {
					t.Fatalf("valid unsafe setup has diagnostics: %#v", params.Diagnostics)
				}
			} else if !hasDiagnosticContaining(params.Diagnostics, fixture.message) {
				t.Fatalf("missing %s diagnostic: %#v", fixture.message, params.Diagnostics)
			}
			break
		}
	}
}
