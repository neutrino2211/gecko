package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func containsSourcePath(paths []string, path string) bool {
	for _, candidate := range paths {
		if candidate == path {
			return true
		}
	}
	return false
}

func TestProjectSourceDiscoveryCachesSymlinkedDirectories(t *testing.T) {
	root := t.TempDir()
	linkedRoot := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	linkedPath := filepath.Join(linkedRoot, "linked.gecko")
	if err := os.WriteFile(mainPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("writing main source: %v", err)
	}
	if err := os.WriteFile(linkedPath, []byte("package linked\nclass Linked {}\n"), 0o644); err != nil {
		t.Fatalf("writing linked source: %v", err)
	}
	link := filepath.Join(root, "alias")
	if err := os.Symlink(linkedRoot, link); err != nil {
		t.Fatalf("linking source directory: %v", err)
	}
	if err := os.Symlink(root, filepath.Join(root, "loop")); err != nil {
		t.Fatalf("linking cycle: %v", err)
	}
	server := NewServer()
	paths, err := server.projectSourcePaths(context.Background(), root)
	if err != nil || len(paths) != 2 || !containsSourcePath(paths, filepath.Join(link, "linked.gecko")) {
		t.Fatalf("discovered sources: %#v, %v", paths, err)
	}
	cachedEntry := server.sourcePaths[root]
	if _, err := server.projectSourcePaths(context.Background(), root); err != nil || server.sourcePaths[root] != cachedEntry {
		t.Fatalf("unchanged source tree did not reuse cache: %v", err)
	}
	server.workspaceRoots = []string{root}
	symbols, err := server.workspaceSymbols(context.Background(), "Linked")
	if err != nil || len(symbols) != 1 || symbols[0].Location.URI != pathToURI(filepath.Join(link, "linked.gecko")) {
		t.Fatalf("linked workspace symbol: %#v, %v", symbols, err)
	}
	created := filepath.Join(root, "created.gecko")
	if err := os.WriteFile(created, []byte("package created\n"), 0o644); err != nil {
		t.Fatalf("writing new source: %v", err)
	}
	if refreshed, err := server.projectSourcePaths(context.Background(), root); err != nil || !containsSourcePath(refreshed, created) || server.sourcePaths[root] == cachedEntry {
		t.Fatalf("new source was missed by cache validation: %#v, %v", refreshed, err)
	}
	server.invalidateSourcePaths()
	if refreshed, err := server.projectSourcePaths(context.Background(), root); err != nil || !containsSourcePath(refreshed, created) {
		t.Fatalf("source cache was not invalidated: %#v, %v", refreshed, err)
	}
}

func TestProjectSourceDiscoveryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := discoverSourcePaths(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled discovery returned %v", err)
	}
}

func TestWatchedSourceCreationRefreshesWorkspaceSymbols(t *testing.T) {
	root := t.TempDir()
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
	server.workspaceRoots = []string{root}
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	var symbols []protocol.SymbolInformation
	if _, err := client.Call(ctx, "workspace/symbol", protocol.WorkspaceSymbolParams{Query: "Fresh"}, &symbols); err != nil || len(symbols) != 0 {
		t.Fatalf("initial symbols: %#v, %v", symbols, err)
	}
	created := filepath.Join(root, "fresh.gecko")
	if err := os.WriteFile(created, []byte("package fresh\nclass Fresh {}\n"), 0o644); err != nil {
		t.Fatalf("writing new source: %v", err)
	}
	if err := client.Notify(ctx, "workspace/didChangeWatchedFiles", protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: pathToURI(created), Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("notifying new source: %v", err)
	}
	if _, err := client.Call(ctx, "workspace/symbol", protocol.WorkspaceSymbolParams{Query: "Fresh"}, &symbols); err != nil || len(symbols) != 1 || symbols[0].Location.URI != pathToURI(created) {
		t.Fatalf("symbols after watched creation: %#v, %v", symbols, err)
	}
}
