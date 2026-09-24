// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// CheckAssignmentType checks if assignment value matches variable type
func (impl *CBackendImplementation) CheckAssignmentType(a *tokens.Assignment, scope *ast.Ast) {
	if a == nil || scope.ErrorScope == nil {
		return
	}

	// `@unsafe with ... { }` as the RHS is emitted specially by NewAssignment
	// (into a temp Result holder that is then assigned in); its type is not
	// inferred by the general expression path, so skip here.
	if a.Value != nil && a.Value.GetUnsafeBlock() != nil {
		return
	}

	resolveScope := scope
	if a.Global {
		resolveScope = scope.GetRoot()
	}

	// Look up variable type
	varOpt := resolveScope.ResolveSymbolAsVariable(a.Name)
	if varOpt.IsNil() {
		if a.Global {
			scope.ErrorScope.NewCompileTimeError(
				"Assignment Error",
				"unable to resolve global variable '"+a.Name+"'",
				a.Pos,
			)
		}
		return
	}

	variable := varOpt.Unwrap()

	// Check if it's a const being reassigned (only for direct variable assignment)
	if a.Field == "" && a.Index == nil && variable.IsConst {
		scope.ErrorScope.NewCompileTimeError(
			"Constant Reassignment",
			"Cannot reassign constant '"+a.Name+"'",
			a.Pos,
		)
		return
	}

	// Get the variable's type from CProgramValues
	fullName := variable.GetFullName()
	valueInfo, hasInfo := (*CProgramValues)[fullName]
	if !hasInfo || valueInfo.GeckoType == nil {
		TypeCheckError(scope, a.Pos, "assignment to '"+a.Name+"'", "type information not available")
		return
	}

	var expectedType *tokens.TypeRef

	// Handle field assignment (e.g., circ.radius = 10)
	if a.Field != "" {
		// Get the class type
		typeName := valueInfo.GeckoType.Type
		rootScope := scope.GetRoot()
		classOpt := rootScope.ResolveClass(typeName)
		if classOpt.IsNil() {
			for _, child := range rootScope.Children {
				classOpt = child.ResolveClass(typeName)
				if !classOpt.IsNil() {
					break
				}
			}
		}
		if classOpt.IsNil() && strings.Contains(typeName, "__") {
			baseType := strings.SplitN(typeName, "__", 2)[0]
			classOpt = rootScope.ResolveClass(baseType)
			if classOpt.IsNil() {
				for _, child := range rootScope.Children {
					classOpt = child.ResolveClass(baseType)
					if !classOpt.IsNil() {
						break
					}
				}
			}
		}
		if classOpt.IsNil() {
			TypeCheckError(scope, a.Pos, "field assignment '"+a.Name+"."+a.Field+"'",
				"cannot resolve type '"+typeName+"'")
			return
		}
		class := classOpt.Unwrap()

		// Find the field
		fieldVar, ok := class.Variables[a.Field]
		if !ok {
			TypeCheckError(scope, a.Pos, "field assignment '"+a.Name+"."+a.Field+"'",
				"field '"+a.Field+"' not found on type '"+typeName+"'")
			return
		}

		// Get field's type from CProgramValues
		fieldFullName := fieldVar.GetFullName()
		fieldInfo, hasFieldInfo := (*CProgramValues)[fieldFullName]
		if !hasFieldInfo || fieldInfo.GeckoType == nil {
			TypeCheckError(scope, a.Pos, "field assignment '"+a.Name+"."+a.Field+"'",
				"type information not available for field")
			return
		}
		expectedType = fieldInfo.GeckoType

		// If indexing into the field (e.g., self.input_buf[i] = c), get element type
		if a.Index != nil && expectedType != nil {
			if expectedType.Size != nil && expectedType.Size.Type != nil {
				// Fixed-size array: [N]T - get element type T
				expectedType = expectedType.Size.Type
			} else if expectedType.Array != nil {
				// Dynamic array: []T - get element type T
				expectedType = expectedType.Array
			}
		}
	} else if a.Index != nil {
		// Direct array indexing (e.g., arr[i] = val)
		expectedType = valueInfo.GeckoType
		if expectedType != nil {
			if expectedType.Size != nil && expectedType.Size.Type != nil {
				expectedType = expectedType.Size.Type
			} else if expectedType.Array != nil {
				expectedType = expectedType.Array
			}
		}
	} else {
		// Direct variable assignment
		expectedType = valueInfo.GeckoType
	}

	// Get the expression type
	actualType := impl.GetTypeOfExpression(a.Value, scope)
	if actualType == nil {
		TypeCheckError(scope, a.Pos, "assignment to '"+a.Name+"'",
			"cannot infer type of right-hand side expression")
		return
	}

	if !TypesAreCompatible(expectedType, actualType, scope) {
		targetName := a.Name
		if a.Field != "" {
			targetName = a.Name + "." + a.Field
		}
		scope.ErrorScope.NewCompileTimeError(
			"Type Mismatch",
			"Cannot assign '"+FormatTypeRef(actualType)+"' to '"+
				targetName+"' of type '"+FormatTypeRef(expectedType)+"'",
			a.Pos,
		)
	} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
		targetName := a.Name
		if a.Field != "" {
			targetName = a.Name + "." + a.Field
		}
		scope.ErrorScope.NewCompileTimeWarning(
			"Lossy Conversion",
			"Assignment to '"+targetName+"': "+warning,
			a.Pos,
		)
	}

	// Move-state updates are applied during assignment code generation after
	// the RHS expression is consumed. Doing it here is too early and can cause
	// false positives while lowering the same expression.
}
