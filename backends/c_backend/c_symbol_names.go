// spec: spec/types.md, spec/functions.md, spec/scoping.md, spec/c-interop.md

package cbackend

import "github.com/neutrino2211/gecko/ast"

// CVariableIdentifier returns the C identifier emitted for a Gecko variable.
// Locals/arguments always use their source name; globals/externals use mangled
// fully qualified names.
func CVariableIdentifier(variable *ast.Variable) string {
	if variable == nil {
		return ""
	}
	if variable.IsGlobal || variable.IsExternal {
		return variable.GetFullName()
	}
	return variable.Name
}
