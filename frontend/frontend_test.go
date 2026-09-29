package frontend_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/frontend"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

func TestSessionReusesSourcesAndInvalidatesDependencies(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "main.gecko")
	lib := filepath.Join(dir, "lib.gecko")
	sources := map[string]string{lib: "package lib\npublic func value(): int32 { return 1 }\n"}
	cfg := &config.CompileCfg{ReadSource: func(path string) ([]byte, error) {
		if content, ok := sources[path]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}}
	content := "package main\nimport lib\nfunc run(): int32 { return lib.value() }\n"
	session := frontend.NewSession()
	first := session.Analyze(main, content, cfg)
	if first.ParseError() != nil || first.View().Program == nil {
		t.Fatalf("analysis failed: %v", first.ParseError())
	}
	if second := session.Analyze(main, content, cfg); second != first {
		t.Fatal("unchanged snapshot was rebuilt")
	}
	if parses, analyses := session.Counts(); parses != 2 || analyses != 1 {
		t.Fatalf("counts: %d parses, %d analyses", parses, analyses)
	}
	sources[lib] = "package lib\npublic func value(): int32 { return 2 }\n"
	if changed := session.Analyze(main, content, cfg); changed == first {
		t.Fatal("dependency edit reused stale result")
	}
	if parses, analyses := session.Counts(); parses != 3 || analyses != 2 {
		t.Fatalf("counts after dependency edit: %d, %d", parses, analyses)
	}
	session.Analyze(filepath.Join(dir, "other.gecko"), content, cfg)
	if parses, analyses := session.Counts(); parses != 4 || analyses != 3 {
		t.Fatalf("shared import was reparsed: %d, %d", parses, analyses)
	}
}

func TestSessionSyntaxSharesParsingWithoutSharingMutableData(t *testing.T) {
	session := frontend.NewSession()
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nclass Widget {}\n"
	first, err := session.Syntax(path, content)
	if err != nil {
		t.Fatal(err)
	}
	first.File.Entries[0].Class.Name = "Changed"
	first.Tokens[0].Value = "Changed"
	second, err := session.Syntax(path, content)
	if err != nil {
		t.Fatal(err)
	}
	if second.File.Entries[0].Class.Name != "Widget" || second.Tokens[0].Value == "Changed" {
		t.Fatal("syntax caller mutated the cached parse")
	}
	if parses, analyses := session.Counts(); parses != 1 || analyses != 0 {
		t.Fatalf("unexpected frontend work: %d parses, %d analyses", parses, analyses)
	}
}

func TestCachedAndFreshCallFactsAgree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nfunc answer(value: int32): int32 { return value }\nexternal func main(): int32 { return answer(7) }\n"
	session := frontend.NewSession()
	cached := session.Analyze(path, content, nil)
	if session.Analyze(path, content, nil) != cached {
		t.Fatal("unchanged analysis was not cached")
	}
	fresh := frontend.Analyze(path, content, nil)
	readCall := func(result *frontend.Result) *semantic.CallResolution {
		view := result.View()
		var call *tokens.FuncCall
		tokens.WalkSyntaxEntries(view.File.Entries, func(node any) {
			if candidate, ok := node.(*tokens.FuncCall); ok && candidate.Function == "answer" {
				call = candidate
			}
		})
		return view.Program.FuncCallResolution(call)
	}
	cachedResolution := readCall(cached)
	freshResolution := readCall(fresh)
	if cachedResolution == nil || cachedResolution.Status != semantic.ResolutionResolved || !reflect.DeepEqual(cachedResolution, freshResolution) {
		t.Fatalf("cached and fresh calls differ: %#v %#v", cachedResolution, freshResolution)
	}
	if !reflect.DeepEqual(cached.View().Program.Diagnostics(), fresh.View().Program.Diagnostics()) {
		t.Fatal("cached and fresh diagnostics differ")
	}
}

func TestBackendCloneRetainsSemanticIdentityWithoutMutatingSnapshot(t *testing.T) {
	content := "package main\nfunc run(): int32 { let answer = 42 return answer }\n"
	result := frontend.Analyze("main.gecko", content, nil)
	copy := result.CloneForBackend(&config.CompileCfg{CheckOnly: true})
	original := result.View().File.Entries[0].Method.Value[0].Field
	cloned := copy.File.Entries[0].Method.Value[0].Field
	if original == cloned || original.Value == cloned.Value {
		t.Fatal("backend shares mutable syntax")
	}
	if typ := copy.Program.TypeOfExpression(cloned.Value); typ == nil || typ.Type != "int32" {
		t.Fatalf("cloned inference lost: %#v", typ)
	}
	cloned.Name = "changed"
	if original.Name != "answer" {
		t.Fatal("backend mutation changed source snapshot")
	}
}

