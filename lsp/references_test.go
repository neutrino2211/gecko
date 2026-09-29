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

func TestLocalReferencesAndRenameRespectShadowing(t *testing.T) {
	content := "package test\nfunc main(): void {\n    let x: int32 = 1\n    let y: int32 = x\n    if true {\n        let x: int32 = 2\n        x = x + 1\n    }\n    let z: int32 = x\n    let s: string = \"x\" // x\n}\n"
	ctx, err := analysis.NewAnalysisContext("test.gecko", content)
	if err != nil {
		t.Fatalf("analyzing test source: %v", err)
	}
	outer := protocol.Position{Line: 3, Character: uint32(strings.LastIndex(strings.Split(content, "\n")[3], "x"))}
	inner := protocol.Position{Line: 6, Character: 8}
	outerRefs := localReferences(ctx, "test.gecko", content, outer, true)
	if len(outerRefs) != 3 {
		t.Fatalf("outer x references: got %d, want 3: %#v", len(outerRefs), outerRefs)
	}
	innerRefs := localReferences(ctx, "test.gecko", content, inner, true)
	if len(innerRefs) != 3 {
		t.Fatalf("inner x references: got %d, want 3: %#v", len(innerRefs), innerRefs)
	}
	for _, location := range outerRefs {
		if location.Range.Start.Line == 5 || location.Range.Start.Line == 6 || location.Range.Start.Line == 9 {
			t.Fatalf("outer reference includes unrelated x: %#v", location)
		}
	}
	edit, err := renameLocal(ctx, "test.gecko", content, outer, "total")
	if err != nil {
		t.Fatalf("renaming outer x: %v", err)
	}
	if edit == nil || len(edit.Changes[pathToURI("test.gecko")]) != 3 {
		t.Fatalf("rename edits: %#v", edit)
	}
	if _, err := renameLocal(ctx, "test.gecko", content, outer, "not-valid"); err == nil {
		t.Fatal("invalid identifier was accepted")
	}
	if _, err := renameLocal(ctx, "test.gecko", content, outer, "let"); err == nil {
		t.Fatal("reserved keyword was accepted")
	}
}

func TestLambdaParametersCapturesAndLocalsUseDeclarationIdentity(t *testing.T) {
	content := "package test\nfunc make(offset: int32): func(int32): int32 {\n    return fn(value: int32): int32 {\n        let result: int32 = value + offset\n        return result\n    }\n}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing lambda: %v", err)
	}
	for _, fixture := range []struct {
		declaration string
		use         string
	}{
		{"make(offset", "value + offset"},
		{"fn(value", "value + offset"},
		{"let result", "return result"},
	} {
		declaration := strings.Index(content, fixture.declaration) + strings.LastIndex(fixture.declaration, "offset")
		if fixture.declaration == "fn(value" {
			declaration = strings.Index(content, fixture.declaration) + len("fn(")
		} else if fixture.declaration == "let result" {
			declaration = strings.Index(content, fixture.declaration) + len("let ")
		}
		use := strings.Index(content, fixture.use)
		if fixture.declaration == "make(offset" {
			use += len("value + ")
		} else if fixture.declaration == "let result" {
			use += len("return ")
		}
		definition := semanticDefinition(ctx, path, content, sourcePosition(content, use))
		if definition == nil || definition.Range.Start != sourcePosition(content, declaration) {
			t.Fatalf("%s lambda definition: %#v", fixture.declaration, definition)
		}
		refs := localReferences(ctx, path, content, sourcePosition(content, declaration), true)
		if len(refs) != 2 {
			t.Fatalf("%s lambda references: %#v", fixture.declaration, refs)
		}
	}
}

func TestInferredLambdaParameterUsesDeclarationIdentity(t *testing.T) {
	content := "package test\nfunc make(offset: int32): func(int32): int32 {\n    let callback: func(int32): int32 = fn(value) { return value + offset }\n    return callback\n}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing inferred lambda: %v", err)
	}
	declaration := strings.Index(content, "fn(value)") + len("fn(")
	use := strings.Index(content, "value + offset")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, use))
	if definition == nil || definition.Range.Start != sourcePosition(content, declaration) {
		t.Fatalf("inferred lambda parameter definition: %#v", definition)
	}
	outerDeclaration := strings.Index(content, "make(offset") + len("make(")
	outerUse := use + len("value + ")
	definition = semanticDefinition(ctx, path, content, sourcePosition(content, outerUse))
	if definition == nil || definition.Range.Start != sourcePosition(content, outerDeclaration) {
		t.Fatalf("inferred lambda capture definition: %#v", definition)
	}
}

