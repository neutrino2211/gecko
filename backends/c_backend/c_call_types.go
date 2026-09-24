// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// GetTypeOfFuncCall attempts to determine the return type of a function call
func (impl *CBackendImplementation) GetTypeOfFuncCall(f *tokens.FuncCall, scope *ast.Ast) *tokens.TypeRef {
	return ResolveCallSelfType(impl.getTypeOfFuncCallRaw(f, scope), f)
}

// ResolveCallSelfType replaces a `Self` return type with the receiver's concrete
// type, since `Self` is only meaningful inside the owning type's methods.
func ResolveCallSelfType(t *tokens.TypeRef, f *tokens.FuncCall) *tokens.TypeRef {
	if t == nil || t.Type != "Self" || f == nil {
		return t
	}
	if f.StaticType != "" {
		resolved := *t
		resolved.Type = f.StaticType
		return &resolved
	}
	return t
}

func (impl *CBackendImplementation) getTypeOfFuncCallRaw(f *tokens.FuncCall, scope *ast.Ast) *tokens.TypeRef {
	if f == nil {
		return nil
	}
	if CurrentSemanticProgram != nil {
		if res := CurrentSemanticProgram.FuncCallResolution(f); res != nil && res.ReturnType != nil {
			return res.ReturnType
		}
	}

	// Static method call: Type::method()
	if f.StaticType != "" {
		rootScope := scope.GetRoot()
		classOpt := scope.ResolveClass(f.StaticType)
		if classOpt.IsNil() {
			classOpt = rootScope.ResolveClass(f.StaticType)
		}
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
			if method, ok := class.Methods[f.Function]; ok {
				aliasKey := f.StaticType + "#" + f.Function
				if fullType, ok := MethodReturnTypes[aliasKey]; ok && fullType != nil {
					return fullType
				}

				// Inherent impl methods are emitted with mangled names (Type__method)
				// and tracked in MethodReturnTypes under scope#MangledName.
				mangledMethodName := class.GetFullName() + "__" + f.Function
				for key, fullType := range MethodReturnTypes {
					if strings.HasSuffix(key, "#"+mangledMethodName) {
						return fullType
					}
				}

				// Prefer full TypeRef (with type args) when available.
				// Class methods are keyed by ClassFullScope#methodName.
				fullKey := class.FullScopeName() + "#" + f.Function
				if fullType, ok := MethodReturnTypes[fullKey]; ok {
					return fullType
				}

				// Imported module scopes may carry additional prefixes (e.g., main.fs.File#open).
				// Fall back to suffix-based lookup to recover the precise generic return type.
				qualifiedSuffix := "." + f.StaticType + "#" + f.Function
				plainSuffix := f.StaticType + "#" + f.Function
				for key, fullType := range MethodReturnTypes {
					if strings.HasSuffix(key, qualifiedSuffix) || strings.HasSuffix(key, plainSuffix) {
						if fullType != nil && fullType.Type == method.Type {
							return fullType
						}
					}
				}

				// Inherent impl methods are usually keyed as scope#<module__Type__method>.
				// Match by mangled method-name suffix when class scope metadata is incomplete.
				mangledSuffix := "__" + f.StaticType + "__" + f.Function
				for key, fullType := range MethodReturnTypes {
					if strings.HasSuffix(key, mangledSuffix) {
						if fullType != nil && fullType.Type == method.Type {
							return fullType
						}
					}
				}

				// Final fallback: recover full return type from registered method signatures.
				// This keeps generic args (e.g., Option<File>) for static calls in imported modules.
				sigLookupKeys := []string{
					class.GetFullName() + "__" + f.Function,
					scope.GetRoot().GetFullName() + "__" + f.StaticType + "__" + f.Function,
					f.StaticType + "__" + f.Function,
				}
				for _, key := range sigLookupKeys {
					if sig, ok := MethodSignatures[key]; ok && sig != nil && sig.ReturnType != nil {
						return sig.ReturnType
					}
				}

				return &tokens.TypeRef{Type: method.Type}
			}
		}
	}

	// Method call on variable: variable.method()
	if f.Module != "" {
		// Check if Module is a local variable
		varOpt := scope.ResolveSymbolAsVariable(f.Module)
		if !varOpt.IsNil() {
			variable := varOpt.Unwrap()
			fullVarName := variable.GetFullName()
			if valueInfo, ok := (*CProgramValues)[fullVarName]; ok && valueInfo.GeckoType != nil {
				// Handle builtin pointer methods first
				if valueInfo.GeckoType.Pointer {
					switch f.Function {
					case "read":
						// ptr.read() returns the element type (dereference)
						return &tokens.TypeRef{Type: valueInfo.GeckoType.Type}
					case "write":
						// ptr.write(value) returns void
						return &tokens.TypeRef{Type: "void"}
					}
				}

				typeName := valueInfo.GeckoType.Type
				typeArgs := valueInfo.GeckoType.TypeArgs
				rootScope := scope.GetRoot()
				classOpt := rootScope.ResolveClass(typeName)
				if !classOpt.IsNil() {
					class := classOpt.Unwrap()

					// First check direct class methods
					if method, ok := class.Methods[f.Function]; ok {
						returnType := method.Type
						// Substitute generic type parameter with actual type arg
						// e.g., Vec<int32>.get() returns T -> substitute T with int32
						if len(typeArgs) > 0 && returnType == "T" {
							return typeArgs[0]
						}
						return &tokens.TypeRef{Type: returnType}
					}

					// Then check trait methods
					for _, traitMethods := range class.Traits {
						if traitMethods == nil {
							continue
						}
						for _, method := range *traitMethods {
							// Trait method names are mangled: TypeName__TraitName__methodName
							suffix := "__" + f.Function
							if len(method.Name) >= len(suffix) && method.Name[len(method.Name)-len(suffix):] == suffix {
								returnType := method.Type
								// Substitute generic type parameter
								if len(typeArgs) > 0 && returnType == "T" {
									return typeArgs[0]
								}
								return &tokens.TypeRef{Type: returnType}
							}
						}
					}
				}
			}
		}
	}

	// Regular function call
	if f.Module == "" {
		mth := scope.ResolveMethod(f.Function)
		if !mth.IsNil() {
			method := mth.Unwrap()
			// Try to get full return type with TypeArgs from MethodReturnTypes
			// Search from root scope since functions are typically defined at module level
			rootScope := scope.GetRoot()
			fullName := rootScope.FullScopeName() + "#" + f.Function
			if fullType, ok := MethodReturnTypes[fullName]; ok {
				return fullType
			}
			// Fallback to string-based type
			return &tokens.TypeRef{Type: method.Type}
		}
	}

	return nil
}
