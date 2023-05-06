package ast

import (
	"strings"

	"github.com/llir/llvm/ir/types"
)

type Variable struct {
	Name       string
	Value      string
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
	return scope.ResolveLLIRType(v.Type).Unwrap()
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

	if v.Value != "" {
		cString += " = " + v.Value
	}

	return cString
}
