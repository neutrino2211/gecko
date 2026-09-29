package main

import (
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func applySingleActionEdit(t *testing.T, content string, uri protocol.DocumentURI, action protocol.CodeAction) string {
	t.Helper()
	if action.Edit == nil || len(action.Edit.Changes[uri]) != 1 {
		t.Fatalf("expected one edit in action: %#v", action)
	}
	edit := action.Edit.Changes[uri][0]
	start := sourceOffset(content, edit.Range.Start)
	end := sourceOffset(content, edit.Range.End)
	if start < 0 || end < start || end > len(content) {
		t.Fatalf("invalid edit range: %#v", edit.Range)
	}
	return content[:start] + edit.NewText + content[end:]
}

func TestUnsafeOperationCodeActionWrapsStandaloneStatement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nexternal func main(): int32 {\n    @trap()\n    return 0\n}\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking unsafe operation: %v", err)
	}
	var unsafeDiagnostic protocol.Diagnostic
	found := false
	for _, diagnostic := range diagnostics {
		if strings.HasPrefix(diagnostic.Message, "Unsafe Required:") {
			unsafeDiagnostic = diagnostic
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unsafe diagnostic missing: %#v", diagnostics)
	}
	if unsafeDiagnostic.Range.Start.Line != 2 || unsafeDiagnostic.Range.Start.Character != 4 || unsafeDiagnostic.Range.End.Line != 2 || unsafeDiagnostic.Range.End.Character != 9 {
		t.Fatalf("unsafe diagnostic range = %#v, want @trap", unsafeDiagnostic.Range)
	}
	for _, action := range GetCodeActions(content, path, unsafeDiagnostic.Range, []protocol.Diagnostic{unsafeDiagnostic}) {
		if action.Title != "Wrap operation in @unsafe" {
			continue
		}
		fixed := applySingleActionEdit(t, content, pathToURI(path), action)
		if !strings.Contains(fixed, "@unsafe { @trap() }") {
			t.Fatalf("unsafe action produced %q", fixed)
		}
		after, err := RunCompilerCheck(string(pathToURI(path)), fixed)
		if err != nil {
			t.Fatalf("checking wrapped operation: %v", err)
		}
		for _, diagnostic := range after {
			if strings.HasPrefix(diagnostic.Message, "Unsafe Required:") {
				t.Fatalf("unsafe diagnostic remained: %#v", after)
			}
		}
		return
	}
	t.Fatal("wrap operation code action missing")
}

func TestOrganizeImportsCodeActionPreservesNonImportLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nimport zed\nimport alpha\nfunc main(): void {}\n"
	for _, action := range GetCodeActions(content, path, protocol.Range{}, nil) {
		if action.Kind != protocol.SourceOrganizeImports {
			continue
		}
		fixed := applySingleActionEdit(t, content, pathToURI(path), action)
		want := "package main\nimport alpha\nimport zed\nfunc main(): void {}\n"
		if fixed != want {
			t.Fatalf("organized imports = %q, want %q", fixed, want)
		}
		return
	}
	t.Fatal("organize imports code action missing")
}

func TestOrganizeImportsSkipsCommentSeparatedImports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nimport zed\n// keep with zed\nimport alpha\nfunc main(): void {}\n"
	for _, action := range GetCodeActions(content, path, protocol.Range{}, nil) {
		if action.Kind == protocol.SourceOrganizeImports {
			t.Fatalf("organize imports crossed a comment: %#v", action)
		}
	}
}

func TestUnsafeOperationActionDoesNotMoveBindingIntoBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nexternal func main(): int32 {\n    let ptr = @alloc(8, 8)\n    return 0\n}\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking unsafe initializer: %v", err)
	}
	for _, action := range GetCodeActions(content, path, protocol.Range{}, diagnostics) {
		if action.Title == "Wrap operation in @unsafe" {
			t.Fatalf("unsafe action would change binding scope: %#v", action)
		}
	}
}

