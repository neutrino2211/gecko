package semantic

import (
	"testing"

	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

func TestDiagnosticCopiesOwnRelatedLocationsAndFixes(t *testing.T) {
	program := NewProgram(&tokens.File{})
	diagnostic := Diagnostic{
		Code:    "E0013",
		Related: []errors.RelatedLocation{{Message: "original"}},
		Fixes:   []errors.SuggestedFix{{Title: "original"}},
	}
	program.addDiagnostic(diagnostic)
	diagnostic.Related[0].Message = "changed"
	diagnostic.Fixes[0].Title = "changed"
	returned := program.Diagnostics()
	if returned[0].Related[0].Message != "original" || returned[0].Fixes[0].Title != "original" {
		t.Fatalf("program retained caller's mutable diagnostic data: %#v", returned)
	}
	returned[0].Related[0].Message = "changed"
	returned[0].Fixes[0].Title = "changed"
	cloned := program.CloneForSyntax(&tokens.File{}, map[any]any{})
	cloned.diagnostics[0].Related[0].Message = "changed"
	cloned.diagnostics[0].Fixes[0].Title = "changed"
	if actual := program.Diagnostics()[0]; actual.Related[0].Message != "original" || actual.Fixes[0].Title != "original" {
		t.Fatalf("view mutation changed cached diagnostics: %#v", actual)
	}
}
