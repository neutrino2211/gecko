package frontend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neutrino2211/gecko/frontend"
	"github.com/neutrino2211/gecko/tokens"
)

func writeDirectorySource(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryReferencesJoinSemanticGraphBeforeBackend(t *testing.T) {
	dir := t.TempDir()
	library := filepath.Join(dir, "library")
	widget := filepath.Join(library, "widget.gecko")
	helper := filepath.Join(library, "helper.gecko")
	writeDirectorySource(t, widget, "package library\npublic class Widget {}\n")
	writeDirectorySource(t, helper, "package library\npublic func value(): int32 { return 7 }\n")
	writeDirectorySource(t, filepath.Join(library, "unused.gecko"), "package library\npublic class Unused {}\n")
	path := filepath.Join(dir, "main.gecko")
	source := "package main\nimport library as Lib\nfunc run(): int32 { let item: Lib.Widget return Lib.value() }\n"
	session := frontend.NewSession()
	first := session.Analyze(path, source, nil)
	if err := first.ParseError(); err != nil {
		t.Fatal(err)
	}
	view := first.View()
	if len(view.File.Imports) != 2 {
		t.Fatalf("expected two referenced files, got %d", len(view.File.Imports))
	}
	seen := map[string]bool{}
	for _, imported := range view.File.Imports {
		if imported.Name != "Lib" || imported.Alias != "Lib" {
			t.Fatalf("lost directory alias: %q, %q", imported.Name, imported.Alias)
		}
		seen[imported.Path] = true
	}
	if !seen[widget] || !seen[helper] || view.Program.ClassForType(&tokens.TypeRef{Module: "Lib", Type: "Widget"}) == nil {
		t.Fatalf("semantic graph missed referenced imports: %#v", seen)
	}
	if len(view.Program.ModuleFunctions("Lib")) != 1 {
		t.Fatal("qualified function missing from semantic program")
	}
	if session.Analyze(path, source, nil) != first {
		t.Fatal("unchanged directory import was rebuilt")
	}
	writeDirectorySource(t, helper, "package library\npublic func value(): string { return \"changed\" }\n")
	changed := session.Analyze(path, source, nil)
	if changed == first || len(changed.View().Program.ModuleFunctions("Lib")) != 1 || changed.View().Program.ModuleFunctions("Lib")[0].ReturnType.Type != "string" {
		t.Fatal("edited directory dependency did not rebuild semantic program")
	}
}

func TestDirectoryReferenceHonorsAliasAndUseSelection(t *testing.T) {
	dir := t.TempDir()
	writeDirectorySource(t, filepath.Join(dir, "library", "widget.gecko"), "package library\npublic class Widget {}\n")
	path := filepath.Join(dir, "main.gecko")
	for _, source := range []string{
		"package main\nimport library as Lib use {Other}\nfunc run(value: Lib.Widget) {}\n",
		"package main\nimport library as Lib\nfunc run(value: library.Widget) {}\n",
	} {
		result := frontend.Analyze(path, source, nil)
		if err := result.ParseError(); err != nil {
			t.Fatal(err)
		}
		if len(result.View().File.Imports) != 0 {
			t.Fatalf("inaccessible symbol joined import graph: %q", source)
		}
	}
}
