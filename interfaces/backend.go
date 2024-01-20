package interfaces

import (
	"os/exec"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/urfave/cli/v2"
)

type BackendInteface interface {
	Init()
	Compile(*BackendConfig) *exec.Cmd
	GetImpls() BackendCodegenImplementations
	ProcessEntries([]*tokens.Entry, *ast.Ast)
}

type BackendConfig struct {
	File       string
	OutName    string
	Ctx        *cli.Context
	SourceFile *tokens.File
}

type BackendCodegenImplementations interface {
	NewReturn(*ast.Ast)
	NewReturnLiteral(*ast.Ast, *tokens.Expression)

	FuncCall(*ast.Ast, *tokens.FuncCall)
	Declaration(*ast.Ast, *tokens.Declaration)

	NewVariable(*ast.Ast, *tokens.Field)
	NewMethod(*ast.Ast, *tokens.Method)

	ParseExpression(*ast.Ast, *tokens.Expression)

	ProcessEntries(*ast.Ast, []*tokens.Entry)

	NewClass(*ast.Ast, *tokens.Class)
	NewDeclaration(*ast.Ast, *tokens.Declaration)
	NewImplementation(*ast.Ast, *tokens.Implementation)
	NewTrait(*ast.Ast, *tokens.Trait)
}
