// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// CheckThrowsHandled checks if a function call to a throwing function is properly handled
func (impl *CBackendImplementation) CheckThrowsHandled(f *tokens.FuncCall, sig *MethodSignature, scope *ast.Ast) {
	if sig.Throws == nil {
		return // Function doesn't throw
	}

	// For now, just emit a compile error - proper handling context will be added later
	scope.ErrorScope.NewCompileTimeError(
		"Unhandled Exception",
		fmt.Sprintf("function '%s' throws '%s' but error is not handled",
			f.Function, FormatTypeRef(sig.Throws)),
		f.Pos,
	)
}

func resolveFunctionCallSignature(f *tokens.FuncCall, scope *ast.Ast) (*MethodSignature, bool) {
	if f == nil || scope == nil {
		return nil, false
	}

	var funcKey string
	var sig *MethodSignature
	var ok bool

	if f.StaticType != "" {
		// Static method call: Type::method()
		rootScope := scope.GetRoot()
		classOpt := rootScope.ResolveClass(f.StaticType)
		if classOpt.IsNil() {
			for _, child := range rootScope.Children {
				classOpt = child.ResolveClass(f.StaticType)
				if !classOpt.IsNil() {
					break
				}
			}
		}
		if !classOpt.IsNil() {
			class := classOpt.Unwrap()
			if method, hasMethod := class.Methods[f.Function]; hasMethod {
				funcKey = method.CIdentifier()
				sig, ok = MethodSignatures[funcKey]
			}
		}
		if !ok {
			funcKey = scope.GetRoot().GetFullName() + "__" + f.StaticType + "__" + f.Function
			sig, ok = MethodSignatures[funcKey]
		}
		if !ok {
			funcKey = f.StaticType + "__" + f.Function
			sig, ok = MethodSignatures[funcKey]
		}
		if !ok {
			// External static methods may be registered by short symbol name.
			sig, ok = MethodSignatures[f.Function]
		}
	} else if f.Module != "" {
		// module.function() call
		funcKey = f.Module + "__" + f.Function
		sig, ok = MethodSignatures[funcKey]
	} else {
		// Regular function call - try multiple lookup strategies
		methodOpt := scope.ResolveMethod(f.Function)
		if !methodOpt.IsNil() {
			method := methodOpt.Unwrap()
			funcKey = method.GetFullName()
			sig, ok = MethodSignatures[funcKey]
		}

		if !ok {
			funcKey = scope.GetRoot().GetFullName() + "__" + f.Function
			sig, ok = MethodSignatures[funcKey]
		}

		// External methods are registered by short name.
		if !ok {
			sig, ok = MethodSignatures[f.Function]
		}

		if !ok {
			parent := scope
			for parent != nil {
				funcKey = parent.GetFullName() + "__" + f.Function
				sig, ok = MethodSignatures[funcKey]
				if ok {
					break
				}
				parent = parent.Parent
			}
		}
	}

	return sig, ok
}

