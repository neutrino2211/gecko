package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/analysis"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func TestEnumTypesAndCasesHaveSemanticIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nenum Color { Red Green Blue }\nfunc choose(c: Color): Color { return match c { Color.Red => Color.Blue default => Color.Green } }\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing enums: %v", err)
	}
	for _, fixture := range []struct {
		name        string
		declaration string
		uses        []string
	}{
		{"Color", "enum Color", []string{"c: Color", "): Color", "Color.Red", "Color.Blue", "Color.Green"}},
		{"Red", "{ Red", []string{"Color.Red"}},
		{"Blue", "Red Green Blue", []string{"Color.Blue"}},
		{"Green", "Red Green", []string{"Color.Green"}},
	} {
		declarationOffset := strings.Index(content, fixture.declaration) + len(fixture.declaration) - len(fixture.name)
		declaration, ok := ctx.SemanticGraph.OccurrenceAt(path, declarationOffset)
		if !ok || !declaration.Declaration {
			t.Fatalf("missing %s declaration: %#v", fixture.name, declaration)
		}
		for _, use := range fixture.uses {
			offset := strings.Index(content, use) + len(use) - len(fixture.name)
			if use == "Color.Red" || use == "Color.Blue" || use == "Color.Green" {
				if fixture.name == "Color" {
					offset = strings.Index(content, use)
				}
			}
			occurrence, found := ctx.SemanticGraph.OccurrenceAt(path, offset)
			if !found || occurrence.SymbolID != declaration.SymbolID {
				t.Fatalf("%s in %q has wrong identity: %#v", fixture.name, use, occurrence)
			}
		}
	}
	data := semanticTokens(&Document{URI: pathToURI(path), Content: content, Analysis: ctx}).Data
	enumTokens := 0
	caseTokens := 0
	for index := 0; index+5 <= len(data); index += 5 {
		switch data[index+3] {
		case 8:
			enumTokens++
		case 9:
			caseTokens++
		}
	}
	if enumTokens != 6 || caseTokens != 6 {
		t.Fatalf("enum semantic tokens: types=%d cases=%d", enumTokens, caseTokens)
	}
}

func TestImportedEnumDefinitionAndWorkspaceRename(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	libContent := "package lib\nenum Color { Red Blue }\n"
	if err := os.WriteFile(libPath, []byte(libContent), 0o644); err != nil {
		t.Fatalf("writing enum module: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib use { Color }\nfunc choose(c: Color): Color { return match c { Color.Red => Color.Blue default => Color.Red } }\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	for _, name := range []string{"Color", "Red"} {
		offset := strings.Index(content, "Color.Red")
		if name == "Red" {
			offset += len("Color.")
		}
		definition := semanticDefinition(doc.Analysis, mainPath, content, sourcePosition(content, offset))
		if definition == nil || definition.URI != pathToURI(libPath) {
			t.Fatalf("%s definition: %#v", name, definition)
		}
		position := sourcePosition(content, offset)
		request, err := jsonrpc2.NewCall(jsonrpc2.NewStringID(name), "textDocument/prepareRename", protocol.PrepareRenameParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: uri}, Position: position,
			},
		})
		if err != nil {
			t.Fatalf("creating prepare rename request: %v", err)
		}
		var prepared any
		err = server.handlePrepareRename(context.Background(), func(_ context.Context, result any, replyErr error) error {
			prepared = result
			return replyErr
		}, request)
		if err != nil || prepared == nil {
			t.Fatalf("preparing %s rename: %#v, %v", name, prepared, err)
		}
		newName := name + "New"
		edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, newName)
		if err != nil || edit == nil {
			t.Fatalf("renaming %s: %#v, %v", name, edit, err)
		}
		if len(edit.Changes[pathToURI(libPath)]) != 1 {
			t.Fatalf("%s declaration edits: %#v", name, edit.Changes[pathToURI(libPath)])
		}
		expectedUses := 6
		if name == "Red" {
			expectedUses = 2
		}
		if len(edit.Changes[uri]) != expectedUses {
			t.Fatalf("%s use edits: %#v", name, edit.Changes[uri])
		}
	}
}

func TestQualifiedEnumsKeepSeparateIdentities(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second"} {
		path := filepath.Join(root, name+".gecko")
		if err := os.WriteFile(path, []byte("package "+name+"\nenum Color { Red }\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport first\nimport second\nfunc choose(): void { let a: first.Color = first.Color.Red let b: second.Color = second.Color.Red }\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing qualified enums: %v", err)
	}
	for _, name := range []string{"first", "second"} {
		start := strings.Index(content, name+".Color.Red")
		for _, offset := range []int{start + len(name) + 1, start + len(name) + len(".Color.")} {
			definition := semanticDefinition(ctx, path, content, sourcePosition(content, offset))
			if definition == nil || definition.URI != pathToURI(filepath.Join(root, name+".gecko")) {
				t.Fatalf("%s qualified enum definition: %#v", name, definition)
			}
		}
	}
}

func TestGlobalEnumInitializerLinksCase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nenum Color { Red Blue }\nlet picked: Color = Color.Red\nlet copied: Color = picked\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing global enum initializer: %v", err)
	}
	declaration := strings.Index(content, "{ Red") + len("{ ")
	use := strings.Index(content, "Color.Red") + len("Color.")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, use))
	if definition == nil || definition.Range.Start != sourcePosition(content, declaration) {
		t.Fatalf("global enum case definition: %#v", definition)
	}
	declaration = strings.Index(content, "let picked") + len("let ")
	use = strings.Index(content, "= picked") + len("= ")
	declarationOccurrence, declarationOK := ctx.SemanticGraph.OccurrenceAt(path, declaration)
	useOccurrence, useOK := ctx.SemanticGraph.OccurrenceAt(path, use)
	if !declarationOK || !useOK || declarationOccurrence.SymbolID != useOccurrence.SymbolID {
		t.Fatalf("global variable identity: %#v %#v", declarationOccurrence, useOccurrence)
	}
}
