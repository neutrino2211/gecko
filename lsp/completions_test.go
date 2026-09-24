// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestImportCompletions(t *testing.T) {
	// Path to the main.gecko file which imports testmodule
	filePath := "testdata/main.gecko"

	// Simulate content with "testmodule." - we want completions after the dot
	content := `package test
import testmodule

func test(): void {
    testmodule.
}
`
	// Line 4 (0-indexed), col 15 (after "testmodule.")
	items := GetCompletions(newTestCtx(content), content, filePath, 4, 15)

	if len(items) == 0 {
		t.Fatal("Expected completions for testmodule, got none")
	}

	// Check that we get expected public functions from testmodule
	expectedFuncs := []string{"color", "helper"}
	foundFuncs := make(map[string]bool)

	for _, item := range items {
		foundFuncs[item.Label] = true
	}

	for _, expected := range expectedFuncs {
		if !foundFuncs[expected] {
			t.Errorf("Expected completion for '%s' not found", expected)
		}
	}

	// Also check for public constants
	expectedConsts := []string{"WIDTH", "HEIGHT"}
	for _, expected := range expectedConsts {
		if !foundFuncs[expected] {
			t.Errorf("Expected constant completion for '%s' not found", expected)
		}
	}

	// Check for public class
	if !foundFuncs["Point"] {
		t.Error("Expected completion for 'Point' class not found")
	}

	t.Logf("Found %d completions: %v", len(items), getLabels(items))
}

func TestImportCompletionsWithPrefix(t *testing.T) {
	filePath := "testdata/main.gecko"

	// Simulate typing "testmodule.co" - should filter to "color"
	content := `package test
import testmodule

func test(): void {
    testmodule.co
}
`
	// Line 4, col 17 (after "testmodule.co")
	items := GetCompletions(newTestCtx(content), content, filePath, 4, 17)

	// Should only get items starting with "co"
	for _, item := range items {
		if !strings.HasPrefix(item.Label, "co") {
			t.Errorf("Item '%s' doesn't match prefix 'co'", item.Label)
		}
	}

	// Should include "color"
	found := false
	for _, item := range items {
		if item.Label == "color" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'color' completion not found")
	}
}

func TestImportedClassMemberVisibility(t *testing.T) {
	filePath := "testdata/main.gecko"

	// Test that imported class members respect visibility
	// Point has public x, y, new, distance and private privateMethod
	content := `package test
import testmodule

func test(): void {
    let p: testmodule.Point = testmodule.Point::new(1, 2)
    p.
}
`
	// Line 5 (0-indexed), col 6 (after "p.")
	items := GetCompletions(newTestCtx(content), content, filePath, 5, 6)

	foundLabels := make(map[string]bool)
	for _, item := range items {
		foundLabels[item.Label] = true
	}

	// Should have public fields and methods
	expectedPublic := []string{"x", "y", "distance"}
	for _, expected := range expectedPublic {
		if !foundLabels[expected] {
			t.Errorf("Expected public member '%s' not found", expected)
		}
	}

	// Should NOT have private methods
	if foundLabels["privateMethod"] {
		t.Error("Private method 'privateMethod' should not be visible from another module")
	}

	t.Logf("Found %d completions: %v", len(items), getLabels(items))
}

func TestEnumVariantCompletions(t *testing.T) {
	// Test enum variant completions with ::
	content := `package test

enum Status {
    Pending
    Active
    Completed
}

func test(): void {
    let s: Status = Status::
}
`
	// Line 9 (0-indexed), col 28 (after "Status::")
	items := GetCompletions(newTestCtx(content), content, "test.gecko", 9, 28)

	foundLabels := make(map[string]bool)
	for _, item := range items {
		foundLabels[item.Label] = true
	}

	// Should have all enum variants
	expectedVariants := []string{"Pending", "Active", "Completed"}
	for _, expected := range expectedVariants {
		if !foundLabels[expected] {
			t.Errorf("Expected enum variant '%s' not found", expected)
		}
	}

	// Check that they have the right kind
	for _, item := range items {
		if item.Kind != protocol.CompletionItemKindEnumMember {
			t.Errorf("Enum variant '%s' has wrong kind: %v", item.Label, item.Kind)
		}
	}

	t.Logf("Found %d completions: %v", len(items), getLabels(items))
}

