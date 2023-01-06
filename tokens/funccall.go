package tokens

import (
	"fmt"

	"github.com/neutrino2211/Gecko/ast"
)

func (f *FuncCall) ToCString(scope *ast.Ast) string {
	base := ""

	mth := scope.ResolveMethod(f.Function)

	if mth == nil {
		scope.ErrorScope.NewCompileTimeError("Function resolution error", fmt.Sprintf("Unable to resolve the function \"%s\"", f.Function), f.Pos)
		return base
	}

	base += mth.GetFullName()

	base += "("

	if len(f.Arguments) > 0 {
		for _, arg := range f.Arguments[:len(f.Arguments)-1] {
			base += arg.Value.ToCString(scope)
			base += ", "
		}

		base += f.Arguments[len(f.Arguments)-1].Value.ToCString(scope)
	}

	base += ")"

	return base
}
