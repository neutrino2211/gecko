// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"os/exec"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

// AsmBackend is a minimal backend for generating assembly
// It only supports core features + freestanding (inline asm, naked functions, etc.)
type AsmBackend struct {
	features *FeatureSet
}

func (b *AsmBackend) Init() {
	b.features = NewAsmFeatureSet()
}

func (b *AsmBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	// Minimal processing - ASM backend doesn't need full codegen
}

func (b *AsmBackend) GetImpls() interfaces.BackendCodegenImplementations {
	// ASM backend doesn't use the standard codegen interface
	return nil
}

func (b *AsmBackend) Features() interfaces.FeatureChecker {
	return b.features
}

func (b *AsmBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	scope := c.Diagnostics.NewScope("compile", c.File, c.SourceFile.Content)
	scope.NewCompileTimeError("Backend Error", "ASM backend compilation is not yet implemented", lexer.Position{Line: 1, Column: 1})
	return nil
}
