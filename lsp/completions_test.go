package main

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestImportCompletions(t *testing.T) {
	// Path to the main.gecko file which imports vga
	filePath := "../examples/hello_kernel/main.gecko"

	// Simulate content with "vga." - we want completions after the dot
	content := `package test
import vga

func test(): void {
    vga.
}
`
	// Line 4 (0-indexed), col 8 (after "vga.")
	items := GetCompletions(content, filePath, 4, 8)

	if len(items) == 0 {
		t.Fatal("Expected completions for vga module, got none")
	}

	// Check that we get expected functions from vga module
	expectedFuncs := []string{"color", "entry", "putchar", "clear", "print"}
	foundFuncs := make(map[string]bool)

	for _, item := range items {
		foundFuncs[item.Label] = true
	}

	for _, expected := range expectedFuncs {
		if !foundFuncs[expected] {
			t.Errorf("Expected completion for '%s' not found", expected)
		}
	}

	// Also check for constants
	expectedConsts := []string{"WIDTH", "HEIGHT", "BLACK", "WHITE"}
	for _, expected := range expectedConsts {
		if !foundFuncs[expected] {
			t.Errorf("Expected constant completion for '%s' not found", expected)
		}
	}

	t.Logf("Found %d completions: %v", len(items), getLabels(items))
}

func TestImportCompletionsWithPrefix(t *testing.T) {
	filePath := "../examples/hello_kernel/main.gecko"

	// Simulate typing "vga.co" - should filter to "color"
	content := `package test
import vga

func test(): void {
    vga.co
}
`
	// Line 4, col 10 (after "vga.co")
	items := GetCompletions(content, filePath, 4, 10)

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

func getLabels(items []protocol.CompletionItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}
