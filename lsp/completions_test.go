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
	items := GetCompletions(content, filePath, 4, 15)

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
	items := GetCompletions(content, filePath, 4, 17)

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
	items := GetCompletions(content, filePath, 5, 6)

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

func getLabels(items []protocol.CompletionItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}
