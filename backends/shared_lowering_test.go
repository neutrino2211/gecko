// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"os/exec"
	"testing"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type fakeBackendImpl struct {
	calls []string
}

func (f *fakeBackendImpl) NewReturn(*ast.Ast) { f.calls = append(f.calls, "return") }
func (f *fakeBackendImpl) NewReturnLiteral(*ast.Ast, *tokens.Expression) {
	f.calls = append(f.calls, "return_literal")
}
func (f *fakeBackendImpl) FuncCall(*ast.Ast, *tokens.FuncCall) {
	f.calls = append(f.calls, "func_call")
}
func (f *fakeBackendImpl) MethodCall(*ast.Ast, *tokens.MethodCall) {
	f.calls = append(f.calls, "method_call")
}
func (f *fakeBackendImpl) Declaration(*ast.Ast, *tokens.Declaration) {}
func (f *fakeBackendImpl) NewVariable(scope *ast.Ast, field *tokens.Field) {
	f.calls = append(f.calls, "field")
	scope.Variables[field.Name] = ast.Variable{Name: field.Name, Parent: scope}
}
func (f *fakeBackendImpl) NewMethod(scope *ast.Ast, method *tokens.Method) {
	f.calls = append(f.calls, "method")
	scope.Methods[method.Name] = &ast.Method{Name: method.Name, Parent: scope, Scope: scope}
}
func (f *fakeBackendImpl) ParseExpression(*ast.Ast, *tokens.Expression) {}
func (f *fakeBackendImpl) ProcessEntries(*ast.Ast, []*tokens.Entry)     {}
func (f *fakeBackendImpl) NewClass(scope *ast.Ast, class *tokens.Class) {
	f.calls = append(f.calls, "class")
	scope.Classes[class.Name] = &ast.Ast{Scope: class.Name, Parent: scope}
}
func (f *fakeBackendImpl) NewDeclaration(scope *ast.Ast, decl *tokens.Declaration) {
	f.calls = append(f.calls, "declaration")
	if decl.Field != nil {
		f.NewVariable(scope, decl.Field)
	}
	if decl.Method != nil {
		f.NewMethod(scope, decl.Method)
	}
}
func (f *fakeBackendImpl) NewImplementation(*ast.Ast, *tokens.Implementation) {
	f.calls = append(f.calls, "impl")
}
func (f *fakeBackendImpl) NewTrait(*ast.Ast, *tokens.Trait) { f.calls = append(f.calls, "trait") }
func (f *fakeBackendImpl) NewEnum(*ast.Ast, *tokens.Enum)   { f.calls = append(f.calls, "enum") }
func (f *fakeBackendImpl) NewIf(*ast.Ast, *tokens.If)       { f.calls = append(f.calls, "if") }
func (f *fakeBackendImpl) NewLoop(*ast.Ast, *tokens.Loop)   { f.calls = append(f.calls, "loop") }
func (f *fakeBackendImpl) NewAssignment(*ast.Ast, *tokens.Assignment) {
	f.calls = append(f.calls, "assignment")
}
func (f *fakeBackendImpl) NewBreak(*ast.Ast)            { f.calls = append(f.calls, "break") }
func (f *fakeBackendImpl) NewContinue(*ast.Ast)         { f.calls = append(f.calls, "continue") }
func (f *fakeBackendImpl) NewAsm(*ast.Ast, *tokens.Asm) { f.calls = append(f.calls, "asm") }
func (f *fakeBackendImpl) IntrinsicStatement(*ast.Ast, *tokens.Intrinsic) {
	f.calls = append(f.calls, "intrinsic")
}
func (f *fakeBackendImpl) NewCImport(*ast.Ast, *tokens.CImport) { f.calls = append(f.calls, "cimport") }

type fakeBackend struct {
	impl *fakeBackendImpl
}

