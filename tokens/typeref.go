package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (t *TypeRef) ToCString(scope *ast.Ast) string {
	base := ""

	if t.Const {
		base += "const "
	}

	if t.Array != nil {
		base += t.Array.ToCString(scope) + "*"
	} else {
		classAstOpt := scope.ResolveClass(t.Type)

		if classAstOpt.IsNil() {
			base += t.Type
		} else {
			base += classAstOpt.Unwrap().GetFullName()
		}
	}

	if t.Size != nil {
		base += t.Size.Type.ToCString(scope)
	}

	if t.Pointer {
		base += "*"
	}

	return base
}

func (s *SizeDef) ToCString() string {
	return "[" + s.Size + "]"
}
