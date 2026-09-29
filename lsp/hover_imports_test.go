package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrino2211/gecko/analysis"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func hoverPosition(t *testing.T, source string) (string, int, int) {
	t.Helper()
	index := strings.IndexByte(source, '|')
	if index < 0 || strings.Count(source, "|") != 1 {
		t.Fatalf("expected one cursor marker in %q", source)
	}
	line := strings.Count(source[:index], "\n")
	column := index - strings.LastIndexByte(source[:index], '\n') - 1
	return source[:index] + source[index+1:], line, column
}

func TestImportedTypeHoverUsesQualifiedModule(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	bridgePath := filepath.Join(root, "bridge.gecko")
	bridge := "package bridge\n/// # Imported Widget\n///\n/// **Bold** description.\n///\n/// ```gecko\n/// let x: Widget\n/// ```\npublic class Widget {}\n"
	source, line, column := hoverPosition(t, "package main\nimport bridge as native\nclass Widget {}\nlet item: native.Wid|get\n")
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, source, func(path string) ([]byte, error) {
		if path == bridgePath {
			return []byte(bridge), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("analyzing imported type: %v", err)
	}
	info := GetHoverInfo(ctx, source, line, column)
	if info == nil || info.Type != "public class Widget" {
		t.Fatalf("imported type hover = %#v", info)
	}
	wantDoc := "# Imported Widget\n\n**Bold** description.\n\n```gecko\nlet x: Widget\n```"
	if info.DocComment != wantDoc {
		t.Fatalf("imported documentation = %q, want %q", info.DocComment, wantDoc)
	}
	location := GetDefinitionLocation(ctx, source, line, column, string(pathToURI(mainPath)))
	if location == nil || location.URI != pathToURI(bridgePath) {
		t.Fatalf("imported definition = %#v", location)
	}
}

func TestSelectedImportedTypeHover(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	bridgePath := filepath.Join(root, "bridge.gecko")
	bridge := "package bridge\n/// Selected documentation.\npublic class Widget {}\n"
	source, line, column := hoverPosition(t, "package main\nimport bridge use {Widget}\nlet item: Wid|get\n")
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, source, func(path string) ([]byte, error) {
		if path == bridgePath {
			return []byte(bridge), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("analyzing selected import: %v", err)
	}
	info := GetHoverInfo(ctx, source, line, column)
	if info == nil || info.Type != "public class Widget" || info.DocComment != "Selected documentation." {
		t.Fatalf("selected import hover = %#v", info)
	}
}

func TestDirectoryImportedTypeHoverBeforeBackend(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	typePath := filepath.Join(root, "library", "widget.gecko")
	if err := os.MkdirAll(filepath.Dir(typePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(typePath, []byte("package library\n/// Directory widget.\npublic class Widget {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	source, line, column := hoverPosition(t, "package main\nimport library as Lib\nlet item: Lib.Wid|get\n")
	ctx, err := analysis.NewAnalysisContext(mainPath, source)
	if err != nil {
		t.Fatal(err)
	}
	info := GetHoverInfo(ctx, source, line, column)
	if info == nil || info.Type != "public class Widget" || info.DocComment != "Directory widget." {
		t.Fatalf("directory type hover = %#v", info)
	}
	location := GetDefinitionLocation(ctx, source, line, column, string(pathToURI(mainPath)))
	if location == nil || location.URI != pathToURI(typePath) {
		t.Fatalf("directory type definition = %#v", location)
	}
}

func TestDirectoryModuleQueriesFindSymbolsAcrossFiles(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "library")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"widget.gecko": "package library\n/// Widget documentation.\npublic class Widget {}\n",
		"extra.gecko":  "package library\n/// Extra documentation.\npublic func extra(): int32 { return 3 }\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "main.gecko")
	source := "package main\nimport library as Lib\nfunc use(item: Lib.Widget): int32 { return Lib.extra() }\n"
	ctx, err := analysis.NewAnalysisContext(path, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.FilesForModule("Lib")) != 2 {
		t.Fatalf("expected both module files, got %d", len(ctx.FilesForModule("Lib")))
	}
	for _, check := range []struct{ name, path, doc string }{
		{"Widget", filepath.Join(directory, "widget.gecko"), "Widget documentation."},
		{"extra", filepath.Join(directory, "extra.gecko"), "Extra documentation."},
	} {
		offset := strings.Index(source, "Lib."+check.name) + len("Lib.")
		position := sourcePosition(source, offset)
		info := GetHoverInfo(ctx, source, int(position.Line), int(position.Character))
		if info == nil || info.DocComment != check.doc {
			t.Fatalf("%s hover = %#v", check.name, info)
		}
		location := GetDefinitionLocation(ctx, source, int(position.Line), int(position.Character), string(pathToURI(path)))
		if location == nil || location.URI != pathToURI(check.path) {
			t.Fatalf("%s definition = %#v", check.name, location)
		}
	}
	items := getImportedModuleCompletions(ctx, ctx.MainFile, path, "Lib", "")
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Label] = true
	}
	if !seen["Widget"] || !seen["extra"] {
		t.Fatalf("module completions missed a file: %#v", seen)
	}
}

func TestImportedTypeKindsHover(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	bridgePath := filepath.Join(root, "bridge.gecko")
	bridge := "package bridge\n/// Reads values.\npublic trait Reader { func read(self): int32 }\nenum Status { Ready }\nforeign \"c\" c_abi { type Handle opaque }\ndeclare external type Legacy\n"
	source := "package main\nimport bridge\nlet reader: bridge.Reader\nlet status: bridge.Status\nlet handle: bridge.Handle*\nlet legacy: bridge.Legacy*\n"
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, source, func(path string) ([]byte, error) {
		if path == bridgePath {
			return []byte(bridge), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("analyzing imported types: %v", err)
	}
	for _, test := range []struct {
		line int
		name string
		typ  string
		doc  string
	}{{2, "Reader", "public trait Reader", "Reads values."}, {3, "Status", "enum Status", ""}, {4, "Handle", "type Handle opaque", ""}, {5, "Legacy", "declare external type Legacy", ""}} {
		column := strings.Index(strings.Split(source, "\n")[test.line], test.name)
		info := GetHoverInfo(ctx, source, test.line, column)
		if info == nil || info.Type != test.typ || info.DocComment != test.doc {
			t.Errorf("%s hover = %#v", test.name, info)
		}
	}
}

func TestHoverResponseContainsRenderedMarkdownSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	source, line, column := hoverPosition(t, "package main\n/// # Widget\n///\n/// ```gecko\n/// let x: int32\n/// ```\npublic class Wid|get {}\n")
	analysisContext, err := analysis.NewAnalysisContext(path, source)
	if err != nil {
		t.Fatalf("analyzing hover source: %v", err)
	}
	serverSide, clientSide := net.Pipe()
	t.Cleanup(func() {
		if err := serverSide.Close(); err != nil {
			t.Errorf("closing server pipe: %v", err)
		}
		if err := clientSide.Close(); err != nil {
			t.Errorf("closing client pipe: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server := NewServer()
	uri := pathToURI(path)
	server.documents.Open(uri, source, 1)
	document, ok := server.documents.Get(uri)
	if !ok {
		t.Fatal("opened document missing")
	}
	document.Analysis = analysisContext
	if !server.documents.SetAnalysis(document) {
		t.Fatal("could not set document analysis")
	}
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	params := protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Position:     protocol.Position{Line: uint32(line), Character: uint32(column)},
	}}
	var result map[string]any
	if _, err := client.Call(ctx, "textDocument/hover", params, &result); err != nil {
		t.Fatalf("requesting hover: %v", err)
	}
	contents, ok := result["contents"].(map[string]any)
	if !ok || contents["kind"] != "markdown" {
		t.Fatalf("hover contents = %#v", result["contents"])
	}
	want := "```gecko\npublic class Widget\n```\n\n# Widget\n\n```gecko\nlet x: int32\n```"
	if contents["value"] != want {
		t.Fatalf("hover markdown = %#v, want %q", contents["value"], want)
	}
}
