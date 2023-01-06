package tokens

import (
	"github.com/neutrino2211/Gecko/ast"
)

func (f *Field) ToAstVariable(scope *ast.Ast) *ast.Variable {
	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    f.Mutability == "const",
		IsPointer:  f.Type.Pointer,
		Type:       f.Type.ToCString(scope),
		Value:      f.Value.ToCString(scope),
		IsExternal: false,
		Parent:     scope,
	}

	if f.Visibility == "external" {
		fieldVariable.IsExternal = true
	}

	return &fieldVariable
}

func (f *Field) ToCString(scope *ast.Ast) string {
	base := ""

	if f.Mutability == "const" {
		base += f.Mutability + " "
	}

	base += f.Type.ToCString(scope) + " "

	base += f.Name + " "

	if f.Value != nil {
		base += "= " + f.Value.ToCString(scope)
	}

	base += ";"

	return base
}
