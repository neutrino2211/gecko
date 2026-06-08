// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"os/exec"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

func BackendRun(b interfaces.BackendInterface, c *interfaces.BackendConfig) *exec.Cmd {
	return b.Compile(c)
}

func BackendProcessEntries(b interfaces.BackendInterface, scope *ast.Ast, entries []*tokens.Entry) {
	impls := b.GetImpls()
	if impls == nil {
		return
	}

	pipeline := newSharedLoweringPipeline(newCompatibilityEmitter(impls))
	pipeline.EmitEntries(scope, entries)
}

var Backends = map[string]interfaces.BackendInterface{
	"llvm": &LLVMBackend{},
	"c":    &CBackend{},
	"asm":  &AsmBackend{},
}
