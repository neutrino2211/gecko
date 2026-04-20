package backends

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
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

func BackendRun(b interfaces.BackendInteface, c *interfaces.BackendConfig) *exec.Cmd {
	return b.Compile(c)
}

func BackendProcessEntries(b interfaces.BackendInteface, scope *ast.Ast, entries []*tokens.Entry) {
	for _, entry := range entries {
		if entry.Method != nil {
			b.GetImpls().NewMethod(scope, entry.Method)
		} else if entry.Field != nil {
			b.GetImpls().NewVariable(scope, entry.Field)
		} else if entry.Class != nil {
			b.GetImpls().NewClass(scope, entry.Class)
		} else if entry.Implementation != nil {
			b.GetImpls().NewImplementation(scope, entry.Implementation)
		} else if entry.Trait != nil {
			// Process any hook attributes on the trait
			hooks.ProcessTraitHooks(entry.Trait, scope.Scope, scope.ErrorScope)
			b.GetImpls().NewTrait(scope, entry.Trait)
		} else if entry.Enum != nil {
			b.GetImpls().NewEnum(scope, entry.Enum)
		} else if entry.Declaration != nil {
			// if entry.Declaration.Field != nil {
			// 	variableOpt := entry.Declaration.ToAstVariable(scope)
			// 	scope.Variables[entry.Declaration.Field.Name] = *variableOpt.UnwrapOrElse(func(err error) *ast.Variable {
			// 		scope.ErrorScope.NewCompileTimeError(
			// 			"Parse Error",
			// 			"Unable to parse the variable named '"+entry.Declaration.Field.Name+"'",
			// 			entry.Pos,
			// 		)
			// 		return &ast.Variable{}
			// 	})
			// } else if entry.Declaration.Method != nil {
			// 	methodOpt := entry.Declaration.ToAstMethod(scope)
			// 	scope.Methods[entry.Declaration.Method.Name] = methodOpt.UnwrapOrElse(func(err error) *ast.Method {
			// 		scope.ErrorScope.NewCompileTimeError(
			// 			"Parse Error",
			// 			"Unable to parse the method named '"+entry.Declaration.Method.Name+"'",
			// 			entry.Pos,
			// 		)

			// 		return &ast.Method{}
			// 	})
			// }
			b.GetImpls().NewDeclaration(scope, entry.Declaration)
		} else if entry.Intrinsic != nil {
			b.GetImpls().IntrinsicStatement(scope, entry.Intrinsic)
		} else if entry.MethodCall != nil {
			b.GetImpls().MethodCall(scope, entry.MethodCall)
		} else if entry.FuncCall != nil {
			b.GetImpls().FuncCall(scope, entry.FuncCall)
		} else if entry.Return != nil {
			b.GetImpls().NewReturnLiteral(scope, entry.Return)
		} else if entry.VoidReturn != nil {
			b.GetImpls().NewReturn(scope)
		} else if entry.If != nil {
			b.GetImpls().NewIf(scope, entry.If)
		} else if entry.Loop != nil {
			b.GetImpls().NewLoop(scope, entry.Loop)
		} else if entry.Assignment != nil {
			b.GetImpls().NewAssignment(scope, entry.Assignment)
		} else if entry.Asm != nil {
			b.GetImpls().NewAsm(scope, entry.Asm)
		} else if entry.Break != nil {
			b.GetImpls().NewBreak(scope)
		} else if entry.Continue != nil {
			b.GetImpls().NewContinue(scope)
		} else if entry.CImport != nil {
			b.GetImpls().NewCImport(scope, entry.CImport)
		}
	}
}

var Backends = map[string]interfaces.BackendInteface{
	"llvm": &LLVMBackend{},
	"c":    &CBackend{},
	"asm":  &AsmBackend{},
}
