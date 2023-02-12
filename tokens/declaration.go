package tokens

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/go-option"
)

func (d *Declaration) ToAstVariable(scope *ast.Ast) option.Optional[*ast.Variable] {
	if d.Field == nil {
		return option.None[*ast.Variable]()
	}

	declaredVariable := d.Field.ToAstVariable(scope)

	if declaredVariable.Value != "" {
		// Push an error, declared variables must not have a value
		scope.ErrorScope.NewCompileTimeError("Declaration Error", "A variable declaration must not have a value", d.Field.Value.Pos)
	}

	return option.Some(declaredVariable)
}

func (d *Declaration) ToAstMethod(scope *ast.Ast) option.Optional[*ast.Method] {
	if d.Method == nil {
		return option.None[*ast.Method]()
	}

	declaredMethod := d.Method.ToAstMethod(scope)

	if declaredMethod.Scope != nil {
		// Push an error, declared functions must not have a scope
		scope.ErrorScope.NewCompileTimeError("Declaration Error", "A method declaration must not have a body", d.Method.Pos)
	}

	return option.Some(declaredMethod)
}
