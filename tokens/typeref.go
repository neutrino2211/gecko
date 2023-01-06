package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (t *TypeRef) ToCString(scope *ast.Ast) string {
	base := ""

	if t.Array != nil {
		base += t.Array.ToCString(scope) + "*"
	} else {
		base += t.Type
	}

	if t.Size != nil {
		base += t.Size.Type.ToCString(scope) + "[" + t.Size.Size + "]"
	}

	if t.Pointer {
		base += "*"
	}

	return base
}
