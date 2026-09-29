package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestWorkspaceRenameClassAcrossTypeAndInitializerUses(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic class Widget { let value: int32 }\n"), 0o644); err != nil {
		t.Fatalf("writing class declaration: %v", err)
	}
	otherPath := filepath.Join(root, "other.gecko")
	if err := os.WriteFile(otherPath, []byte("package other\nclass Widget {}\n"), 0o644); err != nil {
		t.Fatalf("writing unrelated class: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib use { Widget }\nfunc make(): void { let item: Widget = Widget { value: 1 } }\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	importPosition := protocol.Position{Line: 1, Character: uint32(strings.Index(strings.Split(content, "\n")[1], "Widget"))}
	definition := semanticDefinition(doc.Analysis, mainPath, content, importPosition)
	if definition == nil || definition.URI != pathToURI(libPath) || definition.Range.Start.Line != 1 {
		t.Fatalf("imported class definition: %#v", definition)
	}
	position := protocol.Position{Line: 2, Character: uint32(strings.Index(strings.Split(content, "\n")[2], "Widget"))}
	edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "Gadget")
	if err != nil {
		t.Fatalf("renaming class: %v", err)
	}
	if edit == nil || len(edit.Changes[pathToURI(libPath)]) != 1 || len(edit.Changes[uri]) != 3 || len(edit.Changes[pathToURI(otherPath)]) != 0 {
		t.Fatalf("class rename edits: %#v", edit)
	}
}

func TestWorkspaceRenameTraitAcrossParentAndImplementation(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic trait Parent { func read(self): int32 }\n"), 0o644); err != nil {
		t.Fatalf("writing trait declaration: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib\ntrait Child: Parent { func next(self): int32 }\nclass Box {}\nimpl Parent for Box { func read(self): int32 { return 1 } }\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := protocol.Position{Line: 2, Character: uint32(strings.Index(strings.Split(content, "\n")[2], "Parent"))}
	edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "Ancestor")
	if err != nil {
		t.Fatalf("renaming trait: %v", err)
	}
	if edit == nil || len(edit.Changes[pathToURI(libPath)]) != 1 || len(edit.Changes[uri]) != 2 {
		t.Fatalf("trait rename edits: %#v", edit)
	}
}

func TestWorkspaceRenameFieldAcrossInitializerAndAccess(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic class Widget { let value: int32 }\n"), 0o644); err != nil {
		t.Fatalf("writing field declaration: %v", err)
	}
	otherPath := filepath.Join(root, "other.gecko")
	if err := os.WriteFile(otherPath, []byte("package other\nclass Other { let value: int32 }\n"), 0o644); err != nil {
		t.Fatalf("writing unrelated field: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib use { Widget }\nfunc make(): void {\n    let item: Widget = Widget { value: 1 }\n    let read: int32 = item.value\n}\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := protocol.Position{Line: 4, Character: uint32(strings.Index(strings.Split(content, "\n")[4], "value"))}
	edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "amount")
	if err != nil {
		t.Fatalf("renaming field: %v", err)
	}
	if edit == nil || len(edit.Changes[pathToURI(libPath)]) != 1 || len(edit.Changes[uri]) != 2 || len(edit.Changes[pathToURI(otherPath)]) != 0 {
		t.Fatalf("field rename edits: %#v", edit)
	}
}

func TestWorkspaceRenameRejectsUnindexedClassUse(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic class Widget {}\n"), 0o644); err != nil {
		t.Fatalf("writing class declaration: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib use { Widget }\nfunc inspect(): void { let item = Widget }\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := protocol.Position{Line: 1, Character: uint32(strings.Index(strings.Split(content, "\n")[1], "Widget"))}
	if _, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "Gadget"); err == nil {
		t.Fatal("rename accepted a class use missing from the semantic index")
	}
}

func TestWorkspaceRenameClassInStaticCall(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	libContent := "package lib\npublic class Widget { public func number(): int32 { return 7 } }\n"
	if err := os.WriteFile(libPath, []byte(libContent), 0o644); err != nil {
		t.Fatalf("writing class declaration: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib\nfunc use(): int32 { return lib.Widget::number() }\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := sourcePosition(content, strings.Index(content, "Widget::number"))
	edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "Gadget")
	if err != nil || edit == nil || len(edit.Changes[pathToURI(libPath)]) != 1 || len(edit.Changes[uri]) != 1 {
		t.Fatalf("static call class rename edits: %#v, %v", edit, err)
	}
}

func TestWorkspaceRenameFieldKeepsQualifiedClassIdentity(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	for _, fixture := range []struct{ path, content string }{
		{firstPath, "package first\npublic class Widget { let value: int32 }\n"},
		{secondPath, "package second\npublic class Widget { let value: int32 }\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.content), 0o644); err != nil {
			t.Fatalf("writing module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport first\nimport second\nfunc use(a: first.Widget, b: second.Widget): int32 { return a.value + b.value }\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := sourcePosition(content, strings.Index(content, "a.value")+len("a."))
	edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "amount")
	if err != nil || edit == nil || len(edit.Changes[pathToURI(firstPath)]) != 1 || len(edit.Changes[uri]) != 1 || len(edit.Changes[pathToURI(secondPath)]) != 0 {
		t.Fatalf("qualified field rename edits: %#v, %v", edit, err)
	}
}
