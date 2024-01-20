package backends

import (
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/participle/lexer"
	llvmbackend "github.com/neutrino2211/gecko/backends/llvm_backend"
)

var LLVMBackend = Backend{
	Ast:   nil,
	Impls: &llvmbackend.LLVMBackendImplementation{},
	CompileFn: func(b *Backend, c *BackendConfig) *exec.Cmd {

		info := llvmbackend.LLVMGetScopeInformation(b.Ast)

		llir := info.ProgramContext.Module.String()

		if c.Ctx.Bool("print-ir") {
			println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + llir)
		}

		os.WriteFile(c.OutName, []byte(llir), 0o755)

		if c.Ctx.Bool("ir-only") {
			cmd := exec.Command("cp", c.OutName, ".")
			err := streamCommand(cmd)

			if err != nil {
				b.Ast.ErrorScope.NewCompileTimeError("LLIR copy", "Error copying LLVM IR "+err.Error(), lexer.Position{})
			}

			return nil
		}

		llcArgs := []string{"-filetype=obj"}

		if b.Ast.Config.Arch == "arm64" && b.Ast.Config.Platform == "darwin" {
			llcArgs = append(llcArgs, "--mtriple", "arm64-apple-darwin21.4.0")
		} else if b.Ast.Config.Vendor != "" {
			llcArgs = append(llcArgs, "--mtriple", b.Ast.Config.Arch+"-"+b.Ast.Config.Vendor+"-"+b.Ast.Config.Platform)
		}

		llcArgs = append(llcArgs, b.Ast.Config.Ctx.String("output")+"/"+c.OutName)

		cmd := exec.Command("llc", llcArgs...)

		return cmd
	},
}
