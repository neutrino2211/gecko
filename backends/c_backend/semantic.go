// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

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
