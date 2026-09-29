package compiler

import (
	"strings"
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

func TestSemanticDiagnosticMetadataReachesCollector(t *testing.T) {
	file := &tokens.File{Path: "main.gecko", Content: "value\n"}
	collector := &errors.Collector{}
	fix := errors.SuggestedFix{Title: "Replace value", Span: errors.SourceSpan{File: file.Path, Start: 0, End: 5}, NewText: "other"}
	related := errors.RelatedLocation{Span: errors.SourceSpan{File: file.Path, Start: 0, End: 5}, Message: "original"}
	emitSemanticDiagnostics(file, []semantic.Diagnostic{{
		Severity:  semantic.SeverityWarning,
		Title:     "Example",
		Message:   "check value",
		Help:      "use other",
		Pos:       lexer.Position{Filename: file.Path, Line: 1, Column: 1, Offset: 0},
		EndOffset: 5,
		Code:      "W1000",
		Related:   []errors.RelatedLocation{related},
		Fixes:     []errors.SuggestedFix{fix},
	}}, collector)
	scopes := collector.Scopes()
	if len(scopes) != 1 || len(scopes[0].CompileTimeWarnings) != 1 {
		t.Fatalf("missing semantic warning: %#v", scopes)
	}
	warning := scopes[0].CompileTimeWarnings[0]
	if warning.Help != "use other" || warning.Code != "W1000" || warning.EndOffset != 5 {
		t.Fatalf("warning lost metadata: %#v", warning)
	}
	messages := GetAllWarnings(scopes)
	if len(messages) != 1 || !strings.Contains(messages[0].Message, "help: use other") || len(messages[0].Related) != 1 || len(messages[0].Fixes) != 1 {
		t.Fatalf("warning adapter lost metadata: %#v", messages)
	}
	if messages[0].Related[0] != related || messages[0].Fixes[0] != fix {
		t.Fatalf("warning metadata changed: %#v", messages[0])
	}
}