// CheckFunctionCallTypes checks if function call arguments match parameter types
func (impl *CBackendImplementation) CheckFunctionCallTypes(f *tokens.FuncCall, scope *ast.Ast) {
	if f == nil || scope.ErrorScope == nil {
		return
	}

	sig, ok := resolveFunctionCallSignature(f, scope)
	if !ok {
		return // Can't check - method signature not found
	}

	// Build type substitution map for generic functions and validate trait constraints
	typeSubst := make(map[string]*tokens.TypeRef)
	if sig.IsGeneric {
		// If no type arguments provided, we can't check - skip
		if len(f.TypeArgs) == 0 {
			return
		}
		// Build substitution map and validate trait constraints
		for i, typeParam := range sig.TypeParams {
			if i < len(f.TypeArgs) {
				concreteType := f.TypeArgs[i]
				typeSubst[typeParam.Name] = concreteType

				// Validate all trait constraints
				for _, traitName := range typeParam.AllTraits() {
					if !impl.TypeImplementsTrait(concreteType, traitName, scope) {
						scope.ErrorScope.NewCompileTimeError(
							"Trait Constraint Error",
							fmt.Sprintf("Type '%s' does not implement trait '%s' required by type parameter '%s'",
								FormatTypeRef(concreteType), traitName, typeParam.Name),
							f.Pos,
						)
					}
				}
			}
		}
	}

	// Check argument count
	if !sig.Variadic {
		if len(f.Arguments) != len(sig.Parameters) {
			scope.ErrorScope.NewCompileTimeError(
				"Argument Count Mismatch",
				fmt.Sprintf("Function '%s' expects %d arguments, got %d",
					f.Function, len(sig.Parameters), len(f.Arguments)),
				f.Pos,
			)
			return
		}
	} else if len(f.Arguments) < len(sig.Parameters) {
		scope.ErrorScope.NewCompileTimeError(
			"Argument Count Mismatch",
			fmt.Sprintf("Variadic function '%s' expects at least %d arguments, got %d",
				f.Function, len(sig.Parameters), len(f.Arguments)),
			f.Pos,
		)
		return
	}

	seenOutTargets := make(map[string]bool)

	// Check each argument type
	for i, arg := range f.Arguments {
		if i >= len(sig.Parameters) {
			if sig.Variadic {
				continue
			}
			break
		}

		expectedType := sig.Parameters[i]
		if expectedType == nil {
			continue
		}

		// Apply type substitution for generic functions
		if len(typeSubst) > 0 {
			expectedType = substituteTypeRef(expectedType, typeSubst)
		}

		expectedOut := false
		if i < len(sig.ParamOut) {
			expectedOut = sig.ParamOut[i]
		}

		if expectedOut != arg.Out {
			paramName := ""
			if i < len(sig.ParamNames) {
				paramName = sig.ParamNames[i]
			}
			if expectedOut {
				scope.ErrorScope.NewCompileTimeError(
					"Out Argument Required",
					fmt.Sprintf("Argument %s for function '%s' must be passed with 'out'", paramName, f.Function),
					f.Pos,
				)
			} else {
				scope.ErrorScope.NewCompileTimeError(
					"Unexpected Out Argument",
					fmt.Sprintf("Argument %s for function '%s' is not an out parameter", paramName, f.Function),
					f.Pos,
				)
			}
			continue
		}

		if arg.Out {
			targetName := extractSymbolFromExpression(arg.Value)
			if targetName == "" {
				scope.ErrorScope.NewCompileTimeError(
					"Invalid Out Argument",
					"Out arguments must be plain assignable variables (for example: out db)",
					f.Pos,
				)
				continue
			}

			targetOpt := scope.ResolveSymbolAsVariable(targetName)
			if targetOpt.IsNil() {
				scope.ErrorScope.NewCompileTimeError(
					"Invalid Out Argument",
					fmt.Sprintf("Out target '%s' could not be resolved as a variable", targetName),
					f.Pos,
				)
				continue
			}

			targetVar := targetOpt.Unwrap()
			if targetVar.IsConst {
				scope.ErrorScope.NewCompileTimeError(
					"Invalid Out Argument",
					fmt.Sprintf("Out target '%s' is const and cannot be written by callee", targetName),
					f.Pos,
				)
				continue
			}

			fullTarget := targetVar.GetFullName()
			if seenOutTargets[fullTarget] {
				scope.ErrorScope.NewCompileTimeError(
					"Out Argument Aliasing",
					fmt.Sprintf("Out target '%s' is used more than once in the same call", targetName),
					f.Pos,
				)
				continue
			}
			seenOutTargets[fullTarget] = true
		}

		actualType := impl.GetTypeOfExpression(arg.Value, scope)
		if actualType == nil {
			continue
		}

		if !TypesAreCompatible(expectedType, actualType, scope) {
			paramName := ""
			if i < len(sig.ParamNames) {
				paramName = sig.ParamNames[i]
			}
			scope.ErrorScope.NewCompileTimeError(
				"Type Mismatch",
				"Argument "+(paramName)+" expects type '"+FormatTypeRef(expectedType)+
					"', got '"+FormatTypeRef(actualType)+"'",
				f.Pos,
			)
		} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
			// Warn about implicit lossy numeric conversion
			paramName := ""
			if i < len(sig.ParamNames) {
				paramName = " (" + sig.ParamNames[i] + ")"
			}
			scope.ErrorScope.NewCompileTimeWarning(
				"Lossy Conversion",
				"Argument"+paramName+": "+warning,
				f.Pos,
			)
		}
	}

	// Check if throwing function is properly handled
	impl.CheckThrowsHandled(f, sig, scope)
}
