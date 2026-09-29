package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neutrino2211/gecko/parser"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestDocumentSymbolsIncludeMembers(t *testing.T) {
	content := "package test\nclass Widget {\n    let value: int32\n    func get(self): int32 { return self.value }\n}\ntrait Readable {\n    func read(self): int32\n}\nfunc main(): void {}\n"
	file, err := parser.Parser.ParseString("test.gecko", content)
	if err != nil {
		t.Fatalf("parsing document: %v", err)
	}
	symbols := documentSymbols(file, content)
	if len(symbols) != 3 {
		t.Fatalf("expected class, trait, and function: %#v", symbols)
	}
	class := symbols[0]
	if class.Name != "Widget" || class.Kind != protocol.SymbolKindClass || len(class.Children) != 2 {
		t.Fatalf("class hierarchy: %#v", class)
	}
	if class.Children[0].Name != "value" || class.Children[0].Kind != protocol.SymbolKindField || class.Children[1].Name != "get" {
		t.Fatalf("class members: %#v", class.Children)
	}
	if class.Range.End.Line < class.Children[1].SelectionRange.End.Line {
		t.Fatalf("class range does not contain method: %#v", class)
	}
	if symbols[1].Name != "Readable" || len(symbols[1].Children) != 1 || symbols[1].Children[0].Name != "read" {
		t.Fatalf("trait hierarchy: %#v", symbols[1])
	}
}

func TestDocumentSymbolsIncludeEnumCasesAndImplementationMethods(t *testing.T) {
	content := "package test\nenum Color { Red Green Blue }\nclass Point {}\nimpl Point { func origin(): Point { return Point {} } }\n"
	file, err := parser.Parser.ParseString("test.gecko", content)
	if err != nil {
		t.Fatalf("parsing document: %v", err)
	}
	symbols := documentSymbols(file, content)
	if len(symbols) != 3 || symbols[0].Name != "Color" || len(symbols[0].Children) != 3 {
		t.Fatalf("enum symbols: %#v", symbols)
	}
	for index, name := range []string{"Red", "Green", "Blue"} {
		child := symbols[0].Children[index]
		if child.Name != name || child.Kind != protocol.SymbolKindEnumMember || child.SelectionRange.Start.Line != 1 {
			t.Fatalf("enum case %q: %#v", name, child)
		}
	}
	implementation := symbols[2]
	if implementation.Name != "impl Point" || len(implementation.Children) != 1 || implementation.Children[0].Name != "origin" {
		t.Fatalf("implementation methods: %#v", implementation)
	}
	if implementation.Range.End.Line < implementation.Children[0].Range.End.Line {
		t.Fatalf("implementation range does not contain method: %#v", implementation)
	}
}

func TestDocumentAndWorkspaceSymbolProtocol(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	firstPath := filepath.Join(firstRoot, "widget.gecko")
	secondPath := filepath.Join(secondRoot, "readable.gecko")
	if err := os.WriteFile(firstPath, []byte("package first\nclass Widget {}\n"), 0o644); err != nil {
		t.Fatalf("writing first source: %v", err)
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
	uri := pathToURI(secondPath)
	server.documents.Open(uri, "package second\ntrait Readable { func read(self): int32 }\n", 1)
	server.rebuildAnalysis(uri)
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	var initialized protocol.InitializeResult
	_, err := client.Call(ctx, "initialize", map[string]any{
		"capabilities":     map[string]any{},
		"workspaceFolders": []map[string]string{{"uri": string(pathToURI(firstRoot))}, {"uri": string(pathToURI(secondRoot))}},
	}, &initialized)
	if err != nil || initialized.Capabilities.DocumentSymbolProvider != true || initialized.Capabilities.WorkspaceSymbolProvider != true {
		t.Fatalf("symbol capabilities: %#v, %v", initialized.Capabilities, err)
	}
	var document []protocol.DocumentSymbol
	_, err = client.Call(ctx, "textDocument/documentSymbol", protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}}, &document)
	if err != nil || len(document) != 1 || document[0].Name != "Readable" {
		t.Fatalf("document symbols: %#v, %v", document, err)
	}
	var workspace []protocol.SymbolInformation
	_, err = client.Call(ctx, "workspace/symbol", protocol.WorkspaceSymbolParams{Query: "read"}, &workspace)
	if err != nil || len(workspace) != 2 || workspace[0].Location.URI != uri {
		t.Fatalf("workspace symbols from open buffer: %#v, %v", workspace, err)
	}
	_, err = client.Call(ctx, "workspace/symbol", protocol.WorkspaceSymbolParams{Query: "Widget"}, &workspace)
	if err != nil || len(workspace) != 1 || workspace[0].Location.URI != pathToURI(firstPath) {
		t.Fatalf("workspace symbols from second root: %#v, %v", workspace, err)
	}
}

func TestWorkspaceSymbolsReuseFrontendSyntax(t *testing.T) {
	root := t.TempDir()
	closedPath := filepath.Join(root, "closed.gecko")
	openPath := filepath.Join(root, "open.gecko")
	if err := os.WriteFile(closedPath, []byte("package closed\nclass Widget {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	server := NewServer()
	server.workspaceRoots = []string{root}
	uri := pathToURI(openPath)
	server.documents.Open(uri, "package open\nclass Unsaved {}\n", 1)
	server.rebuildAnalysis(uri)
	beforeParses, beforeAnalyses := server.documents.frontendSession.Counts()
	for attempt := 0; attempt < 2; attempt++ {
		symbols, err := server.workspaceSymbols(context.Background(), "")
		if err != nil || len(symbols) != 2 {
			t.Fatalf("workspace symbols on request %d: %#v, %v", attempt, symbols, err)
		}
		parses, analyses := server.documents.frontendSession.Counts()
		if parses != beforeParses+1 || analyses != beforeAnalyses {
			t.Fatalf("workspace query repeated frontend work: %d parses, %d analyses", parses, analyses)
		}
	}
	if err := os.WriteFile(closedPath, []byte("package closed\nclass Changed {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	symbols, err := server.workspaceSymbols(context.Background(), "Changed")
	if err != nil || len(symbols) != 1 || symbols[0].Location.URI != pathToURI(closedPath) {
		t.Fatalf("edited source was not refreshed: %#v, %v", symbols, err)
	}
	if parses, _ := server.documents.frontendSession.Counts(); parses != beforeParses+2 {
		t.Fatalf("edited source was not reparsed once: %d", parses)
	}
}
