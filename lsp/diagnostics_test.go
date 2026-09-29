// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/compiler"
	"go.lsp.dev/protocol"
)

func TestDiagnosticsInheritedTraitMissingParent(t *testing.T) {
	content := `package test

trait Child: MissingParent {
    func value(self): int32
}
`

	diagnostics, err := RunCompilerCheck("file:///tmp/missing_parent.gecko", content)
	if err != nil {
		t.Fatalf("RunCompilerCheck failed: %v", err)
	}

	if !hasDiagnosticContaining(diagnostics, "Could not resolve parent trait") {
		t.Fatalf("Expected inherited-trait diagnostic mentioning unresolved parent, got: %#v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "Could not resolve parent trait") {
			if diagnostic.Range.Start.Line != 2 || diagnostic.Range.End.Character-diagnostic.Range.Start.Character != 5 {
				t.Fatalf("expected trait token range, got %#v", diagnostic.Range)
			}
		}
	}
}

func TestDiagnosticsInheritedTraitOverrideConflict(t *testing.T) {
	content := `package test

trait Parent {
    func value(self): int32
}

trait Child: Parent {
    func value(self): bool
}
`

	diagnostics, err := RunCompilerCheck("file:///tmp/override_conflict.gecko", content)
	if err != nil {
		t.Fatalf("RunCompilerCheck failed: %v", err)
	}

	if !hasDiagnosticContaining(diagnostics, "conflicts with inherited method") {
		t.Fatalf("Expected inherited-trait override conflict diagnostic, got: %#v", diagnostics)
	}

	if !hasDiagnosticContaining(diagnostics, "Parent.value") {
		t.Fatalf("Expected override conflict diagnostic to include parent method origin, got: %#v", diagnostics)
	}
}

func TestTraitOverrideDiagnosticCarriesSpanRelatedMethodAndFix(t *testing.T) {
	content := "package test\ntrait Parent { func value(self): int32 }\ntrait Child: Parent { func value(self): bool }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	uri := pathToURI(path)
	diagnostics, err := RunCompilerCheck(string(uri), content)
	if err != nil {
		t.Fatalf("checking conflicting trait: %v", err)
	}
	var diagnostic *protocol.Diagnostic
	for index := range diagnostics {
		if strings.Contains(diagnostics[index].Message, "conflicts with inherited method") {
			diagnostic = &diagnostics[index]
			break
		}
	}
	if diagnostic == nil {
		t.Fatalf("missing override conflict: %#v", diagnostics)
	}
	start := sourceOffset(content, diagnostic.Range.Start)
	end := sourceOffset(content, diagnostic.Range.End)
	if content[start:end] != "value" {
		t.Fatalf("diagnostic does not select the method name: %#v", diagnostic.Range)
	}
	if len(diagnostic.RelatedInformation) != 1 || diagnostic.RelatedInformation[0].Location.URI != uri {
		t.Fatalf("missing inherited method location: %#v", diagnostic.RelatedInformation)
	}
	related := diagnostic.RelatedInformation[0].Location.Range
	if related.Start.Line != 1 || content[sourceOffset(content, related.Start):sourceOffset(content, related.End)] != "func" {
		t.Fatalf("incorrect inherited method range: %#v", related)
	}
	raw, err := json.Marshal(diagnostic)
	if err != nil {
		t.Fatalf("serializing diagnostic: %v", err)
	}
	var echoed protocol.Diagnostic
	if err := json.Unmarshal(raw, &echoed); err != nil {
		t.Fatalf("reading diagnostic from client: %v", err)
	}
	actions := GetCodeActions(content, path, echoed.Range, []protocol.Diagnostic{echoed})
	if len(actions) != 1 || actions[0].Title != "Change return type to int32" {
		t.Fatalf("missing explicit return type fix: %#v", actions)
	}
	edits := actions[0].Edit.Changes[uri]
	if len(edits) != 1 || content[sourceOffset(content, edits[0].Range.Start):sourceOffset(content, edits[0].Range.End)] != "bool" {
		t.Fatalf("incorrect return type edit: %#v", edits)
	}
	fixStart := sourceOffset(content, edits[0].Range.Start)
	fixEnd := sourceOffset(content, edits[0].Range.End)
	fixed := content[:fixStart] + edits[0].NewText + content[fixEnd:]
	fixedDiagnostics, err := RunCompilerCheck(string(uri), fixed)
	if err != nil || hasDiagnosticContaining(fixedDiagnostics, "conflicts with inherited method") {
		t.Fatalf("suggested fix did not resolve conflict: %#v, %v", fixedDiagnostics, err)
	}
}

