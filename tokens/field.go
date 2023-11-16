package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (f *Field) ToAstVariable(scope *ast.Ast) *ast.Variable {
	if f.Type == nil {
		scope.ErrorScope.NewCompileTimeError("TODO: Infer variable type", "variable type inference not implemented", f.Pos)
		f.Type = &TypeRef{}
	}

	if f.Value == nil && f.Type.Const {
		scope.ErrorScope.NewCompileTimeError("Uninitialesed constant", "constant must be initialised with a value", f.Pos)
		f.Value = &Expression{}
	}

	val := f.Value.ToLLIRValue(scope, f.Type)

	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    f.Type.Const,
		IsPointer:  f.Type.Pointer,
		Type:       f.Type.Type,
		Value:      val,
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
