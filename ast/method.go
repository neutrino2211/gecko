package ast

import "strings"

type MethodArgument struct {
	Name      string
	Type      string
	IsPointer bool
}

type Method struct {
	Name       string
	Arguments  []MethodArgument
	Scope      *Ast
	Visibility string
	Parent     *Ast
	Type       string
}

func (m *Method) GetFullName() string {
	cString := ""

	if m.Visibility == "external" {
		cString = m.Name
	} else {
		cString = strings.ReplaceAll(m.Parent.FullScopeName()+"."+m.Name, ".", "__")
	}

	return cString
}

func (m *Method) ToCString() string {
	content := m.Scope.ToCString()

	return m.Type + " " + m.Name + "() {\n" + content + "}"
}