func TestVisibleBindingsRespectSelfShadowingAndBranches(t *testing.T) {
	content := `package main
class Box {
 func read(self, value: int32): int32 {
  if true {
   let value = "inner"
   let inside = value
  } else {
   let other = value
  }
  let outside = value
  return value
 }
}
func separate(value: bool): bool { return value }
`
	result := frontend.Analyze("scope.gecko", content, nil)
	if result.ParseError() != nil {
		t.Fatal(result.ParseError())
	}
	for fragment, want := range map[string]string{"let inside": "string", "let other": "int32", "let outside": "int32", "return value }": "bool"} {
		symbol := result.View().Program.LookupSymbol("scope.gecko", strings.Index(content, fragment), "value")
		if symbol == nil || semantic.TypeRefString(symbol.Type) != want {
			t.Fatalf("%s: want %s, got %#v", fragment, want, symbol)
		}
	}
	self := result.View().Program.LookupSymbol("scope.gecko", strings.Index(content, "let outside"), "self")
	if self == nil || self.Type == nil || self.Type.Type != "Box" {
		t.Fatalf("self type missing: %#v", self)
	}
	if symbol := result.View().Program.LookupSymbol("scope.gecko", strings.Index(content, "let outside"), "inside"); symbol != nil {
		t.Fatal("branch binding leaked")
	}
}

func TestSessionInvalidatesMissingImportsAndProjectChanges(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "main.gecko")
	lib := filepath.Join(dir, "lib.gecko")
	sources := map[string]string{}
	project := &config.ProjectConfig{ProjectRoot: dir, Dependencies: map[string]*config.Dependency{"lib": {Path: "first"}}}
	cfg := &config.CompileCfg{Project: project, ReadSource: func(path string) ([]byte, error) {
		if source, ok := sources[path]; ok {
			return []byte(source), nil
		}
		return nil, os.ErrNotExist
	}}
	source := "package main\nimport lib\n"
	session := frontend.NewSession()
	missing := session.Analyze(main, source, cfg)
	sources[lib] = "package lib\npublic class Widget {}\n"
	loaded := session.Analyze(main, source, cfg)
	if loaded == missing || len(loaded.View().File.Imports) != 1 {
		t.Fatal("new import did not invalidate missing lookup")
	}
	project.Dependencies["lib"].Path = "second"
	if session.Analyze(main, source, cfg) == loaded {
		t.Fatal("in-place configuration change reused stale result")
	}
}

func TestSessionReusesInvalidSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.gecko")
	source := "package main\n$"
	session := frontend.NewSession()
	first := session.Analyze(path, source, nil)
	if first.ParseError() == nil {
		t.Fatal("expected lexical error")
	}
	if session.Analyze(path, source, nil) != first {
		t.Fatal("invalid snapshot was rebuilt")
	}
	if clone := first.CloneForBackend(nil); clone.Path != path || clone.Content != source || clone.ParseError == nil {
		t.Fatal("invalid snapshot lost diagnostic source")
	}
}