func hasDiagnosticContaining(diagnostics []protocol.Diagnostic, substr string) bool {
	for _, diag := range diagnostics {
		if strings.Contains(diag.Message, substr) {
			return true
		}
	}
	return false
}

func TestWorkspaceCheckUsesOpenImportedBuffer(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	modulePath := filepath.Join(root, "dependency.gecko")
	mainContent := "package main\nimport dependency\nfunc main(): void {}\n"
	moduleContent := "package dependency\ntrait Child: MissingParent {\n    func value(self): int32\n}\n"
	results, err := RunWorkspaceCheck(string(pathToURI(mainPath)), mainContent, map[string]string{modulePath: moduleContent})
	if err != nil {
		t.Fatalf("checking open module: %v", err)
	}
	if !hasDiagnosticContaining(results[pathToURI(modulePath)], "Could not resolve parent trait") {
		t.Fatalf("expected error on imported buffer, got: %#v", results)
	}
	if hasDiagnosticContaining(results[pathToURI(mainPath)], "Could not resolve parent trait") {
		t.Fatalf("imported error attributed to main file: %#v", results)
	}
}

func TestWorkspaceCheckResolvesProjectRootImport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gecko.toml"), []byte("[build]\nbackend = \"c\"\n"), 0o644); err != nil {
		t.Fatalf("writing project config: %v", err)
	}
	mainPath := filepath.Join(root, "src", "main.gecko")
	modulePath := filepath.Join(root, "dependency.gecko")
	mainContent := "package main\nimport dependency\nfunc main(): void {}\n"
	moduleContent := "package dependency\ntrait Child: MissingParent {\n    func value(self): int32\n}\n"
	results, err := RunWorkspaceCheck(string(pathToURI(mainPath)), mainContent, map[string]string{modulePath: moduleContent})
	if err != nil {
		t.Fatalf("checking project import: %v", err)
	}
	if !hasDiagnosticContaining(results[pathToURI(modulePath)], "Could not resolve parent trait") {
		t.Fatalf("expected project root import diagnostic, got %#v", results)
	}
}

func TestWorkspaceCheckReportsUnavailableProjectBackend(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gecko.toml"), []byte("[build]\nbackend = \"asm\"\n"), 0o644); err != nil {
		t.Fatalf("writing project config: %v", err)
	}
	path := filepath.Join(root, "main.gecko")
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), "package main\n")
	if err != nil {
		t.Fatalf("checking ASM project: %v", err)
	}
	if !hasDiagnosticContaining(diagnostics, "ASM backend compilation is not yet implemented") {
		t.Fatalf("missing backend diagnostic: %#v", diagnostics)
	}
}

func TestDiagnosticRangeClampsAndEncodesUTF16(t *testing.T) {
	rng := diagnosticRange("a😀b\n", 1, 6)
	if rng.Start.Character != 3 || rng.End.Character != 4 {
		t.Fatalf("expected UTF-16 range 3..4, got %#v", rng)
	}
	rng = diagnosticRange("", 0, 0)
	if rng.Start.Line != 0 || rng.Start.Character != 0 || rng.End.Character != 0 {
		t.Fatalf("expected clamped empty range, got %#v", rng)
	}
}

