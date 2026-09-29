package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/errors"
)

func TestSemanticCallDiagnosticCarriesCodeAndRange(t *testing.T) {
	content := "package main\nfunc add(a: int32, b: int32): int32 { return a + b }\nexternal func main(): int32 { return add(1) }\n"
	uri := pathToURI(filepath.Join(t.TempDir(), "main.gecko"))
	diagnostics, err := RunCompilerCheck(string(uri), content)
	if err != nil {
		t.Fatalf("checking call: %v", err)
	}
	for _, diagnostic := range diagnostics {
		if !strings.Contains(diagnostic.Message, "Argument Count Mismatch") {
			continue
		}
		if diagnostic.Code != errors.CodeArgumentCount {
			t.Fatalf("wrong diagnostic code: %#v", diagnostic)
		}
		start := sourceOffset(content, diagnostic.Range.Start)
		end := sourceOffset(content, diagnostic.Range.End)
		if content[start:end] != "add(1)" {
			t.Fatalf("wrong call range: %#v", diagnostic.Range)
		}
		return
	}
	t.Fatalf("missing call diagnostic: %#v", diagnostics)
}

func TestCoherenceDiagnosticCarriesCodeAndRange(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GECKO_HOME", root)
	path := filepath.Join(root, "test_sources", "compile_tests", "coherence", "inherent_foreign_type_error.gecko")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := RunCompilerCheck(string(pathToURI(path)), string(content))
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != errors.CodeCoherence {
			continue
		}
		start := sourceOffset(string(content), diagnostic.Range.Start)
		end := sourceOffset(string(content), diagnostic.Range.End)
		if !strings.HasPrefix(string(content)[start:end], "impl Point {") || !strings.Contains(diagnostic.Message, "foreign type") {
			t.Fatalf("wrong coherence diagnostic: %#v", diagnostic)
		}
		return
	}
	t.Fatalf("missing coherence diagnostic: %#v", diagnostics)
}