func TestLambdaParameterShadowsOuterParameter(t *testing.T) {
	content := "package test\nfunc make(value: int32): func(int32): int32 { return fn(value: int32): int32 { return value } }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing shadowed lambda parameter: %v", err)
	}
	outer := strings.Index(content, "make(value") + len("make(")
	inner := strings.Index(content, "fn(value") + len("fn(")
	use := strings.LastIndex(content, "return value") + len("return ")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, use))
	if definition == nil || definition.Range.Start != sourcePosition(content, inner) {
		t.Fatalf("shadowed lambda parameter definition: %#v", definition)
	}
	if refs := localReferences(ctx, path, content, sourcePosition(content, outer), true); len(refs) != 1 {
		t.Fatalf("outer parameter references include lambda uses: %#v", refs)
	}
}

func TestLambdaCaptureRenameRejectsParameterCollision(t *testing.T) {
	content := "package test\nfunc make(outer: int32): func(int32): int32 { return fn(inner: int32): int32 { return outer + inner } }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing lambda capture: %v", err)
	}
	outer := strings.Index(content, "make(outer") + len("make(")
	if _, err := renameLocal(ctx, path, content, sourcePosition(content, outer), "inner"); err == nil {
		t.Fatal("rename accepted a lambda parameter that would shadow the capture")
	}
}

func TestInferredLambdaReceiverLinksFieldDeclaration(t *testing.T) {
	content := "package test\nclass Box { let value: int32 }\nfunc make(): func(Box): int32 { let callback: func(Box): int32 = fn(item) { return item.value }\n    return callback }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing lambda field access: %v", err)
	}
	use := strings.Index(content, "item.value") + len("item.")
	declaration := strings.Index(content, "let value") + len("let ")
	definition := semanticDefinition(ctx, path, content, sourcePosition(content, use))
	if definition == nil || definition.Range.Start != sourcePosition(content, declaration) {
		t.Fatalf("lambda field definition: %#v", definition)
	}
}

func TestSemanticDefinitionUsesResolvedCall(t *testing.T) {
	content := "package test\nfunc add(a: int32): int32 { return a }\nfunc main(): void { let value: int32 = add(1) }\n"
	ctx, err := analysis.NewAnalysisContext("test.gecko", content)
	if err != nil {
		t.Fatalf("analyzing test source: %v", err)
	}
	call := protocol.Position{Line: 2, Character: uint32(strings.Index(strings.Split(content, "\n")[2], "add"))}
	location := semanticDefinition(ctx, "test.gecko", content, call)
	if location == nil || location.Range.Start.Line != 1 || location.Range.Start.Character != 5 || location.Range.End.Character != 8 {
		t.Fatalf("resolved call definition: %#v", location)
	}
}

func TestSemanticDefinitionUsesFieldIdentity(t *testing.T) {
	content := "package test\nclass A { let value: int32 }\nclass B { let value: int32 }\nfunc main(): void {\n    let b: B = B { value: 2 }\n    let x: int32 = b.value\n}\n"
	ctx, err := analysis.NewAnalysisContext("test.gecko", content)
	if err != nil {
		t.Fatalf("analyzing test source: %v", err)
	}
	position := protocol.Position{Line: 5, Character: uint32(strings.Index(strings.Split(content, "\n")[5], "value"))}
	location := semanticDefinition(ctx, "test.gecko", content, position)
	if location == nil || location.Range.Start.Line != 2 || location.Range.Start.Character != 14 || location.Range.End.Character != 19 {
		t.Fatalf("expected B field declaration, got %#v", location)
	}
}

