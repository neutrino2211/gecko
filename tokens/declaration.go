package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (d *Declaration) ToAstVariable(scope *ast.Ast) *ast.Variable {
	if d.Field == nil {
		return nil
	}

	declaredVariable := d.Field.ToAstVariable(scope)

	if declaredVariable.Value != "" {
		// Push an error, declared variables must not have a value
		scope.ErrorScope.NewCompileTimeError("Declaration Error", "A variable declaration must not have a value", d.Field.Value.Pos)
	}

	return declaredVariable
}

func (d *Declaration) ToAstMethod(scope *ast.Ast) *ast.Method {
	if d.Method == nil {
		return nil
	}

	declaredMethod := d.Method.ToAstMethod(scope)

	if declaredMethod.Scope != nil {
		// Push an error, declared functions must not have a scope
		scope.ErrorScope.NewCompileTimeError("Declaration Error", "A method declaration must not have a body", d.Method.Pos)
	}

	return declaredMethod
}