func TestEnumTypeCompletions(t *testing.T) {
	// Test enum type completions in generic context
	content := `package test

enum Color {
    Red
    Green
    Blue
}

func test(): void {
    let c: Col
}
`
	// Line 9 (0-indexed), col 14 (after "Col")
	items := GetCompletions(newTestCtx(content), content, "test.gecko", 9, 14)

	found := false
	for _, item := range items {
		if item.Label == "Color" {
			found = true
			if item.Kind != protocol.CompletionItemKindEnum {
				t.Errorf("Enum type 'Color' has wrong kind: %v", item.Kind)
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected 'Color' enum type completion not found. Got: %v", getLabels(items))
	}
}

func TestImportedEnumCompletions(t *testing.T) {
	filePath := "testdata/main.gecko"

	// Test that imported enums appear in module completions
	content := `package test
import testmodule

func test(): void {
    testmodule.
}
`
	// Line 4 (0-indexed), col 15 (after "testmodule.")
	items := GetCompletions(newTestCtx(content), content, filePath, 4, 15)

	found := false
	for _, item := range items {
		if item.Label == "Color" {
			found = true
			if item.Kind != protocol.CompletionItemKindEnum {
				t.Errorf("Enum 'Color' has wrong kind: %v", item.Kind)
			}
			break
		}
	}

	if !found {
		t.Error("Expected 'Color' enum from testmodule not found")
	}

	t.Logf("Found %d completions: %v", len(items), getLabels(items))
}

func TestNestedScopeCompletions(t *testing.T) {
	// Test that variables in nested blocks are correctly scoped
	content := `package test

func test(): void {
    let outer: int = 1
    if true {
        let inner: int = 2

    }
    let after: int = 3
}
`
	// Test inside the if block - should see outer, inner, but not after
	// Line 6 (0-indexed), col 8 (on the empty line inside if block, after "inner")
	items := GetCompletions(newTestCtx(content), content, "test.gecko", 6, 8)

	foundLabels := make(map[string]bool)
	for _, item := range items {
		foundLabels[item.Label] = true
	}

	// Should have outer (declared before)
	if !foundLabels["outer"] {
		t.Error("Expected 'outer' variable in nested scope")
	}

	// Should have inner (declared in same block before cursor)
	if !foundLabels["inner"] {
		t.Error("Expected 'inner' variable in nested scope")
	}

	// Should NOT have after (declared after the if block)
	if foundLabels["after"] {
		t.Error("'after' should not be visible inside if block (declared later)")
	}

	t.Logf("Found completions inside if block: %v", getLabels(items))
}

func TestWhileLoopVariableCompletions(t *testing.T) {
	// Test that variables in while loops are correctly scoped
	content := `package test

func test(): void {
    let outer: int = 5
    while outer > 0 {
        let inner: int = outer

    }
}
`
	// Line 0: package test
	// Line 1: (empty)
	// Line 2: func test(): void {
	// Line 3:     let outer: int = 5
	// Line 4:     while outer > 0 {
	// Line 5:         let inner: int = outer
	// Line 6: (empty)
	// Line 7:     }
	// Line 8: }
	items := GetCompletions(newTestCtx(content), content, "test.gecko", 6, 8)

	foundLabels := make(map[string]bool)
	for _, item := range items {
		foundLabels[item.Label] = true
	}

	t.Logf("Completions: %v", getLabels(items))

	// Should have 'outer' from outer scope
	if !foundLabels["outer"] {
		t.Errorf("Expected 'outer' variable in completions. Got: %v", getLabels(items))
	}

	// Should have 'inner' declared in the loop
	if !foundLabels["inner"] {
		t.Errorf("Expected 'inner' variable in completions. Got: %v", getLabels(items))
	}
}

func TestGenericTypeCompletions(t *testing.T) {
	// Test that generic type completions substitute type parameters
	content := `package test

class Container<T> {
    let value: T
}

impl Container {
    func get(self): T {
        return self.value
    }

    func set(self, val: T): void {
        self.value = val
    }
}

func test(): void {
    let c: Container<int> = Container { value: 42 }
    c.
}
`
	// Line 18 (0-indexed), col 6 (after "c.")
	items := GetCompletions(newTestCtx(content), content, "test.gecko", 18, 6)

	if len(items) == 0 {
		t.Fatal("Expected completions for Container<int>, got none")
	}

	// Check that methods have 'int' substituted for 'T'
	foundGet := false
	foundSet := false
	foundValue := false

	for _, item := range items {
		t.Logf("Item: %s, Detail: %s", item.Label, item.Detail)
		switch item.Label {
		case "get":
			foundGet = true
			// Detail should show return type as 'int', not 'T'
			if !strings.Contains(item.Detail, "int") || strings.Contains(item.Detail, "T") {
				t.Errorf("'get' method detail should have 'int' substituted for 'T', got: %s", item.Detail)
			}
		case "set":
			foundSet = true
			// Detail should show parameter type as 'int', not 'T'
			if !strings.Contains(item.Detail, "int") || strings.Contains(item.Detail, "T") {
				t.Errorf("'set' method detail should have 'int' substituted for 'T', got: %s", item.Detail)
			}
		case "value":
			foundValue = true
			// Field type should be 'int', not 'T'
			if item.Detail != "int" {
				t.Errorf("'value' field should have type 'int', got: %s", item.Detail)
			}
		}
	}

	if !foundGet {
		t.Error("Expected 'get' method completion not found")
	}
	if !foundSet {
		t.Error("Expected 'set' method completion not found")
	}
	if !foundValue {
		t.Error("Expected 'value' field completion not found")
	}
}

func TestTraitInheritanceMethodCompletions(t *testing.T) {
	content := `package test

trait Parent {
    func base(self): int32

    func doubled(self): int32 {
        return self.base() * 2
    }
}

trait Child: Parent {
    func tripled(self): int32 {
        return self.base() * 3
    }
}

class Counter {
    let n: int32
}

impl Counter {
    func base(self): int32 {
        return self.n
    }
}

default impl Child for Counter

func demo(): void {
    let c: Counter = Counter { n: 7 }
    c.
}
`

	dotIdx := strings.Index(content, "c.")
	if dotIdx == -1 {
		t.Fatal("failed to locate member access for completion test")
	}
	wordIdx := dotIdx + 2 // position after "c."
	line := strings.Count(content[:wordIdx], "\n")
	lastNewline := strings.LastIndex(content[:wordIdx], "\n")
	col := wordIdx
	if lastNewline >= 0 {
		col = wordIdx - lastNewline - 1
	}

	items := GetCompletions(newTestCtx(content), content, "test.gecko", line, col)

	found := make(map[string]bool)
	for _, item := range items {
		found[item.Label] = true
	}

	expected := []string{"base", "doubled", "tripled"}
	for _, method := range expected {
		if !found[method] {
			t.Errorf("Expected inherited trait method completion '%s' not found. Got: %v", method, getLabels(items))
		}
	}
}

func getLabels(items []protocol.CompletionItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}
