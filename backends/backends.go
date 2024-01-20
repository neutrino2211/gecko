package backends

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/urfave/cli/v2"
)

func streamPipe(std io.ReadCloser) {
	buf := bufio.NewReader(std) // Notice that this is not in a loop
	for {

		line, _, err := buf.ReadLine()
		if err != nil {
			break
		}
		fmt.Println(string(line))
	}
}

func streamCommand(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	streamPipe(stdout)
	streamPipe(stderr)

	return nil
}

type Backend struct {
	Ast       *ast.Ast
	CompileFn func(*Backend, *BackendConfig) *exec.Cmd
	Impls     BackendCodegenImplementations
}

type BackendConfig struct {
	File    string
	OutName string
	Ctx     *cli.Context
}

type BackendCodegenImplementations interface {
	NewReturn(*ast.Ast)
	NewReturnLiteral(*ast.Ast, *tokens.Expression)

	FuncCall(*ast.Ast, *tokens.FuncCall)
	Declaration(*ast.Ast, *tokens.Declaration)

	NewVariable(*ast.Ast, *tokens.Field)
	NewMethod(*ast.Ast, *tokens.Method)

	ParseExpression(*ast.Ast, *tokens.Expression)
}

func (b *Backend) Run(c *BackendConfig) *exec.Cmd {
	return b.CompileFn(b, c)
}

var Backends = map[string]*Backend{
	"llvm": &LLVMBackend,
}
