package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

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
