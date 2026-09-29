package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/analysis"
	"go.lsp.dev/protocol"
)

func completionAt(t *testing.T, ctx *analysis.AnalysisContext, path, content string) []protocol.CompletionItem {
	t.Helper()
	offset := strings.IndexByte(content, '|')
	if offset < 0 || strings.Count(content, "|") != 1 {
		t.Fatalf("expected one cursor marker in %q", content)
	}
	line := strings.Count(content[:offset], "\n")
	col := offset - strings.LastIndexByte(content[:offset], '\n') - 1
	content = content[:offset] + content[offset+1:]
	return GetCompletions(ctx, content, path, line, col)
}

func requireCompletion(t *testing.T, items []protocol.CompletionItem, name string, kind protocol.CompletionItemKind) {
	t.Helper()
	for _, item := range items {
		if item.Label == name {
			if item.Kind != kind {
				t.Fatalf("completion %q kind = %v, want %v", name, item.Kind, kind)
			}
			return
		}
	}
	t.Fatalf("missing completion %q in %#v", name, items)
}

func TestForeignNamespaceCompletionFacets(t *testing.T) {
	directory := t.TempDir()
	bridgePath := filepath.Join(directory, "bridge.gecko")
	mainPath := filepath.Join(directory, "main.gecko")
	bridge := "package bridge\nforeign \"c\" c_abi {\n    type Handle opaque\n    func acquire(handle: out Handle*): int32\n    func close(handle: Handle*): int32\n}\npublic func exported(): int32 { return 0 }\n"
	base := "package main\nimport bridge\nforeign \"c\" c_abi {\n    type Handle opaque\n    func local_call(): int32\n}\nexternal func main(): int32 { return 0 }\n"
	if err := os.WriteFile(bridgePath, []byte(bridge), 0644); err != nil {
		t.Fatalf("writing bridge: %v", err)
	}
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, base, func(path string) ([]byte, error) {
		if path == bridgePath {
			return []byte(bridge), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("analyzing completion fixture: %v", err)
	}
	tests := []struct {
		name  string
		line  string
		label string
		kind  protocol.CompletionItemKind
	}{
		{"local namespace", "c_ab|", "c_abi", protocol.CompletionItemKindModule},
		{"local member", "c_abi.|", "local_call", protocol.CompletionItemKindFunction},
		{"local member prefix", "c_abi.loc|", "local_call", protocol.CompletionItemKindFunction},
		{"local foreign type", "Han|", "Handle", protocol.CompletionItemKindClass},
		{"imported namespace", "bridge.|", "c_abi", protocol.CompletionItemKindModule},
		{"imported foreign type", "bridge.Han|", "Handle", protocol.CompletionItemKindClass},
		{"imported foreign member", "bridge.c_abi.|", "acquire", protocol.CompletionItemKindFunction},
		{"imported foreign member prefix", "bridge.c_abi.ac|", "acquire", protocol.CompletionItemKindFunction},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			content := strings.Replace(base, "return 0", test.line, 1)
			items := completionAt(t, ctx, mainPath, content)
			requireCompletion(t, items, test.label, test.kind)
		})
	}
	items := completionAt(t, ctx, mainPath, strings.Replace(base, "return 0", "bridge.c_abi.ac|", 1))
	for _, item := range items {
		if item.Label == "acquire" && !strings.Contains(item.Detail, "out Handle*") {
			t.Fatalf("foreign signature lost out parameter: %q", item.Detail)
		}
	}
}

