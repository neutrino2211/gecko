// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import "github.com/neutrino2211/gecko/ast"

// CVariableIdentifier returns the C identifier emitted for a Gecko variable.
// Locals/arguments always use their source name; globals/externals use mangled
// fully qualified names.
func CVariableIdentifier(variable *ast.Variable) string {
	if variable == nil {
		return ""
	}
	if variable.CName != "" {
		return variable.CName
	}
	if variable.IsGlobal || variable.IsExternal {
		return variable.GetFullName()
	}
	return variable.Name
}
