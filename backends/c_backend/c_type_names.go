// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// TypeRefToCType converts a gecko TypeRef to a C type string
func TypeRefToCType(t *tokens.TypeRef, scope *ast.Ast) string {
	if t == nil {
		return "void"
	}

	// Resolve Self to the current class type
	if t.Type == "Self" && CurrentSelfType != "" {
		return CurrentSelfType
	}

	base := ""

	if t.Size != nil {
		// Fixed-size array: [N]T -> T* (passed as pointer in C)
		base = TypeRefToCType(t.Size.Type, scope) + "*"
	} else if t.Array != nil {
		base = TypeRefToCType(t.Array, scope) + "*"
	} else if t.FuncType != nil {
		// Function pointer type: return_type (*)( param_types... )
		retType := "void"
		if t.FuncType.ReturnType != nil {
			retType = TypeRefToCType(t.FuncType.ReturnType, scope)
		}

		params := ""
		for i, paramType := range t.FuncType.ParamTypes {
			if i > 0 {
				params += ", "
			}
			params += TypeRefToCType(paramType, scope)
		}
		if params == "" {
			params = "void"
		}

		// Return function pointer type with __FUNCPTR__ marker for name placement
		base = retType + " (*__FUNCPTR__)(" + params + ")"
	} else {
		cType, ok := GeckoToCType[t.Type]
		if ok {
			base = cType
		} else if enumType, isEnum := EnumToCType[t.Type]; isEnum {
			base = enumType
		} else if t.Module != "" {
			// Module-qualified type: module.Type
			// Use simple type name (typedef names don't include module prefix)
			if len(t.TypeArgs) > 0 {
				typeArgStrs := make([]string, len(t.TypeArgs))
				for i, typeArg := range t.TypeArgs {
					typeArgStrs[i] = TypeRefToCType(typeArg, scope)
				}
				base = Generics.RequestClassInstantiation(t.Type, typeArgStrs)
			} else {
				base = t.Type
			}
		} else {
			// Check if this is a type parameter that should be substituted
			if CurrentMonomorphContext != nil {
				if concreteType, found := CurrentMonomorphContext.GetConcreteTypeForParam(t.Type); found {
					base = concreteType
				} else if len(t.TypeArgs) > 0 {
					// Generic type instantiation
					typeArgStrs := make([]string, len(t.TypeArgs))
					for i, typeArg := range t.TypeArgs {
						typeArgStrs[i] = TypeRefToCType(typeArg, scope)
					}
					base = Generics.RequestClassInstantiation(t.Type, typeArgStrs)
				} else {
					base = t.Type
				}
			} else if len(t.TypeArgs) > 0 {
				// Convert type arguments to C types
				typeArgStrs := make([]string, len(t.TypeArgs))
				for i, typeArg := range t.TypeArgs {
					typeArgStrs[i] = TypeRefToCType(typeArg, scope)
				}
				// Request instantiation and get mangled name
				base = Generics.RequestClassInstantiation(t.Type, typeArgStrs)
			} else {
				// Unknown type, use as-is (struct or custom type)
				base = t.Type
			}
		}
	}

	// Qualifiers apply to the pointee / value (C: const/volatile T*).
	if t.Const {
		base = "const " + base
	}
	if t.Volatile {
		base = "volatile " + base
	}

	if t.Pointer {
		base += "*"
	}

	return base
}

// GetMonomorphizedClassName returns the class name to use for lookup.
// For generic types like Raw<uint32>, returns the mangled name like Raw__uint32.
func GetMonomorphizedClassName(t *tokens.TypeRef, scope *ast.Ast) string {
	if t == nil {
		return ""
	}
	if len(t.TypeArgs) > 0 {
		// Generic type - get the mangled name
		typeArgStrs := make([]string, len(t.TypeArgs))
		for i, typeArg := range t.TypeArgs {
			typeArgStrs[i] = TypeRefToCType(typeArg, scope)
		}
		return Generics.RequestClassInstantiation(t.Type, typeArgStrs)
	}
	return t.Type
}

// IsFuncPointerType checks if a type string is a function pointer type
func IsFuncPointerType(cType string) bool {
	return strings.Contains(cType, "__FUNCPTR__")
}

// FormatFuncPointerDecl formats a function pointer declaration with variable name
func FormatFuncPointerDecl(cType, varName string) string {
	return strings.Replace(cType, "__FUNCPTR__", varName, 1)
}

// GetScopedTypeName returns the scoped C type name for a module-qualified type.
// This is the single source of truth for type name mangling with module prefixes.
// Examples:
//   - GetScopedTypeName("geometry", "Point") -> "geometry__Point"
//   - GetScopedTypeName("", "Point") -> "Point"
//   - GetScopedTypeName("std.collections", "Vec") -> "std__collections__Vec"
func GetScopedTypeName(module string, typeName string) string {
	if module == "" {
		return typeName
	}
	// Replace dots with double underscores for nested modules
	modulePrefix := strings.ReplaceAll(module, ".", "__")
	return modulePrefix + "__" + typeName
}

// ResolveClassFromTypeRef resolves a TypeRef to a class AST, handling module qualification.
// For module-qualified types (e.g., geometry.Point), it searches the child scope.
// Returns the class AST and the scoped C name for the type.
func ResolveClassFromTypeRef(typeRef *tokens.TypeRef, scope *ast.Ast) (*ast.Ast, string) {
	if typeRef == nil {
		return nil, ""
	}

	rootScope := scope.GetRoot()
	typeName := typeRef.Type
	scopedName := GetScopedTypeName(typeRef.Module, typeName)

	// Handle generic types first
	if len(typeRef.TypeArgs) > 0 {
		typeArgStrs := make([]string, len(typeRef.TypeArgs))
		for i, typeArg := range typeRef.TypeArgs {
			typeArgStrs[i] = TypeRefToCType(typeArg, scope)
		}
		scopedName = Generics.RequestClassInstantiation(scopedName, typeArgStrs)
	}

	// For module-qualified types, search the child scope first
	if typeRef.Module != "" {
		if child, ok := rootScope.Children[typeRef.Module]; ok {
			if classOpt := child.ResolveClass(typeName); !classOpt.IsNil() {
				return classOpt.Unwrap(), scopedName
			}
		}
	}

	// Search in root scope
	if classOpt := rootScope.ResolveClass(typeName); !classOpt.IsNil() {
		return classOpt.Unwrap(), scopedName
	}

	// Search imported modules
	for _, child := range rootScope.Children {
		if classOpt := child.ResolveClass(typeName); !classOpt.IsNil() {
			return classOpt.Unwrap(), scopedName
		}
	}

	return nil, scopedName
}