func TestCompletionIncludesLegacyAndSelectedImports(t *testing.T) {
	directory := t.TempDir()
	bridgePath := filepath.Join(directory, "bridge.gecko")
	mainPath := filepath.Join(directory, "main.gecko")
	bridge := "package bridge\npublic func exported(): int32 { return 0 }\nprivate func hidden(): int32 { return 0 }\n"
	base := "package main\nimport bridge use {exported}\ndeclare external func legacy(): int32\nexternal func main(): int32 { return 0 }\n"
	if err := os.WriteFile(bridgePath, []byte(bridge), 0644); err != nil {
		t.Fatalf("writing bridge: %v", err)
	}
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, base, func(path string) ([]byte, error) {
		if path == bridgePath {
			return []byte(bridge), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("analyzing completion fixture: %v", err)
	}
	for _, test := range []struct {
		prefix string
		label  string
		kind   protocol.CompletionItemKind
	}{{"leg|", "legacy", protocol.CompletionItemKindFunction}, {"exp|", "exported", protocol.CompletionItemKindFunction}, {"bri|", "bridge", protocol.CompletionItemKindModule}} {
		content := strings.Replace(base, "return 0", test.prefix, 1)
		requireCompletion(t, completionAt(t, ctx, mainPath, content), test.label, test.kind)
	}
}

func TestCompletionPrefersLocalReceiverOverImport(t *testing.T) {
	directory := t.TempDir()
	bridgePath := filepath.Join(directory, "bridge.gecko")
	mainPath := filepath.Join(directory, "main.gecko")
	bridge := "package bridge\npublic func exported(): int32 { return 0 }\n"
	base := "package main\nimport bridge\nclass Widget { public func value(self): int32 { return 1 } }\nexternal func main(bridge: Widget): int32 { return 0 }\n"
	if err := os.WriteFile(bridgePath, []byte(bridge), 0644); err != nil {
		t.Fatalf("writing bridge: %v", err)
	}
	ctx, err := analysis.NewAnalysisContextWithReader(mainPath, base, func(path string) ([]byte, error) {
		if path == bridgePath {
			return []byte(bridge), nil
		}
		return os.ReadFile(path)
	})
	if err != nil {
		t.Fatalf("analyzing shadowed import: %v", err)
	}
	content := strings.Replace(base, "return 0", "bridge.|", 1)
	items := completionAt(t, ctx, mainPath, content)
	requireCompletion(t, items, "value", protocol.CompletionItemKindMethod)
	for _, item := range items {
		if item.Label == "exported" {
			t.Fatal("module export offered for a local receiver")
		}
	}
}

func TestCompletionPrefersLocalReceiverOverForeignModule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	base := "package main\nforeign \"c\" c_abi { func raw(): int32 }\nclass Widget { public func value(self): int32 { return 1 } }\nexternal func main(c_abi: Widget): int32 { return 0 }\n"
	ctx, err := analysis.NewAnalysisContext(path, base)
	if err != nil {
		t.Fatalf("analyzing shadowed foreign module: %v", err)
	}
	content := strings.Replace(base, "return 0", "c_abi.|", 1)
	items := completionAt(t, ctx, path, content)
	requireCompletion(t, items, "value", protocol.CompletionItemKindMethod)
	for _, item := range items {
		if item.Label == "raw" {
			t.Fatal("foreign member offered for a local receiver")
		}
	}
}

func TestEnumCompletionsWithValidAnalysis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	base := "package main\nenum Status {\n    Pending\n    Active\n}\nexternal func main(): int32 { return 0 }\n"
	ctx, err := analysis.NewAnalysisContext(path, base)
	if err != nil {
		t.Fatalf("analyzing enum: %v", err)
	}
	content := strings.Replace(base, "return 0", "Status::|", 1)
	items := completionAt(t, ctx, path, content)
	requireCompletion(t, items, "Active", protocol.CompletionItemKindEnumMember)
	content = strings.Replace(base, "return 0", "Status.|", 1)
	items = completionAt(t, ctx, path, content)
	requireCompletion(t, items, "Active", protocol.CompletionItemKindEnumMember)
}

func TestRepeatedForeignBlockHasOneNamespaceCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	base := "package main\nforeign \"c\" c_abi { func first(): int32 }\nforeign \"c\" c_abi { func second(): int32 }\nexternal func main(): int32 { return 0 }\n"
	ctx, err := analysis.NewAnalysisContext(path, base)
	if err != nil {
		t.Fatalf("analyzing repeated foreign blocks: %v", err)
	}
	content := strings.Replace(base, "return 0", "c_ab|", 1)
	count := 0
	for _, item := range completionAt(t, ctx, path, content) {
		if item.Label == "c_abi" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("c_abi completion count = %d, want 1", count)
	}
}

