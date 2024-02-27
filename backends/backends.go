package backends

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"

	"github.com/neutrino2211/gecko/ast"
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

func BackendRun(b interfaces.BackendInterface, c *interfaces.BackendConfig) *exec.Cmd {
	return b.Compile(c)
}

func BackendProcessEntries(b interfaces.BackendInterface, scope *ast.Ast, entries []*tokens.Entry) {
	for _, entry := range entries {
		if entry.Method != nil {
			println(entry.Method.Name)
			b.GetImpls().NewMethod(scope, entry.Method)
		} else if entry.Field != nil {
			b.GetImpls().NewVariable(scope, entry.Field)
		} else if entry.Class != nil {
			b.GetImpls().NewClass(scope, entry.Class)
		} else if entry.Implementation != nil {
			b.GetImpls().NewImplementation(scope, entry.Implementation)
		} else if entry.Trait != nil {
			b.GetImpls().NewTrait(scope, entry.Trait)
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
		} else if entry.FuncCall != nil {
			b.GetImpls().FuncCall(scope, entry.FuncCall)
		} else if entry.Return != nil {
			b.GetImpls().NewReturnLiteral(scope, entry.Return)
		} else if entry.VoidReturn != nil {
			b.GetImpls().NewReturn(scope)
		}
	}
}

var Backends = map[string]interfaces.BackendInterface{
	"llvm":  &LLVMBackend{},
	"clang": &CBackend{},
}