func TestCompilerDiagnosticRangeCoversToken(t *testing.T) {
	content := "😀 let value = 1\n"
	start := strings.Index(content, "value")
	rng := compilerDiagnosticRange(content, compiler.DiagnosticMessage{Line: 1, Column: start + 1, Offset: start})
	if rng.Start.Character != 7 || rng.End.Character != 12 {
		t.Fatalf("expected full identifier in UTF-16, got %#v", rng)
	}
	rng = compilerDiagnosticRange(content, compiler.DiagnosticMessage{Line: 2, Column: 1, Offset: start})
	if rng.Start.Line != 1 || rng.End.Line != 1 {
		t.Fatalf("expected fallback to reported position, got %#v", rng)
	}
}

func TestFileURIEncoding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "space #.gecko")
	uri := pathToURI(path)
	if uriToPath(string(uri)) != path {
		t.Fatalf("file URI did not round trip: %s", uri)
	}
}

func TestDocumentStoreVersionsAndOpenImports(t *testing.T) {
	root := t.TempDir()
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	moduleURI := pathToURI(filepath.Join(root, "dependency.gecko"))
	store := NewDocumentStore()
	store.Open(mainURI, "package main\nimport dependency\n", 7)
	store.Open(moduleURI, "package dependency\npublic class NewType {}\n", 3)
	if store.Update(mainURI, "package stale\n", 6) {
		t.Fatal("accepted older document version")
	}
	doc, ok := store.Get(mainURI)
	if !ok || doc.Version != 7 {
		t.Fatalf("lost client version: %#v", doc)
	}
	doc.RebuildAnalysis(store.Snapshot())
	if doc.Analysis == nil || doc.Analysis.ImportedFiles["dependency"] == nil {
		t.Fatal("analysis did not read the open imported buffer")
	}
	if !store.SetAnalysis(doc) {
		t.Fatal("failed to commit current analysis")
	}
	if !store.Update(mainURI, "", 8) {
		t.Fatal("rejected newer empty document")
	}
	if store.SetAnalysis(doc) {
		t.Fatal("committed stale analysis")
	}
	updated, ok := store.Get(mainURI)
	if !ok || updated.Content != "" || updated.Version != 8 {
		t.Fatalf("incorrect empty document state: %#v", updated)
	}
}

func TestDocumentKeepsValidPrefixDuringIncompleteEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	doc := &Document{URI: pathToURI(path), Content: "package main\nfunc ready(): int32 { return 7 }\nfunc unfinished(\n"}
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil || len(doc.Analysis.SemanticGraph.FunctionsNamed("ready")) != 1 {
		t.Fatalf("lost completed declaration while editing: %#v", doc.Analysis)
	}
	if len(doc.Analysis.SemanticGraph.FunctionsNamed("unfinished")) != 0 {
		t.Fatal("incomplete declaration entered the semantic graph")
	}
	doc.Content = "package main\nfunc ready(): int32 { return 7 }\nfunc unfinished(): void {}\n"
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil || len(doc.Analysis.SemanticGraph.FunctionsNamed("unfinished")) != 1 {
		t.Fatal("analysis did not recover after completing the declaration")
	}
}

func TestDocumentRecoversDeclarationsAfterMiddleSyntaxError(t *testing.T) {
	content := "package main\nfunc before(): void {}\nfunc broken(): void { let value: = 1 }\nfunc after(): void {}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	doc := &Document{URI: pathToURI(path), Content: content}
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil || len(doc.Analysis.SemanticGraph.FunctionsNamed("before")) != 1 || len(doc.Analysis.SemanticGraph.FunctionsNamed("after")) != 1 {
		t.Fatalf("lost declaration after syntax error: %#v", doc.Analysis)
	}
	position := protocol.Position{Line: 3, Character: 5}
	definition := semanticDefinition(doc.Analysis, path, content, position)
	if definition == nil || definition.Range.Start != position {
		t.Fatalf("later declaration moved during recovery: %#v", definition)
	}
}

