// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package tests

import (
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/parser"
)

func stripQuotes(v string) string {
	if len(v) >= 2 && strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		return v[1 : len(v)-1]
	}
	return v
}

func TestDotNotationImports(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		expectedPath   string
		expectedModule string
		expectedUse    []string
	}{
		{
			name: "simple dot notation",
			code: `package main
import std.collections.vec`,
			expectedPath:   "std.collections.vec",
			expectedModule: "vec",
			expectedUse:    nil,
		},
		{
			name: "two level import",
			code: `package main
import std.option`,
			expectedPath:   "std.option",
			expectedModule: "option",
			expectedUse:    nil,
		},
		{
			name: "single level import",
			code: `package main
import math`,
			expectedPath:   "math",
			expectedModule: "math",
			expectedUse:    nil,
		},
		{
			name: "import with use clause",
			code: `package main
import std.option use { Option, Some, None }`,
			expectedPath:   "std.option",
			expectedModule: "option",
			expectedUse:    []string{"Option", "Some", "None"},
		},
		{
			name: "deep import path",
			code: `package main
import std.collections.hash.map`,
			expectedPath:   "std.collections.hash.map",
			expectedModule: "map",
			expectedUse:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.Parser.ParseString("test.gecko", tc.code)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if len(file.Entries) == 0 {
				t.Fatal("No entries parsed")
			}

			imp := file.Entries[0].Import
			if imp == nil {
				t.Fatal("First entry is not an import")
			}

			if imp.Package() != tc.expectedPath {
				t.Errorf("Expected path %q, got %q", tc.expectedPath, imp.Package())
			}

			if imp.ModuleName() != tc.expectedModule {
				t.Errorf("Expected module %q, got %q", tc.expectedModule, imp.ModuleName())
			}

			if tc.expectedUse != nil {
				if len(imp.Objects) != len(tc.expectedUse) {
					t.Errorf("Expected %d use objects, got %d", len(tc.expectedUse), len(imp.Objects))
				} else {
					for i, expected := range tc.expectedUse {
						if imp.Objects[i] != expected {
							t.Errorf("Use object %d: expected %q, got %q", i, expected, imp.Objects[i])
						}
					}
				}
			}
		})
	}
}

func TestCImportClausesParsing(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		expectedLibs  []string
		expectedObjs  []string
		expectedCount int
	}{
		{
			name: "header only",
			code: `package main
cimport "<stdio.h>"`,
			expectedCount: 0,
		},
		{
			name: "withlibrary only",
			code: `package main
cimport "<gtk/gtk.h>" withlibrary "gtk4"`,
			expectedLibs:  []string{"gtk4"},
			expectedCount: 1,
		},
		{
			name: "withobject only",
			code: `package main
cimport "mylib.h" withobject "build/mylib.o"`,
			expectedObjs:  []string{"build/mylib.o"},
			expectedCount: 1,
		},
		{
			name: "both clauses same statement",
			code: `package main
cimport "swiftlib.h" withlibrary "swiftlib" withobject "build/swiftlib.o"`,
			expectedLibs:  []string{"swiftlib"},
			expectedObjs:  []string{"build/swiftlib.o"},
			expectedCount: 2,
		},
		{
			name: "both clauses reverse order",
			code: `package main
cimport "swiftlib.h" withobject "build/swiftlib.o" withlibrary "swiftlib"`,
			expectedLibs:  []string{"swiftlib"},
			expectedObjs:  []string{"build/swiftlib.o"},
			expectedCount: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.Parser.ParseString("test.gecko", tc.code)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if len(file.Entries) == 0 || file.Entries[0].CImport == nil {
				t.Fatal("First entry is not cimport")
			}

			cimp := file.Entries[0].CImport
			if len(cimp.Clauses) != tc.expectedCount {
				t.Fatalf("Expected %d clauses, got %d", tc.expectedCount, len(cimp.Clauses))
			}

			var libs []string
			for _, lib := range cimp.GetWithLibraries() {
				libs = append(libs, stripQuotes(lib))
			}
			var objs []string
			for _, obj := range cimp.GetWithObjects() {
				objs = append(objs, stripQuotes(obj))
			}

			if len(libs) != len(tc.expectedLibs) {
				t.Fatalf("Expected %d libs, got %d: %v", len(tc.expectedLibs), len(libs), libs)
			}
			for i, lib := range tc.expectedLibs {
				if libs[i] != lib {
					t.Fatalf("Library %d mismatch: expected %q got %q", i, lib, libs[i])
				}
			}

			if len(objs) != len(tc.expectedObjs) {
				t.Fatalf("Expected %d objects, got %d: %v", len(tc.expectedObjs), len(objs), objs)
			}
			for i, obj := range tc.expectedObjs {
				if objs[i] != obj {
					t.Fatalf("Object %d mismatch: expected %q got %q", i, obj, objs[i])
				}
			}
		})
	}
}

