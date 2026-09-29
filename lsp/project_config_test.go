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

func TestWorkspaceCheckUsesConfiguredTarget(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "gecko.toml")
	if err := os.WriteFile(configPath, []byte("[build]\nbackend = \"c\"\ndefault_target = \"aarch64-apple-darwin\"\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	uri := pathToURI(filepath.Join(root, "main.gecko"))
	content := "package main\nimpl arm64 { func selected(): void {} }\n"
	matching, err := RunWorkspaceCheckWithTarget(string(uri), content, nil, "")
	if err != nil {
		t.Fatalf("checking configured target: %v", err)
	}
	if hasDiagnosticContaining(matching[uri], "was skipped") {
		t.Fatalf("configured architecture was skipped: %#v", matching[uri])
	}
	overridden, err := RunWorkspaceCheckWithTarget(string(uri), content, nil, "x86_64-unknown-linux-gnu")
	if err != nil {
		t.Fatalf("checking override target: %v", err)
	}
	if !hasDiagnosticContaining(overridden[uri], "was skipped") {
		t.Fatalf("override did not change architecture: %#v", overridden[uri])
	}
}

func TestInitialTargetOption(t *testing.T) {
	server := NewServer()
	if err := server.initializeProjectOptions(json.RawMessage(`{"initializationOptions":{"gecko":{"target":"aarch64-apple-darwin"}}}`)); err != nil {
		t.Fatalf("initializing target: %v", err)
	}
	if server.activeTarget() != "aarch64-apple-darwin" {
		t.Fatalf("initial target was lost: %q", server.activeTarget())
	}
	if err := server.initializeProjectOptions(json.RawMessage(`{"initializationOptions":{"gecko":{"target":"invalid"}}}`)); err == nil {
		t.Fatal("invalid editor target was accepted")
	}
}

func TestMalformedOpenConfigHasOwnDiagnostic(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "gecko.toml")
	if err := os.WriteFile(configPath, []byte("[build]\nbackend = \"c\"\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	configURI := pathToURI(configPath)
	results, err := RunWorkspaceCheck(string(mainURI), "package main\n", map[string]string{configPath: "[build]\nbackend = [\n"})
	if err != nil {
		t.Fatalf("checking malformed config: %v", err)
	}
	if len(results[mainURI]) != 0 || !hasDiagnosticContaining(results[configURI], "parsing config") {
		t.Fatalf("config error was not isolated: %#v", results)
	}
	if results[configURI][0].Range.Start.Line != 1 {
		t.Fatalf("config diagnostic has wrong source range: %#v", results[configURI])
	}
}

func TestInvalidDefaultTargetHasConfigDiagnostic(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "gecko.toml")
	configURI := pathToURI(configPath)
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	results, err := RunWorkspaceCheck(string(mainURI), "package main\n", map[string]string{
		configPath: "[build]\ndefault_target = \"invalid\"\n",
	})
	if err != nil || !hasDiagnosticContaining(results[configURI], "invalid target") || len(results[mainURI]) != 0 {
		t.Fatalf("invalid project target was not attributed to config: %#v, %v", results, err)
	}
	results, err = RunWorkspaceCheckWithTarget(string(mainURI), "package main\n", map[string]string{
		configPath: "[build]\ndefault_target = \"invalid\"\n",
	}, "amd64-linux")
	if err != nil || !hasDiagnosticContaining(results[configURI], "invalid target") {
		t.Fatalf("editor override hid invalid project target: %#v, %v", results, err)
	}
}

func TestOpenConfigResolvesProjectImport(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "src", "main.gecko")
	modulePath := filepath.Join(root, "dependency.gecko")
	configPath := filepath.Join(root, "gecko.toml")
	store := NewDocumentStore()
	uri := pathToURI(mainPath)
	store.Open(uri, "package main\nimport dependency\n", 1)
	store.Open(pathToURI(configPath), "[build]\nbackend = \"c\"\n", 1)
	store.Open(pathToURI(modulePath), "package dependency\npublic class Widget {}\n", 1)
	doc, ok := store.Get(uri)
	if !ok {
		t.Fatal("main document not open")
	}
	doc.RebuildAnalysis(store.Snapshot())
	if doc.Analysis == nil || doc.Analysis.ImportedFiles["dependency"] == nil {
		t.Fatalf("open config did not supply project root: %#v", doc.Analysis)
	}
}