func TestCodeActionsForRequestedKinds(t *testing.T) {
	actions := []protocol.CodeAction{
		{Title: "fix", Kind: protocol.QuickFix},
		{Title: "imports", Kind: protocol.SourceOrganizeImports},
	}
	tests := []struct {
		name string
		only []protocol.CodeActionKind
		want string
	}{
		{name: "quickfix", only: []protocol.CodeActionKind{protocol.QuickFix}, want: "fix"},
		{name: "source", only: []protocol.CodeActionKind{protocol.Source}, want: "imports"},
		{name: "organize imports", only: []protocol.CodeActionKind{protocol.SourceOrganizeImports}, want: "imports"},
		{name: "both", only: []protocol.CodeActionKind{protocol.QuickFix, protocol.Source}, want: "fix,imports"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filtered := codeActionsForKinds(actions, test.only)
			titles := make([]string, 0, len(filtered))
			for _, action := range filtered {
				titles = append(titles, action.Title)
			}
			if got := strings.Join(titles, ","); got != test.want {
				t.Fatalf("filtered actions = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLegacyDeclarationActionPreservesDirectCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\n\ndeclare external func puts(message: string): int32\nexternal func main(): int32 { return puts(\"hello\") }\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking legacy declaration: %v", err)
	}
	for _, action := range GetCodeActions(content, path, protocol.Range{}, diagnostics) {
		if action.Title != "Convert declaration to foreign C module" {
			continue
		}
		fixed := applySingleActionEdit(t, content, pathToURI(path), action)
		if !strings.Contains(fixed, "foreign \"c\" c_abi {\n    func puts(message: string): int32\n}") || !strings.Contains(fixed, "public func puts(message: string): int32 {\n    return c_abi.puts(message)\n}") {
			t.Fatalf("unexpected foreign conversion: %s", fixed)
		}
		after, err := RunCompilerCheck(string(pathToURI(path)), fixed)
		if err != nil {
			t.Fatalf("checking converted declaration: %v", err)
		}
		for _, diagnostic := range after {
			if diagnostic.Severity == protocol.DiagnosticSeverityError || strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
				t.Fatalf("converted source has diagnostic: %#v", diagnostic)
			}
		}
		return
	}
	t.Fatalf("foreign conversion action missing: diagnostics %#v", diagnostics)
}

func TestLegacyOpaqueTypeActionKeepsTypeReferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\n\ndeclare external type Handle\nfunc use_handle(handle: Handle*): void {}\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking legacy opaque type: %v", err)
	}
	for _, action := range GetCodeActions(content, path, protocol.Range{}, diagnostics) {
		if action.Title != "Convert declaration to foreign C module" {
			continue
		}
		fixed := applySingleActionEdit(t, content, pathToURI(path), action)
		if !strings.Contains(fixed, "foreign \"c\" c_abi {\n    type Handle opaque\n}") {
			t.Fatalf("unexpected foreign type conversion: %s", fixed)
		}
		after, err := RunCompilerCheck(string(pathToURI(path)), fixed)
		if err != nil {
			t.Fatalf("checking converted opaque type: %v", err)
		}
		for _, diagnostic := range after {
			if diagnostic.Severity == protocol.DiagnosticSeverityError || strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
				t.Fatalf("converted source has diagnostic: %#v", diagnostic)
			}
		}
		return
	}
	t.Fatalf("foreign type conversion action missing: diagnostics %#v", diagnostics)
}

func TestLegacyOutParameterHasNoUnsafeConversion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\n\ndeclare external func sqlite3_open(filename: string, db: out void*): int32\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking legacy out parameter: %v", err)
	}
	for _, action := range GetCodeActions(content, path, protocol.Range{}, diagnostics) {
		if action.Title == "Convert declaration to foreign C module" {
			t.Fatalf("out parameters cannot be forwarded by a Gecko wrapper: %#v", action)
		}
	}
}

func TestLegacyDeclarationActionsShareForeignModule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\n\ndeclare external func first(): int32\ndeclare external func second(): int32\nexternal func main(): int32 { return first() + second() }\n"
	for step := 0; step < 2; step++ {
		diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
		if err != nil {
			t.Fatalf("checking declarations: %v", err)
		}
		converted := false
		for _, action := range GetCodeActions(content, path, protocol.Range{}, diagnostics) {
			if action.Title != "Convert declaration to foreign C module" {
				continue
			}
			content = applySingleActionEdit(t, content, pathToURI(path), action)
			converted = true
			break
		}
		if !converted {
			t.Fatal("foreign conversion action missing")
		}
	}
	if strings.Count(content, "foreign \"c\" c_abi {") != 2 {
		t.Fatalf("converted declarations use different modules: %s", content)
	}
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking converted declarations: %v", err)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == protocol.DiagnosticSeverityError || strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
			t.Fatalf("converted source has diagnostic: %#v", diagnostic)
		}
	}
}

func TestLegacyVariadicDeclarationHasNoUnsafeConversion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\ndeclare external variadic func printf(format: string): int32\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking variadic declaration: %v", err)
	}
	for _, action := range GetCodeActions(content, path, protocol.Range{}, diagnostics) {
		if action.Title == "Convert declaration to foreign C module" {
			t.Fatalf("variadic call cannot be forwarded safely: %#v", action)
		}
	}
}