func TestHookAttributes(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		attrName        string
		expectedMethods []string
	}{
		{
			name: "single hook method",
			code: `package main
@drop_hook(.drop)
trait Drop {
    func drop(self): void
}`,
			attrName:        "drop_hook",
			expectedMethods: []string{"drop"},
		},
		{
			name: "multiple hook methods",
			code: `package main
@iterator_hook(.next, .has_next)
trait Iterator {
    func next(self): int32
    func has_next(self): bool
}`,
			attrName:        "iterator_hook",
			expectedMethods: []string{"next", "has_next"},
		},
		{
			name: "operator hook",
			code: `package main
@add_hook(.add)
trait Add {
    func add(self, other: int32): int32
}`,
			attrName:        "add_hook",
			expectedMethods: []string{"add"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.Parser.ParseString("test.gecko", tc.code)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if len(file.Entries) == 0 {
				t.Fatal("No entries parsed")
			}

			trait := file.Entries[0].Trait
			if trait == nil {
				t.Fatal("First entry is not a trait")
			}

			if len(trait.Attributes) == 0 {
				t.Fatal("Trait has no attributes")
			}

			attr := trait.Attributes[0]
			if attr.Name != tc.attrName {
				t.Errorf("Expected attribute name %q, got %q", tc.attrName, attr.Name)
			}

			methods := attr.GetHookMethods()
			if len(methods) != len(tc.expectedMethods) {
				t.Errorf("Expected %d hook methods, got %d: %v", len(tc.expectedMethods), len(methods), methods)
			} else {
				for i, expected := range tc.expectedMethods {
					if methods[i] != expected {
						t.Errorf("Hook method %d: expected %q, got %q", i, expected, methods[i])
					}
				}
			}
		})
	}
}

func TestTraitInheritanceParsing(t *testing.T) {
	code := `package main
trait Parent {
    func value(self): int32
}

trait Child: Parent {
    func extra(self): int32
}`

	file, err := parser.Parser.ParseString("test.gecko", code)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Entries) < 2 {
		t.Fatalf("Expected at least 2 entries, got %d", len(file.Entries))
	}

	parent := file.Entries[0].Trait
	if parent == nil || parent.Name != "Parent" {
		t.Fatalf("First entry should be trait Parent")
	}

	child := file.Entries[1].Trait
	if child == nil || child.Name != "Child" {
		t.Fatalf("Second entry should be trait Child")
	}

	if child.Parent != "Parent" {
		t.Fatalf("Expected Child parent to be Parent, got %q", child.Parent)
	}

	parents := child.AllParents()
	if len(parents) != 1 || parents[0] != "Parent" {
		t.Fatalf("Expected AllParents to return [Parent], got %v", parents)
	}
}

func TestStringAttributes(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		attrName      string
		expectedValue string
	}{
		{
			name: "section attribute on function",
			code: `package main
@section(".text.boot")
func boot(): void {
}`,
			attrName:      "section",
			expectedValue: ".text.boot",
		},
		{
			name: "backend attribute on file",
			code: `@backend("llvm")
package main`,
			attrName:      "backend",
			expectedValue: "llvm",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.Parser.ParseString("test.gecko", tc.code)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			var attrValue string

			// Check file-level attributes first
			for _, attr := range file.Attributes {
				if attr.Name == tc.attrName {
					attrValue = attr.GetStringValue()
					break
				}
			}

			// Check entry-level attributes
			if attrValue == "" && len(file.Entries) > 0 {
				entry := file.Entries[0]
				if entry.Method != nil && len(entry.Method.Attributes) > 0 {
					for _, attr := range entry.Method.Attributes {
						if attr.Name == tc.attrName {
							attrValue = attr.GetStringValue()
							break
						}
					}
				}
			}

			if attrValue != tc.expectedValue {
				t.Errorf("Expected attribute value %q, got %q", tc.expectedValue, attrValue)
			}
		})
	}
}

func TestVisibilityParsing(t *testing.T) {
	tests := []struct {
		name               string
		code               string
		expectedVisibility string
	}{
		{
			name: "public class",
			code: `package main
public class Point {
    let x: int32
}`,
			expectedVisibility: "public",
		},
		{
			name: "private class",
			code: `package main
private class Internal {
    let x: int32
}`,
			expectedVisibility: "private",
		},
		{
			name: "default visibility (empty)",
			code: `package main
class DefaultVis {
    let x: int32
}`,
			expectedVisibility: "",
		},
		{
			name: "public function",
			code: `package main
public func exported(): void {
}`,
			expectedVisibility: "public",
		},
		{
			name: "public trait",
			code: `package main
public trait Visible {
    func method(self): void
}`,
			expectedVisibility: "public",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.Parser.ParseString("test.gecko", tc.code)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if len(file.Entries) == 0 {
				t.Fatal("No entries parsed")
			}

			entry := file.Entries[0]
			var visibility string

			if entry.Class != nil {
				visibility = entry.Class.Visibility
			} else if entry.Method != nil {
				visibility = entry.Method.Visibility
			} else if entry.Trait != nil {
				visibility = entry.Trait.Visibility
			} else {
				t.Fatal("Entry is not a class, method, or trait")
			}

			if visibility != tc.expectedVisibility {
				t.Errorf("Expected visibility %q, got %q", tc.expectedVisibility, visibility)
			}
		})
	}
}

func TestAttributeOnMethodInsideClass(t *testing.T) {
	code := `package main
class MyClass {
    @section(".text.hot")
    func hot_method(): void {
    }
}`

	file, err := parser.Parser.ParseString("test.gecko", code)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Entries) == 0 {
		t.Fatal("No entries parsed")
	}

	class := file.Entries[0].Class
	if class == nil {
		t.Fatal("First entry is not a class")
	}

	if len(class.Fields) == 0 {
		t.Fatal("Class has no fields")
	}

	method := class.Fields[0].Method
	if method == nil {
		t.Fatal("First field is not a method")
	}

	if len(method.Attributes) == 0 {
		t.Fatal("Method has no attributes")
	}

	attr := method.Attributes[0]
	if attr.Name != "section" {
		t.Errorf("Expected attribute name 'section', got %q", attr.Name)
	}

	if attr.GetStringValue() != ".text.hot" {
		t.Errorf("Expected attribute value '.text.hot', got %q", attr.GetStringValue())
	}
}
