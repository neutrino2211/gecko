// spec: spec/types.md, spec/functions.md, spec/generics.md, spec/control-flow.md

package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/semantic"
)

// CurrentSemanticProgram stores frontend semantic typing info for this compile run.
var CurrentSemanticProgram *semantic.Program

func SetSemanticProgram(p *semantic.Program) {
	CurrentSemanticProgram = p
}

func applySemanticFlowFacts(state *ast.TypeState, facts *semantic.FlowFacts) {
	if state == nil || facts == nil {
		return
	}
	for name := range facts.NonNullByName {
		state.SetNonNull(name)
	}
}
