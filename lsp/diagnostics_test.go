// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"strings"
	"testing"

	"go.lsp.dev/protocol"
)

func TestDiagnosticsInheritedTraitMissingParent(t *testing.T) {
	content := `package test

trait Child: MissingParent {
    func value(self): int32
}
`

	diagnostics, err := RunCompilerCheck("file:///tmp/missing_parent.gecko", content)
	if err != nil {
		t.Fatalf("RunCompilerCheck failed: %v", err)
	}

	if !hasDiagnosticContaining(diagnostics, "Could not resolve parent trait") {
		t.Fatalf("Expected inherited-trait diagnostic mentioning unresolved parent, got: %#v", diagnostics)
	}
}

func TestDiagnosticsInheritedTraitOverrideConflict(t *testing.T) {
	content := `package test

trait Parent {
    func value(self): int32
}

trait Child: Parent {
    func value(self): bool
}
`

	diagnostics, err := RunCompilerCheck("file:///tmp/override_conflict.gecko", content)
	if err != nil {
		t.Fatalf("RunCompilerCheck failed: %v", err)
	}

	if !hasDiagnosticContaining(diagnostics, "conflicts with inherited method") {
		t.Fatalf("Expected inherited-trait override conflict diagnostic, got: %#v", diagnostics)
	}

	if !hasDiagnosticContaining(diagnostics, "Parent.value") {
		t.Fatalf("Expected override conflict diagnostic to include parent method origin, got: %#v", diagnostics)
	}
}

func hasDiagnosticContaining(diagnostics []protocol.Diagnostic, substr string) bool {
	for _, diag := range diagnostics {
		if strings.Contains(diag.Message, substr) {
			return true
		}
	}
	return false
}
