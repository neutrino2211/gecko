package ast

import (
	"strings"

	"github.com/neutrino2211/gecko/codegen"
)

type Method struct {
	Name       string
	Arguments  []Variable
	Scope      *Ast
	Visibility string
	Parent     *Ast
	Type       string
	Context    *codegen.LocalContext
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
