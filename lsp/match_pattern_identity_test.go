package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchDestructureLinksClassAndFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nclass Point { let x: int32 let y: int32 }\nfunc classify(p: Point): int32 { return match p { Point { x = 0, y = _ } => 1 default => 0 } }\n"
	server := NewServer()
	uri := pathToURI(path)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("analysis missing")
	}
	for _, fixture := range []struct {
		name        string
		declaration string
		use         string
	}{
		{"Point", "class Point", "Point { x"},
		{"x", "let x", "{ x ="},
		{"y", "let y", ", y ="},
	} {
		declaration := strings.Index(content, fixture.declaration) + len(fixture.declaration) - len(fixture.name)
		use := strings.Index(content, fixture.use) + strings.Index(fixture.use, fixture.name)
		definition := semanticDefinition(doc.Analysis, path, content, sourcePosition(content, use))
		if definition == nil || definition.Range.Start != sourcePosition(content, declaration) {
			t.Fatalf("%s pattern definition: %#v", fixture.name, definition)
		}
		edit, err := server.renameWorkspaceSymbol(context.Background(), doc, sourcePosition(content, use), fixture.name+"New")
		if err != nil || edit == nil {
			t.Fatalf("%s pattern rename: %#v, %v", fixture.name, edit, err)
		}
		expected := 2
		if fixture.name == "Point" {
			expected = 3
		}
		if len(edit.Changes[uri]) != expected {
			t.Fatalf("%s pattern rename edits: %#v", fixture.name, edit.Changes[uri])
		}
	}
}
