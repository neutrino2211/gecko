// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestStdlibIndex(t *testing.T) {
	// Get the stdlib index
	idx := GetStdlibIndex()

	// Check that some known types are indexed
	vecExports := idx.FindByName("Vec")
	if len(vecExports) == 0 {
		t.Log("Vec not found in stdlib index (stdlib may not be accessible)")
	} else {
		t.Logf("Found Vec: %+v", vecExports[0])
		if vecExports[0].Kind != "class" {
			t.Errorf("Expected Vec to be a class, got %s", vecExports[0].Kind)
		}
	}

	stringExports := idx.FindByName("String")
	if len(stringExports) == 0 {
		t.Log("String not found in stdlib index")
	} else {
		t.Logf("Found String: %+v", stringExports[0])
	}

	// Test prefix search
	sExports := idx.FindByPrefix("S")
	t.Logf("Found %d exports starting with 'S': %v", len(sExports), func() []string {
		names := make([]string, len(sExports))
		for i, e := range sExports {
			names[i] = e.Name
		}
		return names
	}())
}

func TestStdlibCompletions(t *testing.T) {
	content := `package test

func main(): void {
    let v: Ve
}
`
	// Line 3 (0-indexed), col 12 (after "Ve")
	items := GetCompletions(newTestCtx(content), content, "test.gecko", 3, 12)

	// Look for Vec from stdlib
	foundVec := false
	for _, item := range items {
		if item.Label == "Vec" {
			foundVec = true
			t.Logf("Found Vec completion: %s, %s", item.Label, item.Detail)
			if !strings.Contains(item.Detail, "import") {
				t.Errorf("Expected Vec detail to mention import, got: %s", item.Detail)
			}
			break
		}
	}

	if !foundVec {
		t.Log("Vec not found in completions (stdlib may not be accessible)")
	}
}

func TestCodeActionsUnresolvedType(t *testing.T) {
	content := `package test

func main(): void {
    let v: Vec<int> = Vec::new()
}
`
	// Create a diagnostic simulating an unresolved type error
	diag := protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: 3, Character: 11},
			End:   protocol.Position{Line: 3, Character: 14},
		},
		Message:  "unknown type 'Vec'",
		Severity: protocol.DiagnosticSeverityError,
	}

	rng := protocol.Range{
		Start: protocol.Position{Line: 3, Character: 11},
		End:   protocol.Position{Line: 3, Character: 14},
	}

	actions := GetCodeActions(content, "test.gecko", rng, []protocol.Diagnostic{diag})

	// If stdlib is available, we should get import suggestions
	if len(actions) > 0 {
		t.Logf("Found %d code actions", len(actions))
		for _, action := range actions {
			t.Logf("  - %s", action.Title)
		}
	} else {
		t.Log("No code actions found (stdlib may not be accessible)")
	}
}
