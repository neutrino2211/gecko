package ast

import (
	"strings"
)

type Variable struct {
	Name       string
	IsPointer  bool
	IsConst    bool
	IsVolatile bool
	IsExternal bool
	IsArgument bool
	IsGlobal   bool // Explicit flag for global variables (vs inferred from scope)
	Parent     *Ast
}

func (v *Variable) GetFullName() string {
	cString := ""

	if v.IsExternal || v.Parent == nil {
		cString = v.Name
	} else {
		cString = strings.ReplaceAll(v.Parent.FullScopeName()+"."+v.Name, ".", "__")
	}

	return cString
}

func (v *Variable) ToCDeclaration() string {
	cString := ""

	if v.IsConst {
		cString += "const "
	}

	// cString += v.Type

	if v.IsPointer {
		cString += "*"
	}

	cString += " " + v.Name

	return cString
}
