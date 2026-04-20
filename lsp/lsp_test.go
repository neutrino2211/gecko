package main

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

// LSPTestCase represents a single LSP test case
type LSPTestCase struct {
	Name     string
	Content  string
	Line     int // 0-indexed
	Col      int // 0-indexed
	Expected any
}

// HoverTestCase for hover-specific tests
type HoverTestCase struct {
	LSPTestCase
	ExpectedType    string
	ExpectedContain string // substring that should be in the hover text
}

// CompletionTestCase for completion-specific tests
type CompletionTestCase struct {
	LSPTestCase
	ExpectedLabels    []string // labels that should be present
	NotExpectedLabels []string // labels that should NOT be present
}

// =============================================================================
// HOVER TESTS
// =============================================================================

func TestHoverVariables(t *testing.T) {
	tests := []HoverTestCase{
		{
			LSPTestCase: LSPTestCase{
				Name: "local variable with explicit type",
				Content: `package test
func main(): int32 {
    let x: int32 = 42
    return x
}`,
				Line: 3,
				Col:  11, // on 'x' in 'return x'
			},
			ExpectedContain: "int32",
		},
		{
			LSPTestCase: LSPTestCase{
				Name: "function parameter",
				Content: `package test
func add(a: int32, b: int32): int32 {
    return a
}`,
				Line: 2,
				Col:  11, // on 'a' in 'return a'
			},
			ExpectedContain: "parameter",
		},
		{
			LSPTestCase: LSPTestCase{
				Name: "boolean literal inference",
				Content: `package test
func main(): bool {
    let flag: bool = true
    return flag
}`,
				Line: 3,
				Col:  11, // on 'flag'
			},
			ExpectedContain: "bool",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			info := GetHoverInfo(tc.Content, tc.Line, tc.Col)
			if info == nil {
				t.Fatalf("Expected hover info, got nil")
			}
			if tc.ExpectedContain != "" && !strings.Contains(info.Type, tc.ExpectedContain) {
				t.Errorf("Hover type %q does not contain %q", info.Type, tc.ExpectedContain)
			}
		})
	}
}

func TestHoverFunctions(t *testing.T) {
	tests := []HoverTestCase{
		{
			LSPTestCase: LSPTestCase{
				Name: "function definition",
				Content: `package test
func add(a: int32, b: int32): int32 {
    return a
}`,
				Line: 1,
				Col:  5, // on 'add'
			},
			ExpectedContain: "func",
		},
		{
			LSPTestCase: LSPTestCase{
				Name: "void function",
				Content: `package test
func greet(): void {
    return
}`,
				Line: 1,
				Col:  5, // on 'greet'
			},
			ExpectedContain: "void",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			info := GetHoverInfo(tc.Content, tc.Line, tc.Col)
			if info == nil {
				t.Fatalf("Expected hover info, got nil")
			}
			if tc.ExpectedContain != "" && !strings.Contains(info.Type, tc.ExpectedContain) {
				t.Errorf("Hover type %q does not contain %q", info.Type, tc.ExpectedContain)
			}
		})
	}
}

func TestHoverClasses(t *testing.T) {
	tests := []HoverTestCase{
		{
			LSPTestCase: LSPTestCase{
				Name: "class field",
				Content: `package test
class Point {
    let x: int32
    let y: int32
}`,
				Line: 2,
				Col:  8, // on 'x'
			},
			ExpectedContain: "int32",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			info := GetHoverInfo(tc.Content, tc.Line, tc.Col)
			if info == nil {
				t.Fatalf("Expected hover info, got nil")
			}
			if tc.ExpectedContain != "" && !strings.Contains(info.Type, tc.ExpectedContain) {
				t.Errorf("Hover type %q does not contain %q", info.Type, tc.ExpectedContain)
			}
		})
	}
}

// =============================================================================
// COMPLETION TESTS
// =============================================================================

func TestCompletionKeywords(t *testing.T) {
	tests := []CompletionTestCase{
		{
			LSPTestCase: LSPTestCase{
				Name: "top level keywords",
				Content: `package test
f`,
				Line: 1,
				Col:  1, // after 'f'
			},
			ExpectedLabels: []string{"func"},
		},
		{
			LSPTestCase: LSPTestCase{
				Name: "in function body",
				Content: `package test
func main(): void {
    l
}`,
				Line: 2,
				Col:  5, // after 'l'
			},
			ExpectedLabels: []string{"let"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			items := GetCompletions(tc.Content, "test.gecko", tc.Line, tc.Col)
			labels := getCompletionLabels(items)

			for _, expected := range tc.ExpectedLabels {
				if !containsLabel(labels, expected) {
					t.Errorf("Expected completion %q not found in %v", expected, labels)
				}
			}

			for _, notExpected := range tc.NotExpectedLabels {
				if containsLabel(labels, notExpected) {
					t.Errorf("Unexpected completion %q found in %v", notExpected, labels)
				}
			}
		})
	}
}

func TestCompletionInstanceMembers(t *testing.T) {

	tests := []CompletionTestCase{
		{
			LSPTestCase: LSPTestCase{
				Name: "struct field access",
				Content: `package test
class Point {
    let x: int32
    let y: int32
}

func main(): void {
    let p: Point
    p.
}`,
				Line: 8,
				Col:  6, // after 'p.'
			},
			ExpectedLabels: []string{"x", "y"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			items := GetCompletions(tc.Content, "test.gecko", tc.Line, tc.Col)
			labels := getCompletionLabels(items)

			for _, expected := range tc.ExpectedLabels {
				if !containsLabel(labels, expected) {
					t.Errorf("Expected completion %q not found in %v", expected, labels)
				}
			}
		})
	}
}

func TestCompletionStaticMethods(t *testing.T) {

	tests := []CompletionTestCase{
		{
			LSPTestCase: LSPTestCase{
				Name: "static method call",
				Content: `package test
class Point {
    let x: int32
    let y: int32

    func new(x: int32, y: int32): Point {
        let p: Point
        return p
    }
}

func main(): void {
    Point::
}`,
				Line: 12,
				Col:  11, // after 'Point::'
			},
			ExpectedLabels: []string{"new"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			items := GetCompletions(tc.Content, "test.gecko", tc.Line, tc.Col)
			labels := getCompletionLabels(items)

			for _, expected := range tc.ExpectedLabels {
				if !containsLabel(labels, expected) {
					t.Errorf("Expected completion %q not found in %v", expected, labels)
				}
			}
		})
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func getCompletionLabels(items []protocol.CompletionItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}

func containsLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

// =============================================================================
// FIXTURE-BASED TESTS (for future expansion)
// =============================================================================

// TestFixture represents a test defined in a .gecko.test file
type TestFixture struct {
	FilePath string
	Content  string
	Tests    []FixtureTest
}

type FixtureTest struct {
	Type     string // "hover", "completion", "definition"
	Line     int
	Col      int
	Expected string
}

// ParseTestFixture parses a .gecko.test file
// Format:
// ```gecko
// let x: int32 = 42
// //  ^ hover: "x: int32"
// ```
func ParseTestFixture(content string) *TestFixture {
	// TODO: Implement fixture parsing
	// Look for lines with "^ hover:", "^ completion:", "^ definition:"
	// Extract position from ^ marker and expected value from after :
	return nil
}
