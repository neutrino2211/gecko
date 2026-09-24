// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package tests

import (
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
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
		{
			name: "import with alias",
			code: `package main
import std.collections.vec as Vec`,
			expectedPath:   "std.collections.vec",
			expectedModule: "Vec",
			expectedUse:    nil,
		},
		{
			name: "single level import with alias",
			code: `package main
import math as M`,
			expectedPath:   "math",
			expectedModule: "M",
			expectedUse:    nil,
		},
		{
			name: "import with alias and use clause",
			code: `package main
import std.option as Opt use { Option, Some, None }`,
			expectedPath:   "std.option",
			expectedModule: "Opt",
			expectedUse:    []string{"Option", "Some", "None"},
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

func TestWhereClauseParsing(t *testing.T) {
	code := `package main

func combine<T, U>(a: T, b: U): int32 where T is Addable & Scalable, U is Displayable {
    return 0
}`

	file, err := parser.Parser.ParseString("where.gecko", code)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Entries) == 0 || file.Entries[0].Method == nil {
		t.Fatal("Expected top-level method entry")
	}

	method := file.Entries[0].Method
	if method.Where == nil {
		t.Fatal("Expected where clause to be parsed")
	}
	if len(method.Where.Constraints) != 2 {
		t.Fatalf("Expected 2 where constraints, got %d", len(method.Where.Constraints))
	}

	first := method.Where.Constraints[0]
	if first.Name != "T" {
		t.Errorf("Expected constraint on T, got %q", first.Name)
	}
	if traits := first.AllTraits(); len(traits) != 2 || traits[0] != "Addable" || traits[1] != "Scalable" {
		t.Errorf("Expected T traits [Addable Scalable], got %v", traits)
	}

	second := method.Where.Constraints[1]
	if second.Name != "U" || second.Trait != "Displayable" {
		t.Errorf("Expected constraint U is Displayable, got %q is %q", second.Name, second.Trait)
	}

	// Normalization folds the where clause into the type parameters so the rest
	// of the pipeline sees a single source of constraints.
	tokens.NormalizeWhereClauses(file)
	for _, tp := range method.TypeParams {
		if tp.Name == "T" {
			if traits := tp.AllTraits(); len(traits) != 2 || traits[0] != "Addable" || traits[1] != "Scalable" {
				t.Errorf("Expected merged T traits [Addable Scalable], got %v", traits)
			}
		}
		if tp.Name == "U" {
			if traits := tp.AllTraits(); len(traits) != 1 || traits[0] != "Displayable" {
				t.Errorf("Expected merged U traits [Displayable], got %v", traits)
			}
		}
	}
}

func TestBacktickStringLiteralParsing(t *testing.T) {
	code := "package main\n\nexternal func main(): int32 {\n    let s = `hello\nworld`\n    return 0\n}"

	file, err := parser.Parser.ParseString("backtick_string.gecko", code)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Entries) == 0 || file.Entries[0].Method == nil {
		t.Fatal("Expected top-level method entry")
	}

	mainFn := file.Entries[0].Method
	if len(mainFn.Value) == 0 || mainFn.Value[0].Field == nil {
		t.Fatal("Expected first function statement to be a field declaration")
	}

	lit := mainFn.Value[0].Field.Value.Cond.LogicalOr.LogicalOr.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary.Literal
	if lit == nil {
		t.Fatal("Expected string literal")
	}
	if lit.BacktickString == "" {
		t.Fatalf("Expected BacktickString token, got String=%q BacktickString=%q", lit.String, lit.BacktickString)
	}

	value, err := lit.StringLiteralValue()
	if err != nil {
		t.Fatalf("Expected valid backtick string literal, got error: %v", err)
	}
	if value != "hello\nworld" {
		t.Fatalf("Expected decoded value %q, got %q", "hello\nworld", value)
	}
}

func TestGlobalAssignmentParsing(t *testing.T) {
	code := `package main
let counter: int32 = 0
func main(): int32 {
    let counter: int32 = 1
    global counter = 2
    return 0
}`

	file, err := parser.Parser.ParseString("global_assignment.gecko", code)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Entries) < 2 || file.Entries[1].Method == nil {
		t.Fatalf("Expected second top-level entry to be method, got %d entries", len(file.Entries))
	}

	mainFn := file.Entries[1].Method
	if len(mainFn.Value) < 2 || mainFn.Value[1].Assignment == nil {
		t.Fatal("Expected second function statement to be assignment")
	}

	assign := mainFn.Value[1].Assignment
	if !assign.Global {
		t.Fatal("Expected assignment to be marked global")
	}
	if assign.Name != "counter" {
		t.Fatalf("Expected assignment target 'counter', got %q", assign.Name)
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
