package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/analysis"
)

func TestModuleQualifierLinksTypeAndCallToImport(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic class Widget {}\npublic func add(value: int32): int32 { return value }\n"), 0o644); err != nil {
		t.Fatalf("writing module: %v", err)
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib as library\nfunc use(item: library.Widget): int32 { return library.add(1) }\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing module qualifiers: %v", err)
	}
	positions := []int{
		strings.Index(content, "as library") + len("as "),
		strings.Index(content, "library.Widget"),
		strings.Index(content, "library.add"),
	}
	var declarationID int64
	for index, offset := range positions {
		occurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, offset)
		if !ok {
			t.Fatalf("missing module qualifier occurrence %d", index)
		}
		if index == 0 {
			declarationID = occurrence.SymbolID
			if !occurrence.Declaration {
				t.Fatal("import alias is not a declaration")
			}
		} else if occurrence.SymbolID != declarationID {
			t.Fatalf("module qualifier %d has a different identity: %#v", index, occurrence)
		}
	}
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, positions[2]))
	if definition == nil || definition.Range.Start != sourcePosition(content, positions[0]) {
		t.Fatalf("module qualifier definition: %#v", definition)
	}
	data := semanticTokens(&Document{URI: pathToURI(path), Content: content, Analysis: ctx}).Data
	namespaces := 0
	for index := 0; index+5 <= len(data); index += 5 {
		if data[index+3] == 6 {
			namespaces++
		}
	}
	if namespaces != len(positions) {
		t.Fatalf("namespace tokens = %d, want %d: %#v", namespaces, len(positions), data)
	}
}

func TestStaticCallLinksClassAndModuleQualifiers(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	libContent := "package lib\npublic class Widget { public func make(): int32 { return 7 } }\n"
	if err := os.WriteFile(libPath, []byte(libContent), 0o644); err != nil {
		t.Fatalf("writing module: %v", err)
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib as library\nfunc use(): int32 { return library.Widget::make() }\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing static call: %v", err)
	}
	aliasOffset := strings.Index(content, "as library") + len("as ")
	callOffset := strings.Index(content, "library.Widget::make")
	alias, aliasOK := ctx.SemanticGraph.OccurrenceAt(path, aliasOffset)
	qualifier, qualifierOK := ctx.SemanticGraph.OccurrenceAt(path, callOffset)
	if !aliasOK || !qualifierOK || alias.SymbolID != qualifier.SymbolID {
		t.Fatalf("static call module qualifier identity: %#v %#v", alias, qualifier)
	}
	classOffset := callOffset + len("library.")
	classUse, classOK := ctx.SemanticGraph.OccurrenceAt(path, classOffset)
	classDeclaration, declarationOK := ctx.SemanticGraph.OccurrenceAt(libPath, strings.Index(libContent, "class Widget")+len("class "))
	if !classOK || !declarationOK || classUse.SymbolID != classDeclaration.SymbolID {
		t.Fatalf("static call class identity: %#v %#v", classUse, classDeclaration)
	}
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, classOffset))
	if definition == nil || definition.URI != pathToURI(libPath) {
		t.Fatalf("static call class definition: %#v", definition)
	}
}

func TestQualifiedClassesWithSameNameKeepSeparateIdentities(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	firstContent := "package first\npublic class Widget {}\n"
	secondContent := "package second\npublic class Widget {}\n"
	for _, fixture := range []struct{ path, content string }{
		{firstPath, firstContent},
		{secondPath, secondContent},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.content), 0o644); err != nil {
			t.Fatalf("writing module: %v", err)
		}
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport first\nimport second\nfunc use(a: first.Widget, b: second.Widget): void {}\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing qualified classes: %v", err)
	}
	for _, fixture := range []struct{ name, path, source string }{
		{"first", firstPath, firstContent},
		{"second", secondPath, secondContent},
	} {
		useOffset := strings.Index(content, fixture.name+".Widget") + len(fixture.name) + 1
		declarationOffset := strings.Index(fixture.source, "class Widget") + len("class ")
		use, useOK := ctx.SemanticGraph.OccurrenceAt(path, useOffset)
		declaration, declarationOK := ctx.SemanticGraph.OccurrenceAt(fixture.path, declarationOffset)
		if !useOK || !declarationOK || use.SymbolID != declaration.SymbolID {
			t.Fatalf("%s.Widget identity: %#v %#v", fixture.name, use, declaration)
		}
	}
}

func TestUnqualifiedStaticCallUsesExplicitClassImport(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	for _, fixture := range []struct{ path, source string }{
		{firstPath, "package first\npublic class Widget { public func make(): int32 { return 1 } }\n"},
		{secondPath, "package second\npublic class Widget { public func make(): int32 { return 2 } }\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.source), 0o644); err != nil {
			t.Fatalf("writing class module: %v", err)
		}
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport second\nimport first use { Widget }\nfunc use(): int32 { return Widget::make() }\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing selected class import: %v", err)
	}
	call := strings.Index(content, "Widget::make()") + len("Widget::")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, call))
	if definition == nil || definition.URI != pathToURI(firstPath) {
		t.Fatalf("selected static method definition: %#v", definition)
	}
}

func TestAmbiguousClassImportsDoNotAssignStaticMethodIdentity(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second"} {
		source := "package " + name + "\npublic class Widget { public func make(): int32 { return 1 } }\n"
		if err := os.WriteFile(filepath.Join(root, name+".gecko"), []byte(source), 0o644); err != nil {
			t.Fatalf("writing class module: %v", err)
		}
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport first use { Widget }\nimport second use { Widget }\nfunc use(): int32 { return Widget::make() }\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing ambiguous class imports: %v", err)
	}
	call := strings.Index(content, "Widget::make()") + len("Widget::")
	if definition := semanticDefinition(ctx, path, content, sourcePosition(content, call)); definition != nil {
		t.Fatalf("ambiguous static method gained a definition: %#v", definition)
	}
}

func TestLambdaTypeAnnotationsLinkImportedClass(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	libContent := "package lib\npublic class Widget {}\n"
	if err := os.WriteFile(libPath, []byte(libContent), 0o644); err != nil {
		t.Fatalf("writing class module: %v", err)
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib as library\nfunc use(): void { let callback: func(library.Widget): library.Widget = fn(item: library.Widget): library.Widget { return item } }\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing lambda annotations: %v", err)
	}
	declarationOffset := strings.Index(libContent, "class Widget") + len("class ")
	declaration, ok := ctx.SemanticGraph.OccurrenceAt(libPath, declarationOffset)
	if !ok {
		t.Fatal("imported class declaration missing")
	}
	start := strings.Index(content, "fn(item: ") + len("fn(item: ")
	for index := 0; index < 2; index++ {
		offset := strings.Index(content[start:], "library.Widget") + start
		classUse, found := ctx.SemanticGraph.OccurrenceAt(path, offset+len("library."))
		if !found || classUse.SymbolID != declaration.SymbolID {
			t.Fatalf("lambda annotation %d class identity: %#v", index, classUse)
		}
		start = offset + len("library.Widget")
	}
}
