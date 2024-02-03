package backends

import (
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/llir/llvm/ir"
	"github.com/neutrino2211/gecko/ast"
	llvmbackend "github.com/neutrino2211/gecko/backends/llvm_backend"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type LLVMBackend struct {
	impls interfaces.BackendCodegenImplementations
}

func (b *LLVMBackend) Init() {
	b.impls = &llvmbackend.LLVMBackendImplementation{Backend: b}
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

func (b *LLVMBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	file := &ast.Ast{
		Scope: c.SourceFile.PackageName,
	}

	file.Init(errors.NewErrorScope(c.SourceFile.Name, c.SourceFile.Path, c.SourceFile.Content))
	file.Config = c.SourceFile.Config

	b.ProcessEntries(c.SourceFile.Entries, file)

	info := llvmbackend.LLVMGetScopeInformation(file)

	llir := info.ProgramContext.Module.String()

	if c.Ctx.Bool("print-ir") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + llir)
	}

	os.WriteFile(c.OutName, []byte(llir), 0o755)

	if c.Ctx.Bool("ir-only") {
		cmd := exec.Command("cp", c.OutName, ".")
		err := streamCommand(cmd)

		if err != nil {
			file.ErrorScope.NewCompileTimeError("LLIR copy", "Error copying LLVM IR "+err.Error(), lexer.Position{})
		}

		return nil
	}

	llcArgs := []string{"-filetype=obj"}

	if file.Config.Arch == "arm64" && file.Config.Platform == "darwin" {
		llcArgs = append(llcArgs, "--mtriple", "arm64-apple-darwin21.4.0")
	} else if file.Config.Vendor != "" {
		llcArgs = append(llcArgs, "--mtriple", file.Config.Arch+"-"+file.Config.Vendor+"-"+file.Config.Platform)
	}

	llcArgs = append(llcArgs, file.Config.Ctx.String("output")+"/"+c.OutName)

	cmd := exec.Command("llc", llcArgs...)

	return cmd
}
