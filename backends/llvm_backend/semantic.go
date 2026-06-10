// spec: spec/types.md, spec/functions.md, spec/generics.md, spec/control-flow.md

package llvmbackend

import "github.com/neutrino2211/gecko/semantic"

// CurrentSemanticProgram stores frontend semantic typing info for this compile run.
var CurrentSemanticProgram *semantic.Program

func SetSemanticProgram(p *semantic.Program) {
	CurrentSemanticProgram = p
}