func TestWatchedConfigChangeRefreshesDiagnostics(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "gecko.toml")
	writeConfig := func(target string) {
		t.Helper()
		content := "[build]\nbackend = \"c\"\ndefault_target = \"" + target + "\"\n"
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			t.Fatalf("writing config: %v", err)
		}
	}
	writeConfig("aarch64-apple-darwin")
	serverSide, clientSide := net.Pipe()
	t.Cleanup(func() {
		if err := clientSide.Close(); err != nil {
			t.Errorf("closing client pipe: %v", err)
		}
		if err := serverSide.Close(); err != nil {
			t.Errorf("closing server pipe: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	server := NewServer()
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	published := make(chan protocol.PublishDiagnosticsParams, 16)
	registered := make(chan protocol.RegistrationParams, 1)
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if req.Method() == "textDocument/publishDiagnostics" {
			var params protocol.PublishDiagnosticsParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return err
			}
			published <- params
		}
		if req.Method() == "client/registerCapability" {
			var params protocol.RegistrationParams
			if err := json.Unmarshal(req.Params(), &params); err != nil {
				return err
			}
			registered <- params
		}
		return reply(ctx, nil, nil)
	})
	var initialized protocol.InitializeResult
	if _, err := client.Call(ctx, "initialize", map[string]any{
		"capabilities": map[string]any{"workspace": map[string]any{
			"didChangeWatchedFiles": map[string]any{"dynamicRegistration": true},
		}},
	}, &initialized); err != nil {
		t.Fatalf("initializing LSP: %v", err)
	}
	if err := client.Notify(ctx, "initialized", map[string]any{}); err != nil {
		t.Fatalf("notifying initialization: %v", err)
	}
	select {
	case registration := <-registered:
		if len(registration.Registrations) != 1 || registration.Registrations[0].Method != "workspace/didChangeWatchedFiles" {
			t.Fatalf("incorrect config watcher: %#v", registration)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("config watcher was not registered")
	}
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	server.documents.Open(mainURI, "package main\nimpl arm64 { func selected(): void {} }\n", 1)
	server.rebuildAnalysis(mainURI)
	server.publishDiagnostics(ctx, mainURI)
	first := awaitConfigDiagnostic(t, published, mainURI)
	if hasDiagnosticContaining(first.Diagnostics, "was skipped") {
		t.Fatalf("initial target was ignored: %#v", first.Diagnostics)
	}
	writeConfig("x86_64-unknown-linux-gnu")
	if err := client.Notify(ctx, "workspace/didChangeWatchedFiles", protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: protocol.DocumentURI(pathToURI(configPath)), Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("sending config change: %v", err)
	}
	changed := awaitConfigDiagnostic(t, published, mainURI)
	if !hasDiagnosticContaining(changed.Diagnostics, "was skipped") {
		t.Fatalf("config change did not recheck source: %#v", changed.Diagnostics)
	}
	if !strings.Contains(changed.Diagnostics[0].Message, "arm64") {
		t.Fatalf("wrong architecture warning: %#v", changed.Diagnostics)
	}
	if err := client.Notify(ctx, "workspace/didChangeConfiguration", map[string]any{
		"settings": map[string]any{"gecko": map[string]any{"target": "aarch64-apple-darwin"}},
	}); err != nil {
		t.Fatalf("changing editor target: %v", err)
	}
	overridden := awaitConfigDiagnostic(t, published, mainURI)
	if hasDiagnosticContaining(overridden.Diagnostics, "was skipped") {
		t.Fatalf("editor target override was ignored: %#v", overridden.Diagnostics)
	}
}

func TestOpenConfigEditPublishesAndClearsErrors(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "gecko.toml")
	if err := os.WriteFile(configPath, []byte("[build]\nbackend = \"c\"\n"), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	server := NewServer()
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	published := make(chan protocol.PublishDiagnosticsParams, 16)
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
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	configURI := pathToURI(configPath)
	server.documents.Open(mainURI, "package main\nimpl arm64 { func selected(): void {} }\n", 1)
	if err := client.Notify(ctx, "textDocument/didOpen", protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: configURI, Version: 1, Text: "[build]\nbackend = [\n"},
	}); err != nil {
		t.Fatalf("opening invalid config: %v", err)
	}
	waitProjectDiagnostics(t, published, configURI, mainURI, true, false)
	if err := client.Notify(ctx, "textDocument/didChange", protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: configURI}, Version: 2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: "[build]\nbackend = \"c\"\ndefault_target = \"x86_64-unknown-linux-gnu\"\n"}},
	}); err != nil {
		t.Fatalf("correcting config: %v", err)
	}
	waitProjectDiagnostics(t, published, configURI, mainURI, false, true)
}

func waitProjectDiagnostics(t *testing.T, published <-chan protocol.PublishDiagnosticsParams, configURI, mainURI protocol.DocumentURI, configError, mainWarning bool) {
	t.Helper()
	seenConfig := false
	seenMain := false
	deadline := time.After(6 * time.Second)
	for !seenConfig || !seenMain {
		select {
		case params := <-published:
			switch params.URI {
			case configURI:
				if configError == hasDiagnosticContaining(params.Diagnostics, "parsing config") && (!configError || len(params.Diagnostics) > 0) {
					seenConfig = true
				}
			case mainURI:
				if mainWarning == hasDiagnosticContaining(params.Diagnostics, "was skipped") && (mainWarning || len(params.Diagnostics) == 0) {
					seenMain = true
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for config diagnostics: config=%v main=%v", seenConfig, seenMain)
		}
	}
}

func awaitConfigDiagnostic(t *testing.T, published <-chan protocol.PublishDiagnosticsParams, uri protocol.DocumentURI) protocol.PublishDiagnosticsParams {
	t.Helper()
	for {
		select {
		case params := <-published:
			if params.URI == uri {
				return params
			}
		case <-time.After(6 * time.Second):
			t.Fatalf("timed out waiting for diagnostics on %s", uri)
		}
	}
}