func TestDocumentRecoversPartialFunctionBody(t *testing.T) {
	content := "package main\nfunc partial(): void {\n    let kept: int32 = 1\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	doc := &Document{URI: pathToURI(path), Content: content}
	doc.RebuildAnalysis(nil)
	if doc.Analysis == nil || len(doc.Analysis.SemanticGraph.FunctionsNamed("partial")) != 1 {
		t.Fatalf("lost partial function declaration: %#v", doc.Analysis)
	}
	offset := strings.Index(content, "kept")
	if _, ok := doc.Analysis.SemanticGraph.OccurrenceAt(path, offset); !ok {
		t.Fatal("lost completed local binding in partial body")
	}
}

func TestDocumentStoreFindsNewlyOpenedDependency(t *testing.T) {
	root := t.TempDir()
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	moduleURI := pathToURI(filepath.Join(root, "other.gecko"))
	store := NewDocumentStore()
	store.Open(mainURI, "package main\nimport other\n", 1)
	main, ok := store.Get(mainURI)
	if !ok {
		t.Fatal("main document not open")
	}
	main.RebuildAnalysis(store.Snapshot())
	if !store.SetAnalysis(main) {
		t.Fatal("failed to commit main analysis")
	}
	store.Open(moduleURI, "package other\n", 1)
	dependents := store.Dependents(moduleURI)
	if len(dependents) != 1 || dependents[0] != mainURI {
		t.Fatalf("expected dependent main document, got %#v", dependents)
	}
}

func TestDocumentStoreFindsTransitiveDependency(t *testing.T) {
	root := t.TempDir()
	mainURI := pathToURI(filepath.Join(root, "main.gecko"))
	middlePath := filepath.Join(root, "middle.gecko")
	leafURI := pathToURI(filepath.Join(root, "leaf.gecko"))
	unrelatedURI := pathToURI(filepath.Join(root, "unrelated.gecko"))
	if err := os.WriteFile(middlePath, []byte("package middle\nimport leaf\n"), 0o644); err != nil {
		t.Fatalf("writing intermediate module: %v", err)
	}
	store := NewDocumentStore()
	store.Open(mainURI, "package main\nimport middle\n", 1)
	store.Open(unrelatedURI, "package unrelated\n", 1)
	for _, uri := range []protocol.DocumentURI{mainURI, unrelatedURI} {
		doc, ok := store.Get(uri)
		if !ok {
			t.Fatalf("missing open document %s", uri)
		}
		doc.RebuildAnalysis(store.Snapshot())
		if !store.SetAnalysis(doc) {
			t.Fatalf("setting analysis for %s", uri)
		}
	}
	store.Open(leafURI, "package leaf\n", 1)
	dependents := store.Dependents(leafURI)
	if len(dependents) != 1 || dependents[0] != mainURI {
		t.Fatalf("expected only transitive importer, got %#v", dependents)
	}
}

