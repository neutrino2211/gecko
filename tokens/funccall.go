package tokens

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
)

func getTypeRefFromString(name string) *TypeRef {
	return &TypeRef{
		Type: name,
	}
}

func (f *FuncCall) ToCString(scope *ast.Ast) string {
	base := ""

	mth := scope.ResolveMethod(f.Function)

	if mth.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Function resolution error", fmt.Sprintf("Unable to resolve the function \"%s\"", f.Function), f.Pos)
		return base
	}

	base += mth.Unwrap().GetFullName()

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
