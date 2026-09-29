package main

import (
	"path/filepath"
	"testing"

	"github.com/neutrino2211/gecko/errors"
	"go.lsp.dev/protocol"
)

func TestMissingImportDiagnosticHighlightsImportPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nimport absent.module\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking missing import: %v", err)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != errors.CodeImportNotFound {
			continue
		}
		want := protocol.Range{Start: protocol.Position{Line: 1, Character: 7}, End: protocol.Position{Line: 1, Character: 20}}
		if diagnostic.Range != want {
			t.Fatalf("missing import range = %#v, want %#v", diagnostic.Range, want)
		}
		return
	}
	t.Fatalf("missing import diagnostic: %#v", diagnostics)
}