func TestRenamePreservesIndexedClassField(t *testing.T) {
	content := "package test\nclass Holder { let value: int32 }\nfunc main(): void { let value: int32 = 1 }\n"
	ctx, err := analysis.NewAnalysisContext("test.gecko", content)
	if err != nil {
		t.Fatalf("analyzing test source: %v", err)
	}
	position := protocol.Position{Line: 2, Character: uint32(strings.LastIndex(strings.Split(content, "\n")[2], "value"))}
	edit, err := renameLocal(ctx, "test.gecko", content, position, "total")
	if err != nil || edit == nil {
		t.Fatalf("renaming local with same-named field: %#v, %v", edit, err)
	}
	changes := edit.Changes[pathToURI("test.gecko")]
	if len(changes) != 1 || changes[0].Range.Start.Line != 2 {
		t.Fatalf("rename touched the class field: %#v", changes)
	}
}

func TestWorkspaceReferencesAndRenameResolveImportedFunction(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	otherPath := filepath.Join(root, "other.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic func add(v: int32): int32 { return v }\n"), 0o644); err != nil {
		t.Fatalf("writing library: %v", err)
	}
	if err := os.WriteFile(otherPath, []byte("package other\nfunc add(v: int32): int32 { return v }\n"), 0o644); err != nil {
		t.Fatalf("writing unrelated module: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib\nfunc main(): void { let x: int32 = lib.add(1) }\n"
	uri := pathToURI(mainPath)
	server := NewServer()
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis did not resolve library")
	}
	position := protocol.Position{Line: 2, Character: uint32(strings.Index(strings.Split(content, "\n")[2], "add"))}
	locations, _, _, err := server.workspaceReferences(context.Background(), doc, position, true, false)
	if err != nil || len(locations) != 2 {
		t.Fatalf("workspace references: %#v, %v", locations, err)
	}
	if locations[0].URI != pathToURI(libPath) || locations[1].URI != uri {
		t.Fatalf("references mixed unrelated add declarations: %#v", locations)
	}
	edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "sum")
	if err != nil || edit == nil || len(edit.Changes[pathToURI(libPath)]) != 1 || len(edit.Changes[uri]) != 1 {
		t.Fatalf("cross-file rename: %#v, %v", edit, err)
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
	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(callCtx, server.Handle)
	client.Go(callCtx, jsonrpc2.MethodNotFoundHandler)
	identifier := protocol.TextDocumentIdentifier{URI: uri}
	var wireReferences []protocol.Location
	_, err = client.Call(callCtx, "textDocument/references", protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: identifier, Position: position},
		Context:                    protocol.ReferenceContext{IncludeDeclaration: true},
	}, &wireReferences)
	if err != nil || len(wireReferences) != 2 {
		t.Fatalf("protocol references: %#v, %v", wireReferences, err)
	}
	var wireEdit protocol.WorkspaceEdit
	_, err = client.Call(callCtx, "textDocument/rename", protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: identifier, Position: position},
		NewName:                    "sum",
	}, &wireEdit)
	if err != nil || len(wireEdit.Changes[pathToURI(libPath)]) != 1 || len(wireEdit.Changes[uri]) != 1 {
		t.Fatalf("protocol rename: %#v, %v", wireEdit, err)
	}
	brokenPath := filepath.Join(root, "broken.gecko")
	if err := os.WriteFile(brokenPath, []byte("package broken\nfunc add("), 0o644); err != nil {
		t.Fatalf("writing broken module: %v", err)
	}
	if _, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "sum"); err == nil {
		t.Fatal("rename accepted incomplete project index")
	}
}

func TestWorkspaceClassReferencesIncludeTypeAnnotations(t *testing.T) {
	root := t.TempDir()
	libPath := filepath.Join(root, "lib.gecko")
	if err := os.WriteFile(libPath, []byte("package lib\npublic class Widget { let value: int32 }\n"), 0o644); err != nil {
		t.Fatalf("writing library: %v", err)
	}
	mainPath := filepath.Join(root, "main.gecko")
	content := "package main\nimport lib\nlet item: lib.Widget\n"
	uri := pathToURI(mainPath)
	server := NewServer()
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis did not resolve library")
	}
	position := protocol.Position{Line: 2, Character: uint32(strings.Index(strings.Split(content, "\n")[2], "Widget"))}
	locations, _, _, err := server.workspaceReferences(context.Background(), doc, position, true, false)
	if err != nil || len(locations) != 2 {
		t.Fatalf("class references: %#v, %v", locations, err)
	}
	if locations[0].URI != pathToURI(libPath) || locations[1].URI != uri {
		t.Fatalf("class references crossed modules: %#v", locations)
	}
}

