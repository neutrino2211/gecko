package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/analysis"
	"go.lsp.dev/protocol"
)

func TestTraitMemberCallUsesDeclarationIdentity(t *testing.T) {
	content := "package test\ntrait Reader { func read(self): int32 }\nfunc use(reader: Reader): int32 { return reader.read() }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing trait member call: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	occurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, call)
	if !ok {
		t.Fatal("trait member call has no semantic occurrence")
	}
	declaration := strings.Index(content, "func read(self)") + len("func ")
	declOccurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, declaration)
	if !ok || occurrence.SymbolID != declOccurrence.SymbolID {
		t.Fatalf("trait member call and declaration differ: %#v %#v", occurrence, declOccurrence)
	}
	position := sourcePosition(content, call)
	definition := semanticDefinition(ctx, path, content, position)
	if definition == nil || definition.Range.Start != (protocol.Position{Line: 1, Character: 20}) {
		t.Fatalf("trait member definition: %#v", definition)
	}
}

func TestClassMethodCallUsesDeclarationIdentity(t *testing.T) {
	content := "package test\nclass Box { func read(self): int32 { return 1 } }\nfunc use(value: Box): int32 { return value.read() }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing class method call: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	occurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, call)
	if !ok {
		t.Fatal("class method call has no semantic occurrence")
	}
	declaration := strings.Index(content, "func read(self)") + len("func ")
	declOccurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, declaration)
	if !ok || occurrence.SymbolID != declOccurrence.SymbolID {
		t.Fatalf("class method call and declaration differ: %#v %#v", occurrence, declOccurrence)
	}
}

func TestInheritedTraitMemberCallUsesParentDeclaration(t *testing.T) {
	content := "package test\ntrait Parent { func read(self): int32 }\ntrait Child: Parent {}\nfunc use(value: Child): int32 { return value.read() }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing inherited trait call: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, call))
	if definition == nil || definition.Range.Start.Line != 1 || definition.Range.Start.Character != 20 {
		t.Fatalf("inherited trait method definition: %#v", definition)
	}
}

