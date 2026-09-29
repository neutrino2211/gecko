package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/analysis"
)

func TestTraitConstraintsUseDeclarationIdentity(t *testing.T) {
	content := "package test\ntrait Printable {}\ntrait Comparable {}\nclass Box<T is Printable> {}\nfunc first<T is Printable & Comparable>(value: T): void {}\nfunc second<T>(value: T): void where T is Comparable {}\nfunc typed(value: Box is Printable): void {}\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing trait constraints: %v", err)
	}
	for _, name := range []string{"Printable", "Comparable"} {
		declaration := strings.Index(content, "trait "+name) + len("trait ")
		declOccurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, declaration)
		if !ok {
			t.Fatalf("missing %s declaration", name)
		}
		for offset := declaration + len(name); offset < len(content); {
			index := strings.Index(content[offset:], name)
			if index < 0 {
				break
			}
			offset += index
			occurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, offset)
			if !ok || occurrence.SymbolID != declOccurrence.SymbolID {
				t.Fatalf("%s constraint at %d has no declaration identity: %#v", name, offset, occurrence)
			}
			offset += len(name)
		}
	}
}

func TestTypeParametersKeepDeclarationIdentityAcrossScopes(t *testing.T) {
	content := "package test\ntrait Reader {}\nclass Box<T is Reader> {\n    let value: T\n    func get(self, item: T): T { let copy: T = item\n        return copy }\n}\nfunc identity<T>(item: T): T where T is Reader { let copy: T = item\n    return copy }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing type parameters: %v", err)
	}
	var classID int64
	for index, scope := range []struct {
		declaration string
		uses        []string
	}{
		{"Box<T", []string{"value: T", "item: T", "): T", "copy: T"}},
		{"identity<T", []string{"item: T", "): T", "where T", "copy: T"}},
	} {
		declaration := strings.Index(content, scope.declaration) + len(scope.declaration) - 1
		declOccurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, declaration)
		if !ok {
			t.Fatalf("missing %s type parameter declaration", scope.declaration)
		}
		if index == 0 {
			classID = declOccurrence.SymbolID
		} else if classID == declOccurrence.SymbolID {
			t.Fatal("class and function type parameters share an identity")
		}
		start := declaration + 1
		for _, fragment := range scope.uses {
			offset := strings.Index(content[start:], fragment)
			if offset < 0 {
				t.Fatalf("missing type parameter use %q", fragment)
			}
			start += offset + strings.Index(fragment, "T")
			use, found := ctx.SemanticGraph.OccurrenceAt(path, start)
			if !found || use.SymbolID != declOccurrence.SymbolID {
				t.Fatalf("%s type parameter use %q: %#v %#v", scope.declaration, fragment, use, declOccurrence)
			}
			start++
		}
		definition := semanticDefinition(ctx, path, content, sourcePosition(content, start-1))
		if definition == nil || definition.Range.Start != sourcePosition(content, declaration) {
			t.Fatalf("%s type parameter definition: %#v", scope.declaration, definition)
		}
		references := localReferences(ctx, path, content, sourcePosition(content, declaration), true)
		if len(references) != len(scope.uses)+1 {
			t.Fatalf("%s type parameter references: %#v", scope.declaration, references)
		}
		edit, err := renameLocal(ctx, path, content, sourcePosition(content, declaration), "U")
		if err != nil || edit == nil || len(edit.Changes[pathToURI(path)]) != len(scope.uses)+1 {
			t.Fatalf("%s type parameter rename: %#v, %v", scope.declaration, edit, err)
		}
	}
}

func TestImplementationTypeParameterLinksHeaderAndMethod(t *testing.T) {
	content := "package test\ntrait Identity<T> { func get(self): T }\nclass Box<T> { let value: T }\nimpl<T> Identity<T> for Box<T> { func get(self): T { return self.value } }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing implementation type parameter: %v", err)
	}
	declaration := strings.Index(content, "impl<T") + len("impl<")
	declOccurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, declaration)
	if !ok {
		t.Fatal("implementation type parameter declaration missing")
	}
	start := declaration + 1
	for index := 0; index < 3; index++ {
		offset := strings.Index(content[start:], "<T>")
		within := 1
		if index == 2 {
			offset = strings.Index(content[start:], ": T")
			within = 2
		}
		if offset < 0 {
			t.Fatalf("implementation type parameter use %d missing", index)
		}
		start += offset + within
		use, found := ctx.SemanticGraph.OccurrenceAt(path, start)
		if !found || use.SymbolID != declOccurrence.SymbolID {
			t.Fatalf("implementation type parameter use %d: %#v %#v", index, use, declOccurrence)
		}
		start++
	}
}

func TestTraitTypeParameterLinksMemberSignature(t *testing.T) {
	content := "package test\ntrait Reader<T> { func read(self, value: T): T }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing trait type parameter: %v", err)
	}
	declaration := strings.Index(content, "Reader<T>") + len("Reader<")
	declOccurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, declaration)
	if !ok {
		t.Fatal("trait type parameter declaration missing")
	}
	for _, fragment := range []string{"value: T", "): T"} {
		offset := strings.Index(content, fragment) + strings.Index(fragment, "T")
		use, found := ctx.SemanticGraph.OccurrenceAt(path, offset)
		if !found || use.SymbolID != declOccurrence.SymbolID {
			t.Fatalf("trait type parameter use %q: %#v %#v", fragment, use, declOccurrence)
		}
	}
}

func TestTypeParameterRenameRejectsNestedShadowing(t *testing.T) {
	content := "package test\nclass Box<T> { func convert<U>(self, item: T): T { return item } }\n"
	path := filepath.Join(t.TempDir(), "main.gecko")
	ctx, err := analysis.NewAnalysisContext(path, content)
	if err != nil {
		t.Fatalf("analyzing nested type parameters: %v", err)
	}
	declaration := strings.Index(content, "Box<T>") + len("Box<")
	if _, err := renameLocal(ctx, path, content, sourcePosition(content, declaration), "U"); err == nil {
		t.Fatal("rename accepted a method type parameter that shadows the class parameter")
	}
}