func TestImportedForeignCompletionFromUnsavedAlias(t *testing.T) {
	directory := t.TempDir()
	bridgePath := filepath.Join(directory, "bridge.gecko")
	mainPath := filepath.Join(directory, "main.gecko")
	bridge := "package bridge\nforeign \"c\" c_abi { func acquire(): int32 }\n"
	if err := os.WriteFile(bridgePath, []byte(bridge), 0644); err != nil {
		t.Fatalf("writing bridge: %v", err)
	}
	content := "package main\nimport bridge as native\nexternal func main(): int32 { native.c_abi.| }\n"
	items := completionAt(t, nil, mainPath, content)
	requireCompletion(t, items, "acquire", protocol.CompletionItemKindFunction)
}

func TestCompletionIncludesCurrentKeywords(t *testing.T) {
	for _, keyword := range []string{"foreign", "enum", "match", "defer", "out", "readonly"} {
		content := "package main\n" + keyword[:3] + "|\n"
		items := completionAt(t, nil, "main.gecko", content)
		requireCompletion(t, items, keyword, protocol.CompletionItemKindKeyword)
	}
}

func TestCompletionAcrossManyForeignBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	var source strings.Builder
	source.WriteString("package main\n")
	for index := 0; index < 62; index++ {
		fmt.Fprintf(&source, "foreign \"c\" c_abi { func fn%d(): int32 }\n", index)
	}
	source.WriteString("external func main(): int32 { return 0 }\n")
	base := source.String()
	ctx, err := analysis.NewAnalysisContext(path, base)
	if err != nil {
		t.Fatalf("analyzing foreign blocks: %v", err)
	}
	content := strings.Replace(base, "return 0", "c_abi.fn6|", 1)
	items := completionAt(t, ctx, path, content)
	for _, name := range []string{"fn6", "fn60", "fn61"} {
		requireCompletion(t, items, name, protocol.CompletionItemKindFunction)
	}
	if len(items) != 3 {
		t.Fatalf("prefix completion count = %d, want 3", len(items))
	}
}

func TestForeignNamespaceCompletionIgnoresOtherMethodLocals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	base := "package main\nforeign \"c\" c_abi { func raw(): int32 }\nclass Widget { public func value(self): int32 { return 1 } }\nfunc unrelated(c_abi: Widget): int32 { return 0 }\nexternal func main(): int32 { return 0 }\n"
	content := strings.Replace(base, "external func main(): int32 { return 0 }", "external func main(): int32 { c_abi.| }", 1)
	items := completionAt(t, nil, path, content)
	requireCompletion(t, items, "raw", protocol.CompletionItemKindFunction)
	for _, item := range items {
		if item.Label == "value" {
			t.Fatal("local from another method hid the foreign namespace")
		}
	}
}

func TestImportedClassCompletionFromUnsavedAlias(t *testing.T) {
	directory := t.TempDir()
	bridgePath := filepath.Join(directory, "bridge.gecko")
	mainPath := filepath.Join(directory, "main.gecko")
	bridge := "package bridge\npublic class Widget { public func value(self): int32 { return 1 } }\n"
	if err := os.WriteFile(bridgePath, []byte(bridge), 0644); err != nil {
		t.Fatalf("writing bridge: %v", err)
	}
	content := "package main\nimport bridge as native\nexternal func main(widget: native.Widget): int32 { widget.| }\n"
	items := completionAt(t, nil, mainPath, content)
	requireCompletion(t, items, "value", protocol.CompletionItemKindMethod)
}

func TestCompletionDoesNotLeakPreviousMethodLocals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nfunc unrelated(oldValue: int32): int32 { return oldValue }\nexternal func main(): int32 { old| }\n"
	for _, item := range completionAt(t, nil, path, content) {
		if item.Label == "oldValue" {
			t.Fatal("parameter from previous method appeared in main")
		}
	}
}
