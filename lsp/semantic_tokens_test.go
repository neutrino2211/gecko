package main

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neutrino2211/gecko/analysis"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestSemanticTokensUseResolvedSymbolKinds(t *testing.T) {
	content := "package test\nclass Box { let value: int32 }\nfunc use(): void {\n    let b: Box = Box { value: 1 }\n    let x: int32 = b.value\n}\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing source: %v", err)
	}
	data := semanticTokens(&Document{URI: pathToURI(path), Content: content, Analysis: ctx}).Data
	if len(data) == 0 || len(data)%5 != 0 {
		t.Fatalf("invalid semantic token data: %#v", data)
	}
	var line, character uint32
	foundClassDeclaration := false
	foundFieldUse := false
	for offset := 0; offset < len(data); offset += 5 {
		line += data[offset]
		if data[offset] == 0 {
			character += data[offset+1]
		} else {
			character = data[offset+1]
		}
		if line == 1 && character == 6 && data[offset+2] == 3 && data[offset+3] == 0 && data[offset+4] == 1 {
			foundClassDeclaration = true
		}
		if line == 4 && data[offset+3] == 3 && data[offset+4] == 0 {
			foundFieldUse = true
		}
	}
	if !foundClassDeclaration || !foundFieldUse {
		t.Fatalf("missing class declaration or field use: %#v", data)
	}
}

func TestTraitDefinitionAndSemanticTokens(t *testing.T) {
	content := "package test\ntrait Parent { func read(self): int32 }\ntrait Child: Parent { func next(self): int32 }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing source: %v", err)
	}
	definition := semanticDefinition(ctx, path, content, protocol.Position{Line: 2, Character: 14})
	if definition == nil || definition.Range.Start.Line != 1 || definition.Range.Start.Character != 6 {
		t.Fatalf("parent trait definition: %#v", definition)
	}
	data := semanticTokens(&Document{URI: pathToURI(path), Content: content, Analysis: ctx}).Data
	var line, character uint32
	declaration := false
	use := false
	for offset := 0; offset+5 <= len(data); offset += 5 {
		line += data[offset]
		if data[offset] == 0 {
			character += data[offset+1]
		} else {
			character = data[offset+1]
		}
		if data[offset+3] == 5 && line == 1 && character == 6 && data[offset+4] == 1 {
			declaration = true
		}
		if data[offset+3] == 5 && line == 2 && character == 13 && data[offset+4] == 0 {
			use = true
		}
	}
	if !declaration || !use {
		t.Fatalf("missing trait semantic tokens: %#v", data)
	}
}

func TestSemanticTokensIncludeTypeParameters(t *testing.T) {
	content := "package test\nclass Box<T> { let value: T }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing type parameter: %v", err)
	}
	legend := semanticTokenLegend()
	kind := -1
	for index, tokenType := range legend.TokenTypes {
		if tokenType == protocol.SemanticTokenTypeParameter {
			kind = index
			break
		}
	}
	if kind < 0 {
		t.Fatal("type parameter token kind missing")
	}
	data := semanticTokens(&Document{URI: pathToURI(path), Content: content, Analysis: ctx}).Data
	want := map[uint32]uint32{
		sourcePosition(content, strings.Index(content, "Box<T>")+len("Box<")).Character:      1,
		sourcePosition(content, strings.Index(content, "value: T")+len("value: ")).Character: 0,
	}
	var line, character uint32
	for offset := 0; offset+5 <= len(data); offset += 5 {
		line += data[offset]
		if data[offset] == 0 {
			character += data[offset+1]
		} else {
			character = data[offset+1]
		}
		if line == 1 && data[offset+3] == uint32(kind) {
			modifier, ok := want[character]
			if !ok || modifier != data[offset+4] {
				t.Fatalf("unexpected type parameter token: %#v", data)
			}
			delete(want, character)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing type parameter tokens: %#v", want)
	}
}

func TestSemanticTokensProtocol(t *testing.T) {
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
	uri := pathToURI(t.TempDir() + "/main.gecko")
	server.documents.Open(uri, "package test\nclass Box {}\n", 1)
	server.rebuildAnalysis(uri)
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	var initialized protocol.InitializeResult
	if _, err := client.Call(ctx, "initialize", map[string]any{"capabilities": map[string]any{}}, &initialized); err != nil || initialized.Capabilities.SemanticTokensProvider == nil {
		t.Fatalf("semantic token capability: %#v, %v", initialized.Capabilities, err)
	}
	var tokens protocol.SemanticTokens
	_, err := client.Call(ctx, "textDocument/semanticTokens/full", protocol.SemanticTokensParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}}, &tokens)
	if err != nil || len(tokens.Data) < 15 || len(tokens.Data)%5 != 0 {
		t.Fatalf("semantic token response: %#v, %v", tokens, err)
	}
}

func TestSemanticTokensHighlightSyntaxWithoutAnalysis(t *testing.T) {
	content := "package demo\n/// documentation\ndeclare external func open(): int32\nlet name: string = \"hello\"\nlet count: int32 = 42\n"
	doc := &Document{URI: pathToURI(filepath.Join(t.TempDir(), "main.gecko")), Content: content}
	data := semanticTokens(doc).Data
	for _, expected := range []struct {
		text      string
		kind      uint32
		modifiers uint32
	}{
		{"package", semanticKeyword, 0},
		{"/// documentation", semanticComment, 2},
		{"declare", semanticKeyword, 0},
		{"int32", semanticType, 0},
		{"\"hello\"", semanticString, 0},
		{"42", semanticNumber, 0},
	} {
		if !containsSemanticToken(content, data, expected.text, expected.kind, expected.modifiers) {
			t.Fatalf("missing %s token in %#v", expected.text, data)
		}
	}
}

func TestSemanticTokensKeepColorsDuringIncompleteEdit(t *testing.T) {
	content := "package demo\nlet text: string = `first\nsecond`\nlet unfinished: string = \""
	doc := &Document{URI: pathToURI(filepath.Join(t.TempDir(), "main.gecko")), Content: content}
	data := semanticTokens(doc).Data
	for _, text := range []string{"package", "`first", "second`"} {
		kind := semanticString
		if text == "package" {
			kind = semanticKeyword
		}
		if !containsSemanticToken(content, data, text, kind, 0) {
			t.Fatalf("missing %s token during incomplete edit: %#v", text, data)
		}
	}
}

func containsSemanticToken(content string, data []uint32, text string, kind, modifiers uint32) bool {
	offset := strings.Index(content, text)
	if offset < 0 {
		return false
	}
	start := sourcePosition(content, offset)
	end := sourcePosition(content, offset+len(text))
	var line, character uint32
	for index := 0; index+5 <= len(data); index += 5 {
		line += data[index]
		if data[index] == 0 {
			character += data[index+1]
		} else {
			character = data[index+1]
		}
		if line == start.Line && character == start.Character &&
			data[index+2] == end.Character-start.Character &&
			data[index+3] == kind && data[index+4] == modifiers {
			return true
		}
	}
	return false
}
