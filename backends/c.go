package backends

import (
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/neutrino2211/gecko/ast"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type CBackend struct {
	impls interfaces.BackendCodegenImplementations
}

func (b *CBackend) Init() {
	b.impls = &cbackend.CBackendImplementation{Backend: b}
	cbackend.FuncCalls = make(map[string]string)
	cbackend.Methods = make(map[string]*ast.Method)
}

func (b *CBackend) ProcessEntries(entries []*tokens.Entry, scope *ast.Ast) {
	BackendProcessEntries(b, scope, entries)
}

func (b *CBackend) GetImpls() interfaces.BackendCodegenImplementations {
	return b.impls
}

func (b *CBackend) Compile(c *interfaces.BackendConfig) *exec.Cmd {
	file := &ast.Ast{
		Scope: c.SourceFile.PackageName,
	}

	file.Init(errors.NewErrorScope(c.SourceFile.Name, c.SourceFile.Path, c.SourceFile.Content))
	file.Config = c.SourceFile.Config

	b.ProcessEntries(c.SourceFile.Entries, file)

	info := cbackend.GetCScope(file)

	if c.Ctx.Bool("print-c") {
		println(c.File + "\n" + strings.Repeat("_", len(c.File)) + "\n\n" + info.GetSource())
	}

	os.WriteFile(c.OutName, []byte(info.GetSource()), 0o755)

	if c.Ctx.Bool("c-only") {
		cmd := exec.Command("cp", c.OutName, ".")
		err := streamCommand(cmd)

		if err != nil {
			file.ErrorScope.NewCompileTimeError("C copy", "Error copying C source "+err.Error(), lexer.Position{})
		}

		return nil
	}

	clangArgs := []string{"-I", config.GeckoConfig.StdLibPath + "/clang"}

	if file.Config.Arch == "arm64" && file.Config.Platform == "darwin" {
		clangArgs = append(clangArgs, "--mtriple", "arm64-apple-darwin21.4.0")
	} else if file.Config.Vendor != "" {
		clangArgs = append(clangArgs, "--mtriple", file.Config.Arch+"-"+file.Config.Vendor+"-"+file.Config.Platform)
	}

	clangArgs = append(clangArgs, "-c")
	clangArgs = append(clangArgs, file.Config.Ctx.String("output")+"/"+c.OutName)

	cmd := exec.Command("clang", clangArgs...)

	return cmd
}
