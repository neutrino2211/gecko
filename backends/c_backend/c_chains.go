// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

// processChain handles chained field/method access like a.b.c() or ptr.field.method()
func (impl *CBackendImplementation) processChain(base string, l *tokens.Literal, scope *ast.Ast) string {
	// Track the current type as we traverse the chain
	var currentType *tokens.TypeRef
	var isPointer bool
	var isReceiver bool
	var isModule bool     // Track if base is a module, not a variable
	var moduleName string // The module name for module.constant/function patterns
	var moduleScope *ast.Ast
	var baseVarFullName string

	// Check if the base symbol is a module or enum (not a variable)
	if l.Symbol != "" && l.SymbolModule == "" {
		symbolName := l.Symbol
		varOpt := scope.ResolveSymbolAsVariable(symbolName)
		if varOpt.IsNil() {
			// Not a variable - check if it's an enum type
			if enumCType, isEnum := EnumToCType[symbolName]; isEnum && len(l.Chain) > 0 {
				// Enum value access: Color.Red -> enums__Color_Red
				chain := l.Chain[0]
				enumValue := enumCType[:len(enumCType)] + "_" + chain.Name
				return enumValue
			}

			// Not a variable - check if it's a module
			// Search up the scope hierarchy for imported modules
			checkScope := scope
			for checkScope != nil {
				if childScope, ok := checkScope.Children[symbolName]; ok {
					isModule = true
					moduleName = symbolName
					moduleScope = childScope
					break
				}
				checkScope = checkScope.Parent
			}
		}
	}

	// If the base is a module, handle chain access as module-prefixed symbols
	if isModule && len(l.Chain) > 0 {
		result := base
		for i, chain := range l.Chain {
			if chain.IsMethodCall() {
				// Module function call: module.func(args)
				args := ""
				for j, arg := range chain.GetArgs() {
					if j > 0 {
						args += ", "
					}
					args += impl.moveCallArgument(arg.Value, impl.ExpressionToCString(arg.Value, scope), scope)
				}
				// Generate: module__func(args), unless a foreign/imported module
				// resolves to an external/link alias.
				funcName := moduleName + "__" + chain.Name
				if moduleScope != nil {
					if method := moduleScope.ResolveMethod(chain.Name); !method.IsNil() {
						requireUnsafeCall(method.Unwrap(), scope, chain.Pos)
						funcName = method.Unwrap().CIdentifier()
					}
				}
				if args != "" {
					result = funcName + "(" + args + ")"
				} else {
					result = funcName + "()"
				}
			} else {
				// Module constant access: module.CONSTANT -> module__CONSTANT
				if i == 0 {
					result = moduleName + "__" + chain.Name
				} else {
					result = result + "__" + chain.Name
				}
			}
		}
		return result
	}

	// Try to get the initial type from the literal
	if l.Symbol != "" {
		symbolName := l.Symbol
		if l.SymbolModule != "" {
			// module.field pattern - base is already "module.field" or "var.field"
			// Get the type of the field, not the module/var
			varOpt := scope.ResolveSymbolAsVariable(l.SymbolModule)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				reportUseAfterMoveIfNeeded(scope, variable.GetFullName(), l.SymbolModule, l.Pos)
				fullName := variable.GetFullName()
				if info, ok := (*CProgramValues)[fullName]; ok && info.GeckoType != nil {
					// Get the class to find the field type
					typeName := info.GeckoType.Type
					rootScope := scope.GetRoot()
					classOpt := rootScope.ResolveClass(typeName)
					if !classOpt.IsNil() {
						class := classOpt.Unwrap()
						if fieldVar, ok := class.Variables[symbolName]; ok {
							// Try CProgramValues first
							fieldFullName := fieldVar.GetFullName()
							if fieldInfo, ok := (*CProgramValues)[fieldFullName]; ok {
								currentType = fieldInfo.GeckoType
								isPointer = currentType != nil && currentType.Pointer
							} else {
								// Construct minimal TypeRef
								currentType = &tokens.TypeRef{Pointer: fieldVar.IsPointer}
								isPointer = fieldVar.IsPointer
							}
						}
					}
				}
			}
		} else {
			// Simple symbol
			varOpt := scope.ResolveSymbolAsVariable(symbolName)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				reportUseAfterMoveIfNeeded(scope, variable.GetFullName(), symbolName, l.Pos)
				fullName := variable.GetFullName()
				baseVarFullName = fullName
				// First check if the variable itself is a pointer (like self in trait methods)
				isPointer = variable.IsPointer
				isReceiver = variable.IsReceiver
				if info, ok := (*CProgramValues)[fullName]; ok {
					currentType = info.GeckoType
					// Also check the TypeRef's Pointer flag
					if currentType != nil && currentType.Pointer {
						isPointer = true
					}
				}
			}
		}
	}

	result := base

	for chainIndex, chain := range l.Chain {
		if chain.IsMethodCall() {
			if currentType != nil && currentType.Pointer && (chain.Name == "deref" || chain.Name == "value") {
				requireUnsafe(scope, chain.Pos, "raw pointer read")
			}
			dropHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookDrop)
			isDropCall := chainIndex == 0 && l.SymbolModule == "" && dropHook != nil && len(dropHook.Methods) > 0 && chain.Name == dropHook.Methods[0]

			// Method call - check for builtin trait methods first
			if currentType != nil {
				if code, ok := impl.TryBuiltinMethod(result, currentType, chain.Name, chain.GetArgs(), scope); ok {
					result = code
					if isDropCall && baseVarFullName != "" && CurrentTypeState != nil {
						if !CGetScopeInformation(scope).PreparingDefer {
							CurrentTypeState.SetMoved(baseVarFullName)
						}
						if v := scope.ResolveSymbolAsVariable(l.Symbol); !v.IsNil() && v.Unwrap().DropFlag != "" {
							result = "({ " + v.Unwrap().DropFlag + " = 0; " + result + "; })"
						}
					}
					// After a method call, we don't know the return type without more analysis
					currentType = nil
					isPointer = false
					continue
				}
			}

			if isPointer && !isReceiver {
				requireUnsafe(scope, chain.Pos, "method call through a raw pointer")
			}
			// Regular method call - convert to function call with self argument
			// For now, generate as obj.method(args) -> Type__method(&obj, args)
			args := ""
			for i, arg := range chain.GetArgs() {
				if i > 0 {
					args += ", "
				}
				args += impl.moveCallArgument(arg.Value, impl.ExpressionToCString(arg.Value, scope), scope)
			}

			// Try to find the method in the type's class or traits
			if currentType != nil {
				resolver := NewMethodResolver(impl)
				resolution := resolver.ResolveMethod(currentType, chain.Name, scope)

				if resolution.Found {
					// Calling an @unsafe method requires an @unsafe region at the call site.
					requireUnsafeCall(resolution.Method, scope, chain.Pos)
					// Check method visibility for cross-module calls
					if resolution.Method != nil {
						if visErr := resolution.Method.CheckVisibility(scope); visErr != "" {
							scope.ErrorScope.NewCompileTimeError(
								"Visibility Error",
								visErr,
								chain.Pos,
							)
						}
					}

					selfArg := result
					if !isPointer {
						selfArg = "&" + result
					}
					if args != "" {
						result = resolution.MethodName + "(" + selfArg + ", " + args + ")"
					} else {
						result = resolution.MethodName + "(" + selfArg + ")"
					}
					if isDropCall && baseVarFullName != "" && CurrentTypeState != nil {
						if !CGetScopeInformation(scope).PreparingDefer {
							CurrentTypeState.SetMoved(baseVarFullName)
						}
						if v := scope.ResolveSymbolAsVariable(l.Symbol); !v.IsNil() && v.Unwrap().DropFlag != "" {
							result = "({ " + v.Unwrap().DropFlag + " = 0; " + result + "; })"
						}
					}
					currentType = nil
					isPointer = false
					goto nextChain
				}
			}

			// Fallback: generate as a regular function call
			if args != "" {
				result = chain.Name + "(" + result + ", " + args + ")"
			} else {
				result = chain.Name + "(" + result + ")"
			}
			if isDropCall && baseVarFullName != "" && CurrentTypeState != nil {
				if !CGetScopeInformation(scope).PreparingDefer {
					CurrentTypeState.SetMoved(baseVarFullName)
				}
				if v := scope.ResolveSymbolAsVariable(l.Symbol); !v.IsNil() && v.Unwrap().DropFlag != "" {
					result = "({ " + v.Unwrap().DropFlag + " = 0; " + result + "; })"
				}
			}
			currentType = nil
			isPointer = false
		} else {
			// Field access
			if isPointer {
				if !isReceiver || result != base {
					requireUnsafe(scope, chain.Pos, "raw pointer field access")
				}
				result = result + "->" + chain.Name
			} else {
				result = result + "." + chain.Name
			}

			// Try to update current type based on field type
			if currentType != nil {
				typeName := currentType.Type
				rootScope := scope.GetRoot()
				classOpt := rootScope.ResolveClass(typeName)
				if !classOpt.IsNil() {
					class := classOpt.Unwrap()
					if fieldVar, ok := class.Variables[chain.Name]; ok {
						// First try CProgramValues for full type info
						fullFieldName := fieldVar.GetFullName()
						if info, ok := (*CProgramValues)[fullFieldName]; ok {
							currentType = info.GeckoType
							isPointer = currentType != nil && currentType.Pointer
						} else {
							// Fall back to constructing a minimal TypeRef from Variable flags
							currentType = &tokens.TypeRef{
								Pointer: fieldVar.IsPointer,
							}
							isPointer = fieldVar.IsPointer
						}
					} else {
						currentType = nil
						isPointer = false
					}
				} else {
					currentType = nil
					isPointer = false
				}
			}
		}
	nextChain:
	}

	return result
}
