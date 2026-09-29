package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func applyForeignAction(t *testing.T, action *protocol.CodeAction, sources map[string]string) map[string]string {
	t.Helper()
	result := make(map[string]string, len(sources))
	for path, content := range sources {
		result[path] = content
	}
	for uri, edits := range action.Edit.Changes {
		path := uriToPath(string(uri))
		content, ok := sources[path]
		if !ok {
			t.Fatalf("unexpected edit in %s", path)
		}
		sort.Slice(edits, func(i, j int) bool {
			return sourceOffset(content, edits[i].Range.Start) > sourceOffset(content, edits[j].Range.Start)
		})
		for _, edit := range edits {
			start := sourceOffset(content, edit.Range.Start)
			end := sourceOffset(content, edit.Range.End)
			content = content[:start] + edit.NewText + content[end:]
		}
		result[path] = content
	}
	return result
}

func TestForeignMigrationRewritesWorkspaceReferences(t *testing.T) {
	directory := t.TempDir()
	bridgePath := filepath.Join(directory, "bridge.gecko")
	mainPath := filepath.Join(directory, "main.gecko")
	bridge := "package bridge\n" +
		"declare external type Handle\n" +
		"declare external func acquire(\n    name: string,\n    handle: out Handle*\n): int32\n" +
		"declare external func puts(message: string): int32\n" +
		"func local(): int32 {\n    let handle: Handle* = null\n    return acquire(\"x\", out handle)\n}\n"
	main := "package main\nimport bridge use {acquire, puts}\n" +
		"external func main(): int32 {\n    let handle: bridge.Handle* = null\n    return acquire(\"x\", out handle) + puts(\"ok\")\n}\n"
	for path, content := range map[string]string{bridgePath: bridge, mainPath: main} {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("writing source: %v", err)
		}
	}
	s := NewServer()
	uri := pathToURI(bridgePath)
	s.documents.Open(uri, bridge, 1)
	doc, ok := s.documents.Get(uri)
	if !ok {
		t.Fatal("bridge document missing")
	}
	doc.RebuildAnalysis(map[string]string{bridgePath: bridge, mainPath: main})
	if doc.Analysis == nil {
		t.Fatal("bridge analysis missing")
	}
	diagnostics, err := RunCompilerCheck(string(uri), bridge)
	if err != nil {
		t.Fatalf("checking bridge: %v", err)
	}
	action, err := s.allForeignDeclarationsAction(context.Background(), doc, diagnostics)
	if err != nil {
		t.Fatalf("building migration: %v", err)
	}
	if action == nil {
		t.Fatal("file-wide migration missing")
	}
	direct, err := s.directForeignDeclarationActions(context.Background(), doc, diagnostics)
	if err != nil {
		t.Fatalf("building direct migration: %v", err)
	}
	directFound := false
	for _, candidate := range direct {
		if len(candidate.Diagnostics) != 1 || candidate.Diagnostics[0].Range.Start.Line != 2 {
			continue
		}
		converted := applyForeignAction(t, &candidate, map[string]string{bridgePath: bridge, mainPath: main})
		if !strings.Contains(converted[mainPath], "bridge.c_abi.acquire(\"x\", out handle)") || !strings.Contains(converted[bridgePath], "declare external func puts") {
			t.Fatalf("direct migration incorrect: %#v", converted)
		}
		checkedDirect, err := RunWorkspaceCheck(string(pathToURI(mainPath)), converted[mainPath], converted)
		if err != nil {
			t.Fatalf("checking direct migration: %v", err)
		}
		for _, fileDiagnostics := range checkedDirect {
			for _, diagnostic := range fileDiagnostics {
				if diagnostic.Severity == protocol.DiagnosticSeverityError {
					t.Fatalf("direct migration diagnostic: %#v", diagnostic)
				}
			}
		}
		directFound = true
	}
	if !directFound {
		t.Fatal("out declaration quick fix missing")
	}
	result := applyForeignAction(t, action, map[string]string{bridgePath: bridge, mainPath: main})
	if strings.Count(result[bridgePath], "foreign \"c\" c_abi {") != 1 || !strings.Contains(result[bridgePath], "func acquire(\n") || !strings.Contains(result[bridgePath], "c_abi.acquire(\"x\", out handle)") {
		t.Fatalf("bridge migration incorrect:\n%s", result[bridgePath])
	}
	if !strings.Contains(result[mainPath], "import bridge use {puts}") || !strings.Contains(result[mainPath], "bridge.c_abi.acquire(\"x\", out handle)") || !strings.Contains(result[mainPath], "puts(\"ok\")") {
		t.Fatalf("workspace migration incorrect:\n%s", result[mainPath])
	}
	checked, err := RunWorkspaceCheck(string(pathToURI(mainPath)), result[mainPath], result)
	if err != nil {
		t.Fatalf("checking migrated workspace: %v", err)
	}
	for _, fileDiagnostics := range checked {
		for _, diagnostic := range fileDiagnostics {
			if diagnostic.Severity == protocol.DiagnosticSeverityError || strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
				t.Fatalf("migrated workspace diagnostic: %#v", diagnostic)
			}
		}
	}
}

func TestMultilineDeclarationOffersSingleFix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\ndeclare external func puts(\n    message: string\n): int32\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking declaration: %v", err)
	}
	for _, action := range GetCodeActions(content, path, protocol.Range{}, diagnostics) {
		if action.Title != "Convert declaration to foreign C module" {
			continue
		}
		fixed := applySingleActionEdit(t, content, pathToURI(path), action)
		if !strings.Contains(fixed, "func puts(\n        message: string\n    ): int32") {
			t.Fatalf("multiline member incorrectly formatted: %s", fixed)
		}
		return
	}
	t.Fatal("multiline declaration action missing")
}

func TestForeignMigrationRewritesVariardicCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\ndeclare external variardic func printf(format: string): int32\nexternal func main(): int32 { return printf(\"ok\") }\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing source: %v", err)
	}
	s := NewServer()
	uri := pathToURI(path)
	s.documents.Open(uri, content, 1)
	doc, ok := s.documents.Get(uri)
	if !ok {
		t.Fatal("document missing")
	}
	doc.RebuildAnalysis(map[string]string{path: content})
	if doc.Analysis == nil {
		t.Fatal("analysis missing")
	}
	diagnostics, err := RunCompilerCheck(string(uri), content)
	if err != nil {
		t.Fatalf("checking declaration: %v", err)
	}
	action, err := s.allForeignDeclarationsAction(context.Background(), doc, diagnostics)
	if err != nil {
		t.Fatalf("building migration: %v", err)
	}
	if action == nil {
		t.Fatal("variadic migration missing")
	}
	after := applyForeignAction(t, action, map[string]string{path: content})[path]
	if !strings.Contains(after, "func printf(format: string, ...): int32") || !strings.Contains(after, "return c_abi.printf(\"ok\")") {
		t.Fatalf("variadic migration incorrect: %s", after)
	}
	checked, err := RunCompilerCheck(string(uri), after)
	if err != nil {
		t.Fatalf("checking migrated source: %v", err)
	}
	for _, diagnostic := range checked {
		if diagnostic.Severity == protocol.DiagnosticSeverityError || strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
			t.Fatalf("migrated source diagnostic: %#v", diagnostic)
		}
	}
}
