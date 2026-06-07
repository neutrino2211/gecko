// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir"
	"github.com/neutrino2211/gecko/ast"
	llvmbackend "github.com/neutrino2211/gecko/backends/llvm_backend"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type LLVMBackend struct {
	impls    interfaces.BackendCodegenImplementations
	features *FeatureSet
}

func (b *LLVMBackend) Init() {
	b.impls = &llvmbackend.LLVMBackendImplementation{Backend: b}
	b.features = NewLLVMFeatureSet()
	llvmbackend.CurrentBackend = b
	llvmbackend.FuncCalls = make(map[string]*ir.InstCall)
	llvmbackend.Methods = make(map[string]*ast.Method)
}

func (b *LLVMBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	BackendProcessEntries(b, scope, entries)
}

func (b *LLVMBackend) GetImpls() interfaces.BackendCodegenImplementations {
	return b.impls
}

func (b *LLVMBackend) Features() interfaces.FeatureChecker {
	return b.features
}

func (b *LLVMBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	sharedResult := PrepareSharedCompilePipeline(b, c, SharedCompilePipelineOptions{
		ProcessImports:            true,
		MarkImportedModules:       true,
		TrackLazyResolvedAsImport: false,
	})
	file := sharedResult.RootScope
	importScopes := sharedResult.ImportScopes

	for _, importScope := range importScopes {
		if importScope.ErrorScope.HasErrors() {
			return nil
		}
	}

	// Check for errors - bail early if any
	if file.ErrorScope.HasErrors() {
		return nil
	}

	info := llvmbackend.LLVMGetScopeInformation(file)

	llir := info.ProgramContext.Module.String()

	if c.Ctx.Bool("print-ir") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + llir)
	}

	if err := os.MkdirAll(filepath.Dir(c.OutName), 0o755); err != nil {
		file.ErrorScope.NewCompileTimeError("LLVM output", "Error creating LLVM output directory "+err.Error(), lexer.Position{})
		return nil
	}
	if err := os.WriteFile(c.OutName, []byte(llir), 0o755); err != nil {
		file.ErrorScope.NewCompileTimeError("LLVM output", "Error writing LLVM IR "+err.Error(), lexer.Position{})
		return nil
	}

	if c.Ctx.Bool("ir-only") {
		return nil
	}

	objOut := strings.TrimSuffix(c.OutName, filepath.Ext(c.OutName)) + ".o"
	llcArgs := []string{"-filetype=obj"}

	if file.Config.Arch == "arm64" && file.Config.Platform == "darwin" {
		llcArgs = append(llcArgs, "--mtriple", "arm64-apple-darwin21.4.0")
	} else if file.Config.Vendor != "" {
		llcArgs = append(llcArgs, "--mtriple", file.Config.Arch+"-"+file.Config.Vendor+"-"+file.Config.Platform)
	}

	if c.Ctx != nil {
		llcArgs = append(llcArgs, c.Ctx.StringSlice("llc-args")...)
	}

	llcArgs = append(llcArgs, "-o", objOut, c.OutName)

	cmd := exec.Command("llc", llcArgs...)

	return cmd
}