func TestImportedTraitMemberReferencesAcrossFiles(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic trait Reader { func read(self): int32 }\n"), 0o644); err != nil {
		t.Fatalf("writing trait module: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib use { Reader }\nfunc use(reader: Reader): int32 { return reader.read() }\n"
	server := NewServer()
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := sourcePosition(content, strings.LastIndex(content, "read()"))
	locations, _, _, err := server.workspaceReferences(context.Background(), doc, position, true, false)
	if err != nil || len(locations) != 2 || locations[0].URI != pathToURI(libPath) || locations[1].URI != uri {
		t.Fatalf("imported trait method references: %#v, %v", locations, err)
	}
}

func TestQualifiedTraitMemberKeepsModuleIdentity(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	for _, fixture := range []struct{ path, source string }{
		{firstPath, "package first\npublic trait Reader { func read(self): int32 }\n"},
		{secondPath, "package second\npublic trait Reader { func read(self): bool }\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing trait module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport first\nimport second\nfunc use(reader: first.Reader): int32 { return reader.read() }\n"
	ctx, err := analysis.NewAnalysisContext(mainPath, content)
	if err != nil {
		t.Fatalf("analyzing qualified trait: %v", err)
	}
	definition := semanticDefinition(ctx, mainPath, content, sourcePosition(content, strings.LastIndex(content, "read()")))
	if definition == nil || definition.URI != pathToURI(firstPath) {
		t.Fatalf("qualified trait method definition: %#v", definition)
	}
}

func TestQualifiedClassMethodKeepsModuleIdentity(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	for _, fixture := range []struct{ path, source string }{
		{firstPath, "package first\npublic class Widget { public func value(self): int32 { return 1 } }\n"},
		{secondPath, "package second\npublic class Widget { public func value(self): int32 { return 2 } }\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing class module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport first\nimport second\nclass Holder { let first: first.Widget\nlet second: second.Widget }\nfunc use(a: first.Widget, b: second.Widget, h: Holder): int32 { return a.value() + b.value() + h.first.value() + h.second.value() }\n"
	ctx, err := analysis.NewAnalysisContext(mainPath, content)
	if err != nil {
		t.Fatalf("analyzing qualified class calls: %v", err)
	}
	for _, fixture := range []struct{ receiver, path string }{
		{"a", firstPath},
		{"b", secondPath},
	} {
		offset := strings.Index(content, fixture.receiver+".value()") + len(fixture.receiver) + 1
		definition := semanticDefinition(ctx, mainPath, content, sourcePosition(content, offset))
		if definition == nil || definition.URI != pathToURI(fixture.path) {
			t.Fatalf("%s.value method definition: %#v", fixture.receiver, definition)
		}
	}
	for _, fixture := range []struct{ receiver, path string }{
		{"h.first", firstPath},
		{"h.second", secondPath},
	} {
		offset := strings.Index(content, fixture.receiver+".value()") + len(fixture.receiver) + 1
		definition := semanticDefinition(ctx, mainPath, content, sourcePosition(content, offset))
		if definition == nil || definition.URI != pathToURI(fixture.path) {
			t.Fatalf("%s.value chained method definition: %#v", fixture.receiver, definition)
		}
	}
}

func TestChainedTraitMemberUsesDeclarationIdentity(t *testing.T) {
	content := "package test\ntrait Reader { func read(self): int32 }\nclass Holder { let reader: Reader }\nfunc use(holder: Holder): int32 { return holder.reader.read() }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing chained trait member: %v", err)
	}
	declaration := strings.Index(content, "func read(self)") + len("func ")
	call := strings.Index(content, "holder.reader.read()") + len("holder.reader.")
	declOccurrence, declOK := ctx.SemanticGraph.OccurrenceAt(path, declaration)
	callOccurrence, callOK := ctx.SemanticGraph.OccurrenceAt(path, call)
	if !declOK || !callOK || callOccurrence.SymbolID != declOccurrence.SymbolID {
		t.Fatalf("chained trait method identity: %#v %#v", callOccurrence, declOccurrence)
	}
}

func TestQualifiedChildTraitResolvesImportedParentMethod(t *testing.T) {
	root := t.TempDir()
	parentPath := filepath.Join(root, "parent.gecko")
	childPath := filepath.Join(root, "child.gecko")
	for _, fixture := range []struct{ path, source string }{
		{parentPath, "package parent\npublic trait Parent { func read(self): int32 }\n"},
		{childPath, "package child\nimport parent use { Parent }\npublic trait Child: Parent {}\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing trait module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport child\nfunc use(reader: child.Child): int32 { return reader.read() }\n"
	ctx, err := analysis.NewAnalysisContext(mainPath, content)
	if err != nil {
		t.Fatalf("analyzing inherited imported trait: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	definition := semanticDefinition(ctx, mainPath, content, sourcePosition(content, call))
	if definition == nil || definition.URI != pathToURI(parentPath) {
		t.Fatalf("inherited imported trait method: %#v", definition)
	}
}

func TestInheritedTraitMethodUsesSelectedImport(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	childPath := filepath.Join(root, "child.gecko")
	for _, fixture := range []struct{ path, source string }{
		{firstPath, "package first\npublic trait Parent { func read(self): int32 }\n"},
		{secondPath, "package second\npublic trait Parent { func read(self): bool }\n"},
		{childPath, "package child\nimport first use { Parent }\npublic trait Child: Parent {}\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing trait module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport child\nimport second\nfunc use(reader: child.Child): int32 { return reader.read() }\n"
	ctx, err := analysis.NewAnalysisContext(mainPath, content)
	if err != nil {
		t.Fatalf("analyzing selected parent import: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	definition := semanticDefinition(ctx, mainPath, content, sourcePosition(content, call))
	if definition == nil || definition.URI != pathToURI(firstPath) {
		t.Fatalf("selected parent method definition: %#v", definition)
	}
}

func TestUnqualifiedTraitMethodUsesExplicitImport(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	for _, fixture := range []struct{ path, source string }{
		{firstPath, "package first\npublic trait Reader { func read(self): int32 }\n"},
		{secondPath, "package second\npublic trait Reader { func read(self): bool }\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing trait module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport second\nimport first use { Reader }\nfunc use(reader: Reader): void { reader.read() }\n"
	ctx, err := analysis.NewAnalysisContext(mainPath, content)
	if err != nil {
		t.Fatalf("analyzing selected trait import: %v", err)
	}
	definition := semanticDefinition(ctx, mainPath, content, sourcePosition(content, strings.LastIndex(content, "read()")))
	if definition == nil || definition.URI != pathToURI(firstPath) {
		t.Fatalf("selected trait method definition: %#v", definition)
	}
}

func TestAmbiguousTraitImportsDoNotAssignMethodIdentity(t *testing.T) {
	root := t.TempDir()
	for _, fixture := range []struct{ name, source string }{
		{"first", "package first\npublic trait Reader { func read(self): int32 }\n"},
		{"second", "package second\npublic trait Reader { func read(self): int32 }\n"},
	} {
		if err := os.WriteFile(filepath.Join(root, fixture.name+".gecko"), []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing trait module: %v", err)
		}
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport first use { Reader }\nimport second use { Reader }\nfunc use(reader: Reader): void { reader.read() }\n"
	ctx, err := analysis.NewAnalysisContext(mainPath, content)
	if err != nil {
		t.Fatalf("analyzing ambiguous trait imports: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	if definition := semanticDefinition(ctx, mainPath, content, sourcePosition(content, call)); definition != nil {
		t.Fatalf("ambiguous trait method gained a definition: %#v", definition)
	}
}

func TestGenericTraitBoundMethodUsesTraitDeclaration(t *testing.T) {
	content := "package test\ntrait Reader { func read(self): int32 }\nfunc use<T is Reader>(value: T): int32 { return value.read() }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing generic trait method: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, call))
	if definition == nil || definition.Range.Start.Line != 1 || definition.Range.Start.Character != 20 {
		t.Fatalf("generic trait method definition: %#v", definition)
	}
}

func TestGenericWhereClauseMethodUsesTraitDeclaration(t *testing.T) {
	content := "package test\ntrait Reader { func read(self): int32 }\nfunc use<T>(value: T): int32 where T is Reader { return value.read() }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing where-clause trait method: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, call))
	if definition == nil || definition.Range.Start.Line != 1 || definition.Range.Start.Character != 20 {
		t.Fatalf("where-clause trait method definition: %#v", definition)
	}
}

func TestAmbiguousGenericTraitBoundsDoNotAssignMethodIdentity(t *testing.T) {
	content := "package test\ntrait First { func read(self): int32 }\ntrait Second { func read(self): int32 }\nfunc use<T is First & Second>(value: T): int32 { return value.read() }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing ambiguous trait bounds: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	if definition := semanticDefinition(ctx, path, content, sourcePosition(content, call)); definition != nil {
		t.Fatalf("ambiguous trait method gained a definition: %#v", definition)
	}
}

func TestGenericClassBoundMethodUsesTraitDeclaration(t *testing.T) {
	content := "package test\ntrait Reader { func read(self): int32 }\nclass Box<T is Reader> { func use(self, value: T): int32 { return value.read() } }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing class trait bound: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, call))
	if definition == nil || definition.Range.Start.Line != 1 || definition.Range.Start.Character != 20 {
		t.Fatalf("class trait bound method definition: %#v", definition)
	}
}

func TestGenericClassWhereClauseMethodUsesTraitDeclaration(t *testing.T) {
	content := "package test\ntrait Reader { func read(self): int32 }\nclass Box<T> where T is Reader { func use(self, value: T): int32 { return value.read() } }\n"
	path := filepath.Join(t.TempDir(), "test.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing class where clause: %v", err)
	}
	call := strings.LastIndex(content, "read()")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, call))
	if definition == nil || definition.Range.Start.Line != 1 || definition.Range.Start.Character != 20 {
		t.Fatalf("class where-clause method definition: %#v", definition)
	}
}
