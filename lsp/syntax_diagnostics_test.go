package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntaxDiagnosticPreservesParserOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.gecko")
	content := "package main\nfunc broken(): void { let value: = 1 }\n"
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), content)
	if err != nil {
		t.Fatalf("checking malformed source: %v", err)
	}
	want := strings.Index(content, "= 1")
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "Syntax Error") {
			if diagnostic.Range.Start.Line != 1 || int(diagnostic.Range.Start.Character) != want-strings.Index(content, "func broken") {
				t.Fatalf("syntax error position = %#v, want offset %d in %s", diagnostic, want, path)
			}
			return
		}
	}
	t.Fatalf("syntax diagnostic missing: %#v", diagnostics)
}
