package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// MethodResolution contains the result of resolving a method call
type MethodResolution struct {
	MethodName      string      // The mangled C function name (e.g., "Raw__uint32_t__init")
	Found           bool        // Whether the method was found
	IsGenericDirect bool        // True if resolved directly from generic type (no class lookup)
	Method          *ast.Method // The resolved method (for visibility checking), may be nil for generic direct
	OriginModule    string      // Module where the method was defined
}

// MethodResolver handles method resolution for both FuncCall and processChain
type MethodResolver struct {
	Impl *CBackendImplementation
}

// NewMethodResolver creates a new MethodResolver
func NewMethodResolver(impl *CBackendImplementation) *MethodResolver {
	return &MethodResolver{Impl: impl}
}

// ResolveMethod resolves a method call on a type, returning the mangled function name.
// It handles generic instances, direct class methods, and trait methods.
//
// Parameters:
//   - typeRef: The type of the receiver (e.g., Raw<uint32> or StringBuilder)
//   - methodName: The method being called (e.g., "init", "push")
//   - scope: The current AST scope for lookups
//
// Returns MethodResolution with the mangled function name if found.
func (r *MethodResolver) ResolveMethod(
	typeRef *tokens.TypeRef,
	methodName string,
	scope *ast.Ast,
) MethodResolution {
	if typeRef == nil {
		return MethodResolution{}
	}

	// Get the monomorphized type name (e.g., "Raw__uint32_t" for Raw<uint32>)
	typeName := GetMonomorphizedClassName(typeRef, scope)
	isGenericInstance := len(typeRef.TypeArgs) > 0 || strings.Contains(typeRef.Type, "__")

	// For generic class instances, directly construct the method name.
	// The class may not be registered yet when we process method calls
	// (monomorphization happens lazily during code generation).
	if isGenericInstance {
		// Get the base class name (without type args) for origin lookup
		// The type might be "Vec" or "Vec__int32_t" (already mangled)
		baseClassName := typeRef.Type
		if idx := strings.Index(baseClassName, "__"); idx > 0 {
			// Already mangled - extract the base name
			baseClassName = baseClassName[:idx]
		}

		// Include origin module prefix if the generic class was defined in another module
		methodPrefix := typeName
		if originModule, ok := Generics.GenericClassOrigins[baseClassName]; ok && originModule != "" {
			methodPrefix = originModule + "__" + typeName
		}

		return MethodResolution{
			MethodName:      methodPrefix + "__" + methodName,
			Found:           true,
			IsGenericDirect: true,
		}
	}

	// For non-generic types, look up the class
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(typeName)

	// If not found in root scope, search imported modules
	if classOpt.IsNil() {
		for _, child := range rootScope.Children {
			classOpt = child.ResolveClass(typeName)
			if !classOpt.IsNil() {
				break
			}
		}
	}

	if classOpt.IsNil() {
		return MethodResolution{}
	}

	class := classOpt.Unwrap()
	originModule := class.GetOriginModule()

	// First check for direct class methods
	if method, ok := class.Methods[methodName]; ok {
		return MethodResolution{
			MethodName:   class.GetFullName() + "__" + methodName,
			Found:        true,
			Method:       method,
			OriginModule: originModule,
		}
	}

	// Then search trait methods
	for traitName, traitMethods := range class.Traits {
		if traitMethods == nil {
			continue
		}
		expectedName := typeName + "__" + traitName + "__" + methodName
		for _, method := range *traitMethods {
			if method.Name == expectedName {
				return MethodResolution{
					MethodName:   expectedName,
					Found:        true,
					Method:       method,
					OriginModule: originModule,
				}
			}
		}
	}

	// Fallback: if class was found but method not registered yet (timing issue),
	// construct the method name directly. The method might be defined later
	// in the same impl block.
	return MethodResolution{
		MethodName:   class.GetFullName() + "__" + methodName,
		Found:        true,
		OriginModule: originModule,
	}
}

// ResolveConstrainedGeneric checks if a type is a constrained generic parameter
// and returns the resolved method name if so. This handles cases like:
//
//	func process<T is Trait>(x: T) { x.method() }
//	func process<T is A & B>(x: T) { x.method() }  // Multiple constraints
//
// where T is substituted with a concrete type at monomorphization time.
// For multiple constraints, it searches to find which trait provides the method.
func (r *MethodResolver) ResolveConstrainedGeneric(
	typeName string,
	methodName string,
) (string, bool) {
	if CurrentMonomorphContext == nil {
		return "", false
	}

	concreteType, ok := CurrentMonomorphContext.GetConcreteTypeForParam(typeName)
	if !ok {
		return "", false
	}

	// Find which trait provides this method (supports multiple constraints)
	traitName := CurrentMonomorphContext.FindTraitWithMethod(typeName, methodName)
	if traitName == "" {
		// Fallback to first trait if method lookup fails (backwards compat)
		traitName, ok = CurrentMonomorphContext.GetTraitConstraint(typeName)
		if !ok {
			return "", false
		}
	}

	// Build the trait method name: ConcreteType__TraitName__methodName
	return concreteType + "__" + traitName + "__" + methodName, true
}