func TestWorkspaceReferencesAndRenameAcrossProjectRoots(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	stdlib := filepath.Join(root, "std")
	for _, directory := range []string{first, second, stdlib} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatalf("creating workspace directory: %v", err)
		}
	}
	t.Setenv("GECKO_HOME", root)
	declarationPath := filepath.Join(stdlib, "shared.gecko")
	otherPath := filepath.Join(second, "other.gecko")
	for _, fixture := range []struct{ path, content string }{
		{declarationPath, "package shared\npublic func add(value: int32): int32 { return value }\n"},
		{otherPath, "package other\nimport std.shared\nfunc use(): int32 { return shared.add(2) }\n"},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.content), 0o644); err != nil {
			t.Fatalf("writing workspace source: %v", err)
		}
	}
	mainPath := filepath.Join(first, "main.gecko")
	content := "package main\nimport std.shared\nfunc use(): int32 { return shared.add(1) }\n"
	server := NewServer()
	server.workspaceRoots = []string{first, second, stdlib}
	uri := pathToURI(mainPath)
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	doc, ok := server.documents.Get(uri)
	if !ok || doc.Analysis == nil {
		t.Fatal("main analysis missing")
	}
	position := sourcePosition(content, strings.Index(content, "shared.add")+len("shared."))
	locations, _, _, err := server.workspaceReferences(context.Background(), doc, position, true, false)
	if err != nil || len(locations) != 3 || locations[0].URI != pathToURI(mainPath) || locations[1].URI != pathToURI(otherPath) || locations[2].URI != pathToURI(declarationPath) {
		t.Fatalf("cross-project references: %#v, %v", locations, err)
	}
	edit, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "sum")
	if err != nil || edit == nil || len(edit.Changes[pathToURI(mainPath)]) != 1 || len(edit.Changes[pathToURI(otherPath)]) != 1 || len(edit.Changes[pathToURI(declarationPath)]) != 1 {
		t.Fatalf("cross-project rename: %#v, %v", edit, err)
	}
	server.workspaceRoots = []string{first, second}
	if _, err := server.renameWorkspaceSymbol(context.Background(), doc, position, "sum"); err == nil {
		t.Fatal("rename accepted a declaration outside workspace roots")
	}
}

func TestClassReferencesIncludeExplicitTypeArgumentsAndCasts(t *testing.T) {
	content := "package test\nclass Widget {}\nclass Holder<T> { let value: T }\nclass Factory { func echo<T>(self, value: T): T { return value } }\nfunc identity<T>(value: T): T { return value }\nfunc use(): void {\n    let item: Widget = Widget {}\n    let copy = identity<Widget>(item)\n    let cast = item as Widget\n    let holder = Holder<Widget>{value: item}\n    let factory = Factory {}\n    let echoed = factory.echo<Widget>(item)\n}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing type contexts: %v", err)
	}
	declaration := strings.Index(content, "class Widget") + len("class ")
	declOccurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, declaration)
	if !ok {
		t.Fatal("class declaration has no identity")
	}
	for _, fragment := range []string{"identity<Widget>", "as Widget", "Holder<Widget>", "echo<Widget>"} {
		offset := strings.Index(content, fragment) + strings.LastIndex(fragment, "Widget")
		occurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, offset)
		if !ok || occurrence.SymbolID != declOccurrence.SymbolID {
			t.Fatalf("type context %q has no class identity: %#v", fragment, occurrence)
		}
	}
}