func TestSessionInvalidatesDirectoryChanges(t *testing.T) {
	dir := t.TempDir()
	library := filepath.Join(dir, "library")
	if err := os.Mkdir(library, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.gecko")
	source := "package main\nimport library use {Widget}\n"
	session := frontend.NewSession()
	first := session.Analyze(path, source, nil)
	if err := os.WriteFile(filepath.Join(library, "widget.gecko"), []byte("package widget\npublic class Widget {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	second := session.Analyze(path, source, nil)
	if second == first || len(second.View().File.Imports) != 1 {
		t.Fatal("directory addition reused stale imports")
	}
}

func TestConcurrentFrontendSessionsKeepSelfTypesSeparate(t *testing.T) {
	var work sync.WaitGroup
	failures := make(chan string, 12)
	for index := 0; index < cap(failures); index++ {
		work.Add(1)
		go func(index int) {
			defer work.Done()
			name := fmt.Sprintf("Box%d", index)
			source := fmt.Sprintf("package main\nclass %s { func copy(self): Self { return self } }\n", name)
			path := name + ".gecko"
			result := frontend.Analyze(path, source, nil)
			if result.ParseError() != nil {
				failures <- result.ParseError().Error()
				return
			}
			symbol := result.View().Program.LookupSymbol(path, strings.Index(source, "return self"), "self")
			if symbol == nil || symbol.Type == nil || symbol.Type.Type != name {
				failures <- name + " lost its self type"
			}
		}(index)
	}
	work.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

func TestPublishedSourceReaderDoesNotChangeDependencies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	extra := filepath.Join(filepath.Dir(path), "unrelated.gecko")
	sources := map[string]string{}
	cfg := &config.CompileCfg{ReadSource: func(path string) ([]byte, error) {
		if content, ok := sources[path]; ok {
			return []byte(content), nil
		}
		return nil, os.ErrNotExist
	}}
	session := frontend.NewSession()
	source := "package main\n"
	first := session.Analyze(path, source, cfg)
	if _, err := first.View().File.Config.ReadSource(extra); !os.IsNotExist(err) {
		t.Fatalf("expected missing source: %v", err)
	}
	sources[extra] = "package unrelated\n"
	if session.Analyze(path, source, cfg) != first {
		t.Fatal("editor-only read changed the published dependency snapshot")
	}
}

func TestCachedResultIsolatedFromEditorAndBackendViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nclass Box { public let value: int32\npublic func read(self): int32 { return self.value } }\n"
	session := frontend.NewSession()
	result := session.Analyze(path, content, nil)
	if err := result.ParseError(); err != nil {
		t.Fatal(err)
	}
	first := result.View()
	class := first.Program.Class("Box")
	if class == nil || len(first.Program.MethodsOfType("Box")) == 0 {
		t.Fatal("expected semantic class and method")
	}
	first.File.Entries[0].Class.Name = "Changed"
	first.Program.Symbols[class.SymbolID].Name = "Changed"
	class.Fields["value"].Type = "string"
	first.Program.MethodsOfType("Box")[0].ReturnType.Type = "string"
	first.Program.Symbols[class.SymbolID].Type.Type = "Changed"
	fresh := result.View()
	if fresh.File.Entries[0].Class.Name != "Box" || fresh.Program.Symbols[class.SymbolID].Name != "Box" {
		t.Fatal("editor mutation reached cached syntax or symbol")
	}
	if fresh.Program.Class("Box").Fields["value"].Type != "int32" || fresh.Program.MethodsOfType("Box")[0].ReturnType.Type != "int32" {
		t.Fatal("editor mutation reached cached type or signature")
	}
	backend := result.CloneForBackend(&config.CompileCfg{CheckOnly: true})
	backend.Program.Symbols[class.SymbolID].Name = "BackendChanged"
	backend.File.Entries[0].Class.Name = "BackendChanged"
	if result.View().File.Entries[0].Class.Name != "Box" || result.View().Program.Symbols[class.SymbolID].Name != "Box" {
		t.Fatal("backend mutation reached cached result")
	}
	if session.Analyze(path, content, nil) != result {
		t.Fatal("view mutation invalidated the cached result")
	}
}

func TestCachedResultReturnsIndependentDiagnosticsAndTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nimport missing\n"
	result := frontend.Analyze(path, content, nil)
	view := result.View()
	if len(view.Scopes) == 0 || len(view.Scopes[0].CompileTimeErrors) == 0 {
		t.Fatal("expected missing import diagnostic")
	}
	original := view.Scopes[0].CompileTimeErrors[0].Message
	view.Scopes[0].CompileTimeErrors[0].Message = "changed"
	view.Scopes[0].Name = "changed"
	tokens := result.LexicalTokens(path)
	if len(tokens) == 0 {
		t.Fatal("expected lexical tokens")
	}
	tokens[0].Value = "changed"
	fresh := result.View()
	if fresh.Scopes[0].Name == "changed" || fresh.Scopes[0].CompileTimeErrors[0].Message != original {
		t.Fatal("diagnostics mutated through a view")
	}
	if result.LexicalTokens(path)[0].Value == "changed" {
		t.Fatal("lexical tokens mutated through a query")
	}
}

func TestLazyImportMutationStaysInCallerView(t *testing.T) {
	dir := t.TempDir()
	library := filepath.Join(dir, "library")
	if err := os.Mkdir(library, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "widget.gecko"), []byte("package widget\npublic class Widget {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.gecko")
	result := frontend.Analyze(path, "package main\nimport library\n", nil)
	view := result.View()
	if len(view.File.Imports) != 0 {
		t.Fatal("unselected directory import was already loaded")
	}
	if _, ok := view.ResolveType("Widget"); !ok || len(view.File.Imports) != 1 {
		t.Fatal("caller view did not resolve directory type")
	}
	if len(result.View().File.Imports) != 0 {
		t.Fatal("lazy import changed the cached result")
	}
}

func TestEditorConfigAndSourceReadsCannotMutateSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.gecko")
	lib := filepath.Join(dir, "lib.gecko")
	project := &config.ProjectConfig{ProjectRoot: dir}
	cfg := &config.CompileCfg{Project: project, CFlags: []string{"-O1"}, ReadSource: func(path string) ([]byte, error) {
		if path == lib {
			return []byte("package lib\npublic class Widget {}\n"), nil
		}
		return nil, os.ErrNotExist
	}}
	source := "package main\nimport lib\n"
	session := frontend.NewSession()
	result := session.Analyze(path, source, cfg)
	view := result.View()
	view.File.Config.Project.ProjectRoot = "changed"
	view.File.Config.CFlags[0] = "-O0"
	read, err := view.File.Config.ReadSource(lib)
	if err != nil {
		t.Fatal(err)
	}
	read[0] = 'X'
	fresh := result.View()
	if fresh.File.Config.Project.ProjectRoot != dir || fresh.File.Config.CFlags[0] != "-O1" {
		t.Fatal("editor configuration mutation reached cached result")
	}
	read, err = fresh.File.Config.ReadSource(lib)
	if err != nil || !strings.HasPrefix(string(read), "package lib") {
		t.Fatalf("source read changed cached bytes: %q, %v", read, err)
	}
	if session.Analyze(path, source, cfg) != result {
		t.Fatal("editor mutation invalidated cached result")
	}
}
