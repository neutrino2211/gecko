package parser_test

import (
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/parser"
)

func TestSourcePreservesExactRangesAndLexicalTokens(t *testing.T) {
	content := "package main\nfunc first(): int32 { return 1 }\n\n\nfunc second(): int32 { return 2 }\n"
	source, err := parser.ParseSource("ranges.gecko", content)
	if err != nil {
		t.Fatal(err)
	}
	method := source.File.Entries[0].Method
	end := method.EndPos
	source.File.ComputeRanges()
	if method.EndPos != end {
		t.Fatalf("range changed: %v -> %v", end, method.EndPos)
	}
	if end.Offset > strings.Index(content, "func second") {
		t.Fatal("first method overlaps second")
	}
	if len(source.Tokens) == 0 || source.Tokens[0].Value != "package" {
		t.Fatal("lexical tokens missing")
	}
}

func TestSourceReturnsLexicalPrefixAndError(t *testing.T) {
	source, err := parser.ParseSource("broken.gecko", "package main\n$")
	if err == nil || len(source.Tokens) == 0 {
		t.Fatal("lexical error or valid prefix missing")
	}
}