func (f *fakeBackend) Init()                                       {}
func (f *fakeBackend) Compile(*interfaces.BackendConfig) *exec.Cmd { return nil }
func (f *fakeBackend) GetImpls() interfaces.BackendCodegenImplementations {
	return f.impl
}
func (f *fakeBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	BackendProcessEntries(f, scope, entries)
}
func (f *fakeBackend) Features() interfaces.FeatureChecker {
	return NewCoreOnlyFeatureSet("fake")
}

func TestBackendProcessEntriesUsesSharedLoweringOrder(t *testing.T) {
	b := &fakeBackend{impl: &fakeBackendImpl{}}
	scope := &ast.Ast{Scope: "main"}
	scope.Init(nil)

	entries := []*tokens.Entry{
		{Field: &tokens.Field{Name: "x", Type: &tokens.TypeRef{Type: "int32"}}},
		{Method: &tokens.Method{Name: "run"}},
		{If: &tokens.If{}},
		{Break: boolPtr(true)},
		{Continue: boolPtr(true)},
		{VoidReturn: boolPtr(true)},
	}

	b.ProcessEntries(entries, scope)

	want := []string{"field", "method", "if", "break", "continue", "return"}
	if len(b.impl.calls) != len(want) {
		t.Fatalf("unexpected call count: got %d want %d (%v)", len(b.impl.calls), len(want), b.impl.calls)
	}
	for i := range want {
		if b.impl.calls[i] != want[i] {
			t.Fatalf("unexpected call at index %d: got %q want %q (all calls: %v)", i, b.impl.calls[i], want[i], b.impl.calls)
		}
	}
}

func TestPrepareSharedCompilePipelineImportUseAndNestedResolution(t *testing.T) {
	b := &fakeBackend{impl: &fakeBackendImpl{}}

	utilFile := &tokens.File{
		Name:        "util",
		PackageName: "util",
		Path:        "/virtual/util.gecko",
		Content:     "package util",
		Entries: []*tokens.Entry{
			{Field: &tokens.Field{Name: "helper", Type: &tokens.TypeRef{Type: "int32"}}},
		},
	}

	coreFile := &tokens.File{
		Name:        "core",
		PackageName: "core",
		Path:        "/virtual/core.gecko",
		Content:     "package core",
		Entries: []*tokens.Entry{
			{Import: &tokens.Import{Path: []string{"util"}, Objects: []string{"helper"}}},
			{Field: &tokens.Field{Name: "answer", Type: &tokens.TypeRef{Type: "int32"}}},
		},
		Imports: []*tokens.File{utilFile},
	}

	rootFile := &tokens.File{
		Name:        "main",
		PackageName: "main",
		Path:        "/virtual/main.gecko",
		Content:     "package main",
		Entries: []*tokens.Entry{
			{Import: &tokens.Import{Path: []string{"core"}, Objects: []string{"answer"}}},
			{Method: &tokens.Method{Name: "main"}},
		},
		Imports: []*tokens.File{coreFile},
	}

	result := PrepareSharedCompilePipeline(b, &interfaces.BackendConfig{
		SourceFile: rootFile,
	}, SharedCompilePipelineOptions{
		ProcessImports:            true,
		MarkImportedModules:       true,
		TrackLazyResolvedAsImport: false,
	})

	if result.RootScope == nil {
		t.Fatalf("expected shared pipeline root scope to be initialized")
	}

	coreScope, ok := result.RootScope.Children["core"]
	if !ok || coreScope == nil {
		t.Fatalf("expected root scope to contain imported module 'core'")
	}

	if _, ok := result.RootScope.Variables["answer"]; !ok {
		t.Fatalf("expected 'import core use { answer }' to copy symbol into root scope")
	}

	if _, ok := coreScope.Variables["helper"]; !ok {
		t.Fatalf("expected nested 'import util use { helper }' to copy symbol into module scope")
	}

	if len(result.ImportScopes) != 2 {
		t.Fatalf("expected two import scopes (core + util), got %d", len(result.ImportScopes))
	}
}

func boolPtr(v bool) *bool {
	return &v
}
