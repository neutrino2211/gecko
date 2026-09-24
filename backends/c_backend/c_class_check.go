// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// ValidateClassTypeArgs checks that type arguments satisfy class-level type parameter constraints
func (impl *CBackendImplementation) ValidateClassTypeArgs(className string, typeArgs []*tokens.TypeRef, scope *ast.Ast, pos lexer.Position) {
	if className == "" || len(typeArgs) == 0 || scope.ErrorScope == nil {
		return
	}

	// Get the generic class token
	classToken, ok := Generics.GenericClasses[className]
	if !ok || classToken == nil {
		return // Not a registered generic class
	}

	// Check each type parameter constraint
	for i, typeParam := range classToken.TypeParams {
		if i >= len(typeArgs) {
			break
		}

		concreteType := typeArgs[i]
		for _, traitName := range typeParam.AllTraits() {
			if !impl.TypeImplementsTrait(concreteType, traitName, scope) {
				scope.ErrorScope.NewCompileTimeError(
					"Trait Constraint Error",
					fmt.Sprintf("Type '%s' does not implement trait '%s' required by type parameter '%s' of class '%s'",
						FormatTypeRef(concreteType), traitName, typeParam.Name, className),
					pos,
				)
			}
		}
	}
}

// CheckStructLiteralTypes validates that struct literal field values match expected types
func (impl *CBackendImplementation) CheckStructLiteralTypes(structType string, fields []*tokens.ObjectKeyValue, scope *ast.Ast, pos lexer.Position) {
	if structType == "" || len(fields) == 0 || scope.ErrorScope == nil {
		return
	}

	// Look up the struct/class in scope
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(structType)

	// Also check imported modules
	if classOpt.IsNil() {
		for _, child := range rootScope.Children {
			classOpt = child.ResolveClass(structType)
			if !classOpt.IsNil() {
				break
			}
		}
	}

	if classOpt.IsNil() {
		// Class not found - might be generic or external, can't check
		return
	}

	class := classOpt.Unwrap()

	// Check each field
	for _, kv := range fields {
		fieldName := kv.Key
		fieldVar, ok := class.Variables[fieldName]
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Unknown Field",
				fmt.Sprintf("Struct '%s' has no field named '%s'", structType, fieldName),
				pos,
			)
			continue
		}

		// Get expected type from CProgramValues
		fieldFullName := fieldVar.GetFullName()
		fieldInfo, hasInfo := (*CProgramValues)[fieldFullName]
		if !hasInfo || fieldInfo.GeckoType == nil {
			continue // Can't check without type info
		}

		// Get actual type of the value expression
		actualType := impl.GetTypeOfExpression(kv.Value, scope)
		if actualType == nil {
			continue
		}

		expectedType := fieldInfo.GeckoType
		if !TypesAreCompatible(expectedType, actualType, scope) {
			scope.ErrorScope.NewCompileTimeError(
				"Type Mismatch",
				fmt.Sprintf("Field '%s' expects type '%s', got '%s'",
					fieldName, FormatTypeRef(expectedType), FormatTypeRef(actualType)),
				pos,
			)
		} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
			scope.ErrorScope.NewCompileTimeWarning(
				"Lossy Conversion",
				fmt.Sprintf("Field '%s': %s", fieldName, warning),
				pos,
			)
		}
	}
}

// TypeImplementsTrait checks if a type implements a given trait
func (impl *CBackendImplementation) TypeImplementsTrait(typeRef *tokens.TypeRef, traitName string, scope *ast.Ast) bool {
	if typeRef == nil || traitName == "" {
		return true // Can't check, assume OK
	}

	typeName := typeRef.Type
	if typeName == "" {
		return true
	}

	// Remove pointer suffix for class lookup
	baseType := typeName
	for len(baseType) > 0 && baseType[len(baseType)-1] == '*' {
		baseType = baseType[:len(baseType)-1]
	}

	// Look up the class in scope
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(baseType)

	// Also check imported modules
	if classOpt.IsNil() {
		for _, child := range rootScope.Children {
			classOpt = child.ResolveClass(baseType)
			if !classOpt.IsNil() {
				break
			}
		}
	}

	if classOpt.IsNil() {
		// Class not found - might be a primitive or external type
		// For now, return true (can't verify)
		return true
	}

	class := classOpt.Unwrap()

	// Check if the class has the trait implemented (directly or via trait inheritance).
	// Traits are stored with mangled names like "TraitName" or "TraitName__TypeArg".
	for registeredTrait := range class.Traits {
		if TraitMatchesOrExtends(registeredTrait, traitName) {
			return true
		}
	}

	return false
}