func TestQualifiedFieldReferencesKeepClassIdentity(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.gecko")
	secondPath := filepath.Join(root, "second.gecko")
	firstContent := "package first\npublic class Widget { let value: int32 }\n"
	secondContent := "package second\npublic class Widget { let value: int32 }\n"
	for _, fixture := range []struct{ path, content string }{
		{firstPath, firstContent},
		{secondPath, secondContent},
	} {
		if err := os.WriteFile(fixture.path, []byte(fixture.content), 0o644); err != nil {
			t.Fatalf("writing module: %v", err)
		}
	}
	path := filepath.Join(root, "main.gecko")
	content := "package main\nimport first\nimport second\nfunc use(a: first.Widget, b: second.Widget): int32 {\n    a.value = 1\n    b.value = 2\n    return a.value + b.value\n}\n"
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing qualified field uses: %v", err)
	}
	for _, fixture := range []struct{ receiver, path, source string }{
		{"a", firstPath, firstContent},
		{"b", secondPath, secondContent},
	} {
		declarationOffset := strings.Index(fixture.source, "let value") + len("let ")
		declaration, declarationOK := ctx.SemanticGraph.OccurrenceAt(fixture.path, declarationOffset)
		if !declarationOK {
			t.Fatalf("missing %s.value declaration", fixture.receiver)
		}
		search := 0
		for useIndex := 0; useIndex < 2; useIndex++ {
			useOffset := strings.Index(content[search:], fixture.receiver+".value") + search + len(fixture.receiver) + 1
			use, useOK := ctx.SemanticGraph.OccurrenceAt(path, useOffset)
			if !useOK || use.SymbolID != declaration.SymbolID {
				t.Fatalf("%s.value use %d identity: %#v %#v", fixture.receiver, useIndex, use, declaration)
			}
			search = useOffset + len("value")
		}
	}
}

func TestDestructuredFieldLinksDeclaration(t *testing.T) {
	content := "package test\nclass Point { let x: int32 }\nfunc use(point: Point): int32 {\n    let Point { x = coordinate } = point\n    return coordinate\n}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing destructuring: %v", err)
	}
	fieldDeclaration := strings.Index(content, "let x") + len("let ")
	fieldUse := strings.Index(content, "{ x = coordinate") + len("{ ")
	declaration, declarationOK := ctx.SemanticGraph.OccurrenceAt(path, fieldDeclaration)
	use, useOK := ctx.SemanticGraph.OccurrenceAt(path, fieldUse)
	if !declarationOK || !useOK || use.SymbolID != declaration.SymbolID {
		t.Fatalf("destructured field identity: %#v %#v", use, declaration)
	}
	classDeclaration := strings.Index(content, "class Point") + len("class ")
	classUse := strings.Index(content, "let Point") + len("let ")
	classDecl, classDeclOK := ctx.SemanticGraph.OccurrenceAt(path, classDeclaration)
	classOccurrence, classUseOK := ctx.SemanticGraph.OccurrenceAt(path, classUse)
	if !classDeclOK || !classUseOK || classOccurrence.SymbolID != classDecl.SymbolID {
		t.Fatalf("destructured class identity: %#v %#v", classOccurrence, classDecl)
	}
}

func TestReferencesAndRenameProtocol(t *testing.T) {
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
	server.conn = jsonrpc2.NewConn(jsonrpc2.NewStream(serverSide))
	client := jsonrpc2.NewConn(jsonrpc2.NewStream(clientSide))
	server.conn.Go(ctx, server.Handle)
	client.Go(ctx, jsonrpc2.MethodNotFoundHandler)
	content := "package test\nfunc main(): void {\n    let value: int32 = 1\n    value = value + 1\n}\n"
	uri := pathToURI(t.TempDir() + "/main.gecko")
	server.documents.Open(uri, content, 1)
	server.rebuildAnalysis(uri)
	position := protocol.Position{Line: 3, Character: 4}
	identifier := protocol.TextDocumentIdentifier{URI: uri}
	var references []protocol.Location
	_, err := client.Call(ctx, "textDocument/references", protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: identifier, Position: position},
		Context:                    protocol.ReferenceContext{IncludeDeclaration: false},
	}, &references)
	if err != nil || len(references) != 2 {
		t.Fatalf("references: %d, %v", len(references), err)
	}
	var span protocol.Range
	if _, err := client.Call(ctx, "textDocument/prepareRename", protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: identifier, Position: position},
	}, &span); err != nil || span.Start.Line != 3 || span.End.Character != 9 {
		t.Fatalf("prepare rename: %#v, %v", span, err)
	}
	var edit protocol.WorkspaceEdit
	if _, err := client.Call(ctx, "textDocument/rename", protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: identifier, Position: position},
		NewName:                    "total",
	}, &edit); err != nil || len(edit.Changes[uri]) != 3 {
		t.Fatalf("rename: %#v, %v", edit, err)
	}
}
