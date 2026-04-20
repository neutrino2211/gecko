package backends

import (
	"os/exec"

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
	// TODO: Implement ASM generation
	// For now, this is a stub to demonstrate feature validation
	println("ASM backend compilation not yet implemented")
	return nil
}
