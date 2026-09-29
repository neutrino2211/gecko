package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neutrino2211/gecko/analysis"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestDependencyCacheCoversDirectoryImportsAndTransitiveFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gecko.toml"), []byte("[build]\nbackend = \"c\"\n"), 0o644); err != nil {
		t.Fatalf("writing project config: %v", err)
	}
	directory := filepath.Join(root, "module")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatalf("creating module: %v", err)
	}
	memberPath := filepath.Join(directory, "item.gecko")
	if err := os.WriteFile(memberPath, []byte("package item\nimport leaf\n"), 0o644); err != nil {
		t.Fatalf("writing directory member: %v", err)
	}
	leafPath := filepath.Join(root, "leaf.gecko")
	if err := os.WriteFile(leafPath, []byte("package leaf\n"), 0o644); err != nil {
		t.Fatalf("writing leaf: %v", err)
	}
	mainURI := pathToURI(filepath.Join(root, "src", "main.gecko"))
	leafURI := pathToURI(leafPath)
	store := NewDocumentStore()
	store.Open(mainURI, "package main\nimport module\n", 1)
	main, ok := store.Get(mainURI)
	if !ok {
		t.Fatal("main document missing")
	}
	main.RebuildAnalysis(store.Snapshot())
	if !store.SetAnalysis(main) {
		t.Fatal("setting main analysis")
	}
	unrelated := pathToURI(filepath.Join(root, "unrelated.gecko"))
	if dependents := store.Dependents(unrelated); len(dependents) != 0 {
		t.Fatalf("unrelated file has dependents: %#v", dependents)
	}
	first := store.dependencies[mainURI]
	if first == nil || !first.contains(leafPath) || !first.contains(filepath.Join(directory, "new.gecko")) {
		t.Fatalf("missing transitive directory dependency: %#v", first)
	}
	store.Dependents(unrelated)
	if store.dependencies[mainURI] != first {
		t.Fatal("dependency closure was rebuilt for an unrelated query")
	}
	if dependents := store.Dependents(leafURI); len(dependents) != 1 || dependents[0] != mainURI {
		t.Fatalf("missing transitive importer: %#v", dependents)
	}
	if store.dependencies[mainURI] != nil {
		t.Fatal("changed dependency did not invalidate its cached closure")
	}
}

func TestWatchedSourceChangeRefreshesOpenImporter(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic class Old {}\n"), 0o644); err != nil {
		t.Fatalf("writing library: %v", err)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := NewServer()
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	server.documents.Open(mainURI, "package main\nimport lib\n", 1)
	server.rebuildAnalysis(mainURI)
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.requestHandler())
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	defer server.cancelDiagnostics(mainURI)
	if err := os.WriteFile(libPath, []byte("package lib\npublic class New {}\n"), 0o644); err != nil {
		t.Fatalf("updating library: %v", err)
	}
	if err := client.Notify(ctx, "workspace/didChangeWatchedFiles", protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: pathToURI(libPath), Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("notifying file change: %v", err)
	}
	var symbols []protocol.DocumentSymbol
	if _, err := client.Call(ctx, "textDocument/documentSymbol", protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: mainURI},
	}, &symbols); err != nil {
		t.Fatalf("waiting for file change: %v", err)
	}
	main, ok := server.documents.Get(mainURI)
	if !ok || main.Analysis == nil || main.Analysis.ImportedFiles["lib"] == nil {
		t.Fatalf("importer analysis missing: %#v", main)
	}
	entries := main.Analysis.ImportedFiles["lib"].Entries
	if len(entries) != 1 || entries[0].Class == nil || entries[0].Class.Name != "New" {
		t.Fatalf("watched file did not refresh importer: %#v", entries)
	}
}

func TestAnalysisResolvesTransitiveImportsWithCycle(t *testing.T) {
	root := t.TempDir()
	for _, fixture := range []struct{ name, source string }{
		{"child", "package child\nimport parent\npublic trait Child {}\n"},
		{"parent", "package parent\nimport child\npublic trait Parent {}\n"},
	} {
		path := filepath.Join(root, fixture.name+".gecko")
		if err := os.WriteFile(path, []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	ctx, err := analysis.NewAnalysisContext(mainPath, "package main\nimport child\n")
	if err != nil {
		t.Fatalf("analyzing import cycle: %v", err)
	}
	child := ctx.ImportedFiles["child"]
	if child == nil || len(child.Imports) != 1 || len(child.Imports[0].Imports) != 1 || child.Imports[0].Imports[0] != child {
		t.Fatalf("transitive import graph did not reuse the cycle: %#v", child)
	}
}