func TestDefinitionInOpenImportedModule(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	modulePath := filepath.Join(root, "other.gecko")
	mainContent := "package main\nimport other\nlet item: other.Widget\n"
	moduleContent := "package other\npublic class Widget {}\n"
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, mainContent, func(path string) ([]byte, error) {
		if path == modulePath {
			return []byte(moduleContent), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("building analysis: %v", err)
	}
	col := strings.Index(strings.Split(mainContent, "\n")[2], "Widget")
	location := GetDefinitionLocation(ctx, mainContent, 2, col, string(pathToURI(mainPath)))
	if location == nil || location.URI != pathToURI(modulePath) || location.Range.Start.Line != 1 {
		t.Fatalf("expected imported Widget definition, got %#v", location)
	}
}

func TestDefinitionRespectsLocalShadowing(t *testing.T) {
	content := "package test\nfunc main(): void {\n    let x: int32 = 1\n    if true {\n        let x: int32 = 2\n        let y: int32 = x\n    }\n    let z: int32 = x\n}\n"
	ctx, err := analysis.NewAnalysisContext("test.gecko", content)
	if err != nil {
		t.Fatalf("building analysis: %v", err)
	}
	inner := GetDefinitionLocation(ctx, content, 5, 23, "file:///tmp/test.gecko")
	outer := GetDefinitionLocation(ctx, content, 7, 19, "file:///tmp/test.gecko")
	if inner == nil || inner.Range.Start.Line != 4 {
		t.Fatalf("expected inner declaration, got %#v", inner)
	}
	if outer == nil || outer.Range.Start.Line != 2 {
		t.Fatalf("expected outer declaration, got %#v", outer)
	}
}

func TestDefinitionUsesReceiverType(t *testing.T) {
	content := "package test\nclass A {\n    let value: int32\n}\nclass B {\n    let value: int32\n}\nfunc main(): void {\n    let b: B = B { value: 2 }\n    let n: int32 = b.value\n}\n"
	ctx, err := analysis.NewAnalysisContext("test.gecko", content)
	if err != nil {
		t.Fatalf("building analysis: %v", err)
	}
	col := strings.Index(strings.Split(content, "\n")[9], "value")
	location := GetDefinitionLocation(ctx, content, 9, col, "file:///tmp/test.gecko")
	if location == nil || location.Range.Start.Line != 5 {
		t.Fatalf("expected B.value, got %#v", location)
	}
}

func TestCompletionUsesOpenImportedModule(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.gecko")
	modulePath := filepath.Join(root, "other.gecko")
	base := "package main\nimport other\nfunc main(): void {}\n"
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, base, func(path string) ([]byte, error) {
		if path == modulePath {
			return []byte("package other\npublic func fresh(): void {}\n"), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("building analysis: %v", err)
	}
	content := "package main\nimport other\nfunc main(): void {\n    other.fr\n}\n"
	items := GetCompletions(ctx, content, mainPath, 3, len("    other.fr"))
	for _, item := range items {
		if item.Label == "fresh" {
			return
		}
	}
	t.Fatalf("missing completion from open imported module: %#v", items)
}

func TestDiagnosticsCoverUnsafeAndBorrowRules(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("finding repository root: %v", err)
	}
	t.Setenv("GECKO_HOME", root)
	fixtures := []struct {
		path    string
		message string
	}{
		{"unsafe_alloc_context_error", "unsafe"},
		{"borrow_move_error", "move"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.path, func(t *testing.T) {
			path := filepath.Join(root, "test_sources", "compile_tests", fixture.path, "main.gecko")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			diagnostics, err := RunCompilerCheck(string(pathToURI(path)), string(content))
			if err != nil {
				t.Fatalf("checking fixture: %v", err)
			}
			found := false
			for _, diagnostic := range diagnostics {
				if strings.Contains(strings.ToLower(diagnostic.Message), fixture.message) {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing %s diagnostic: %#v", fixture.message, diagnostics)
			}
		})
	}
}

func TestEveryLegacyDeclarationHasDiagnostic(t *testing.T) {
	path, err := filepath.Abs("../test_sources/compile_tests/deprecation_diagnostics/main.gecko")
	if err != nil {
		t.Fatalf("finding legacy source: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading legacy source: %v", err)
	}
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), string(content))
	if err != nil {
		t.Fatalf("checking legacy source: %v", err)
	}
	var lines []uint32
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
			lines = append(lines, diagnostic.Range.Start.Line)
		}
	}
	if len(lines) != 2 || lines[0] != 2 || lines[1] != 3 {
		t.Fatalf("expected one warning per declaration, got lines %v in %#v", lines, diagnostics)
	}
	clean := "package main\nfunc main(): int32 { return 0 }\n"
	cleanDiagnostics, err := RunCompilerCheck(string(pathToURI(path)), clean)
	if err != nil || hasDiagnosticContaining(cleanDiagnostics, "`declare external` is deprecated") {
		t.Fatalf("legacy warning remained after replacement: %#v, %v", cleanDiagnostics, err)
	}
}
