package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (t *TypeRef) ToCString(scope *ast.Ast) string {
	base := ""

	if t.Array != nil {
		base += t.Array.ToCString(scope) + "*"
	} else if t.FuncType != nil {
		// Function pointer type: return_type (*)(param_types)
		// For now, return a placeholder - proper handling in C backend
		base = "void (*)()"
	} else {
		base += t.Type
	}

	if t.Size != nil {
		base += t.Size.Type.ToCString(scope) + "[" + t.Size.Size + "]"
	}

	if t.Volatile {
		base += " volatile"
	}

	if t.Pointer {
		base += "*"
	}

	return base
}

// IsVolatile returns true if this type or any nested array type is volatile
func (t *TypeRef) IsVolatile() bool {
	if t.Volatile {
		return true
	}
	if t.Array != nil {
		return t.Array.IsVolatile()
	}
	return false
}
