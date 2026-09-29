package main

import (
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
)

func TestDocumentRecoversAfterUnfinishedMultilineDeclaration(t *testing.T) {
	content := "package main\nfunc before(): void {}\nfunc broken(\n    first: int32,\n    second: int32\nfunc after(): void {}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	doc := &Document{URI: pathToURI(path), Content: content}
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil || len(doc.Analysis.SemanticGraph.FunctionsNamed("before")) != 1 || len(doc.Analysis.SemanticGraph.FunctionsNamed("after")) != 1 {
		t.Fatalf("lost declarations around unfinished multiline declaration: %#v", doc.Analysis)
	}
	position := protocol.Position{Line: 5, Character: 5}
	definition := semanticDefinition(doc.Analysis, path, content, position)
	if definition == nil || definition.Range.Start != position {
		t.Fatalf("later declaration moved during recovery: %#v", definition)
	}
}

func TestDocumentRecoversAfterUnclosedClassBody(t *testing.T) {
	content := "package main\nfunc before(): void {}\nclass Broken {\n    let value: int32\nfunc after(): void {}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	doc := &Document{URI: pathToURI(path), Content: content}
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil || len(doc.Analysis.SemanticGraph.FunctionsNamed("before")) != 1 || len(doc.Analysis.SemanticGraph.FunctionsNamed("after")) != 1 {
		t.Fatalf("lost declarations around unclosed class: %#v", doc.Analysis)
	}
	topLevel := false
	for _, entry := range doc.Analysis.MainFile.Entries {
		if entry != nil && entry.Method != nil && entry.Method.Name == "after" {
			topLevel = true
		}
	}
	if !topLevel {
		t.Fatal("later function was absorbed into unfinished class")
	}
	position := protocol.Position{Line: 4, Character: 5}
	definition := semanticDefinition(doc.Analysis, path, content, position)
	if definition == nil || definition.Range.Start != position {
		t.Fatalf("later declaration moved during recovery: %#v", definition)
	}
}

func TestDocumentRecoversAfterUnclosedFunctionBody(t *testing.T) {
	content := "package main\nfunc broken(): void {\n    let value: int32 = 1\nfunc after(): void {}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	doc := &Document{URI: pathToURI(path), Content: content}
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil {
		t.Fatal("analysis was lost after unclosed function")
	}
	for _, entry := range doc.Analysis.MainFile.Entries {
		if entry != nil && entry.Method != nil && entry.Method.Name == "after" {
			return
		}
	}
	t.Fatal("later top-level function was hidden by unfinished body")
}

func TestDocumentRecoversAfterUnclosedGenericHeader(t *testing.T) {
	content := "package main\nfunc before(): void {}\nfunc broken<\n    T is Printable,\n    U\nfunc after(): void {}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	doc := &Document{URI: pathToURI(path), Content: content}
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil || len(doc.Analysis.SemanticGraph.FunctionsNamed("before")) != 1 || len(doc.Analysis.SemanticGraph.FunctionsNamed("after")) != 1 {
		t.Fatalf("lost declarations around unfinished generic header: %#v", doc.Analysis)
	}
	position := protocol.Position{Line: 5, Character: 5}
	definition := semanticDefinition(doc.Analysis, path, content, position)
	if definition == nil || definition.Range.Start != position {
		t.Fatalf("later declaration moved during recovery: %#v", definition)
	}
}
