package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (c *Class) ToAst(scope *ast.Ast) *ast.Ast {
	classAst := &ast.Ast{
		Scope:  c.Name,
		Parent: scope,
	}

	classAst.Init(scope.ErrorScope)

	for _, f := range c.Fields {
		if f.Method != nil {
			classAst.Methods[f.Method.Name] = *f.Method.ToAstMethod(classAst)
		}

		if f.Field != nil {
			classAst.Variables[f.Field.Name] = *f.Field.ToAstVariable(classAst)
		}
	}

	return classAst
}
