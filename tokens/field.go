// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

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

func (f *Field) IsConstBinding() bool {
	if f == nil {
		return false
	}
	if f.Mutability == "const" {
		return true
	}
	if f.Type == nil || f.Type.Pointer {
		return false
	}
	if f.Type.Const {
		return true
	}
	return f.Type.Size != nil && f.Type.Size.Type != nil && f.Type.Size.Type.Const
}
