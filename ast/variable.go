package ast

import (
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type Variable struct {
	Name       string
	Value      value.Value
	Type       string
	IsPointer  bool
	IsConst    bool
	IsExternal bool
	Parent     *Ast
}

func (v *Variable) GetFullName() string {
	cString := ""

	if v.IsExternal {
		cString = v.Name
	} else {
		cString = strings.ReplaceAll(v.Parent.FullScopeName()+"."+v.Name, ".", "__")
	}

	return cString
}

func (v *Variable) GetLLIRType(scope *Ast) *types.Type {
	repr.Println(scope.LocalContext.Types)
	return scope.ResolveLLIRType(v.Type).UnwrapOrElse(func(err error) *types.Type {
		scope.ErrorScope.NewCompileTimeError("Type Resolution Error", "unable to resolve the type '"+v.Type+"'", lexer.Position{})
		return &types.NewPointer(UnknownType.Type).ElemType
	})
}

func (v *Variable) ToCDeclaration() string {
	cString := ""

	if v.IsConst {
		cString += "const "
	}

	cString += v.Type

	if v.IsPointer {
		cString += "*"
	}

	cString += " " + v.Name

	return cString
}
