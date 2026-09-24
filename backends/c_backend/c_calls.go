// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

// FuncCallToCString converts function calls to C code
func (impl *CBackendImplementation) FuncCallToCString(f *tokens.FuncCall, scope *ast.Ast) string {
	if f == nil {
		return ""
	}

	// Type check the function call arguments
	impl.CheckFunctionCallTypes(f, scope)

	// Treeshake v1 safety gate: calls through function-pointer variables are dynamic
	// and currently unsafe for static reachability pruning.
	if f.StaticType == "" && f.Module == "" {
		methodOpt := scope.ResolveMethod(f.Function)
		if methodOpt.IsNil() {
			varOpt := scope.ResolveSymbolAsVariable(f.Function)
			if !varOpt.IsNil() {
				v := varOpt.Unwrap()
				if valueInfo, ok := (*CProgramValues)[v.GetFullName()]; ok && valueInfo != nil && valueInfo.GeckoType != nil && valueInfo.GeckoType.FuncType != nil {
					RecordTreeshakeDynamicCall(
						f.Pos,
						"dynamic call through function pointer '"+f.Function+"'",
					)
				}
			}
		}
	}

	var funcName string
	var baseFuncName string
	var selfArg string

	// Handle static type calls: module.Type<Args>::function() or Type::function()
	if f.StaticType != "" {
		requireUnsafeCall(genericMethodMetadata(f.StaticType, f.Function), scope, f.Pos)
		// Build the mangled type name with type arguments
		typeName := f.StaticType
		rootScope := scope.GetRoot()
		isGenericInstance := len(f.StaticTypeArgs) > 0

		// If there's a static module prefix (e.g., console.Console::new())
		// we need to look up the class in that module's scope
		var lookupScope *ast.Ast
		modulePrefix := ""
		if f.StaticModule != "" {
			modulePrefix = f.StaticModule + "__"
			// Look up the module in scope hierarchy
			checkScope := scope
			for checkScope != nil {
				if modScope, ok := checkScope.Children[f.StaticModule]; ok {
					lookupScope = modScope
					break
				}
				checkScope = checkScope.Parent
			}
			if lookupScope == nil {
				lookupScope = rootScope
			}
		} else {
			lookupScope = scope
		}

		if isGenericInstance {
			// Generic type instantiation - build mangled name
			typeArgStrs := make([]string, len(f.StaticTypeArgs))
			for i, typeArg := range f.StaticTypeArgs {
				typeArgStrs[i] = TypeRefToCType(typeArg, scope)
			}

			// Validate class type parameter constraints
			impl.ValidateClassTypeArgs(f.StaticType, f.StaticTypeArgs, scope, f.Pos)

			typeName = mangleName(typeName, typeArgStrs)

			// Get origin module for proper method naming
			effectivePrefix := modulePrefix
			if originModule, ok := Generics.GenericClassOrigins[f.StaticType]; ok && originModule != "" && effectivePrefix == "" {
				effectivePrefix = originModule + "__"
			}

			baseFuncName = effectivePrefix + typeName + "__" + f.Function
		} else {
			// Non-generic type reference - but check if the class is actually generic
			// For Slice::from_raw<Rectangle>(), the type args are on the method call
			// but should be applied to the class instantiation
			if Generics.IsGenericClass(f.StaticType) && len(f.TypeArgs) > 0 {
				// This is a generic class with type args on the method call
				// Treat as: GenericClass<TypeArgs>::method()
				typeArgStrs := make([]string, len(f.TypeArgs))
				for i, typeArg := range f.TypeArgs {
					typeArgStrs[i] = TypeRefToCType(typeArg, scope)
				}

				// Validate class type parameter constraints
				impl.ValidateClassTypeArgs(f.StaticType, f.TypeArgs, scope, f.Pos)

				// Request class instantiation
				mangledTypeName := Generics.RequestClassInstantiation(f.StaticType, typeArgStrs)

				// Get origin module for proper method naming
				originModule := Generics.GenericClassOrigins[f.StaticType]
				if originModule != "" {
					baseFuncName = originModule + "__" + mangledTypeName + "__" + f.Function
				} else {
					baseFuncName = modulePrefix + mangledTypeName + "__" + f.Function
				}

				// Clear TypeArgs since we've handled them as class type args
				f.TypeArgs = nil
			} else {
				// Non-generic type - look up the class to check for direct methods and trait methods
				classOpt := lookupScope.ResolveClass(f.StaticType)
				if !classOpt.IsNil() {
					class := classOpt.Unwrap()
					fullTypeName := class.GetFullName()

					// First check for direct class method
					if method, ok := class.Methods[f.Function]; ok {
						// Calling an @unsafe method requires an @unsafe region at the call site.
						requireUnsafeCall(method, scope, f.Pos)
						// Check method visibility for cross-module calls
						if visErr := method.CheckVisibility(scope); visErr != "" {
							scope.ErrorScope.NewCompileTimeError(
								"Visibility Error",
								visErr,
								f.Pos,
							)
						}
						baseFuncName = method.CIdentifier()
					} else {
						// Not a direct method - search trait implementations for static methods
						found := false
						for traitName, traitMethods := range class.Traits {
							if traitMethods == nil {
								continue
							}
							for _, method := range *traitMethods {
								// Check if this trait has a method with matching name
								expectedName := typeName + "__" + traitName + "__" + f.Function
								if method.Name == expectedName {
									// Calling an @unsafe method requires an @unsafe region at the call site.
									requireUnsafeCall(method, scope, f.Pos)
									// Check method visibility for cross-module calls
									if visErr := method.CheckVisibility(scope); visErr != "" {
										scope.ErrorScope.NewCompileTimeError(
											"Visibility Error",
											visErr,
											f.Pos,
										)
									}
									baseFuncName = modulePrefix + expectedName
									found = true
									break
								}
							}
							if found {
								break
							}
						}

						// If still not found, use the default mangling (might be external or direct method)
						if !found {
							baseFuncName = fullTypeName + "__" + f.Function
						}
					}
				} else {
					// Class not found - use module-prefixed default mangling
					baseFuncName = modulePrefix + typeName + "__" + f.Function
				}
			}
		}
	} else if f.Module != "" {
		// Could be module.function() or variable.method() (trait call)
		// First check if it's a local variable
		varOpt := scope.ResolveSymbolAsVariable(f.Module)
		if !varOpt.IsNil() {
			// It's a variable - check for trait method call
			variable := varOpt.Unwrap()
			reportUseAfterMoveIfNeeded(scope, variable.GetFullName(), f.Module, f.Pos)
			info := CGetScopeInformation(scope)
			varName := resolveVariableWithIdentifier(variable, scope, info)

			// Get the variable's type to find trait methods
			fullVarName := variable.GetFullName()
			valueInfo, hasInfo := (*CProgramValues)[fullVarName]
			if hasInfo && valueInfo.GeckoType != nil {
				if valueInfo.GeckoType.Pointer && (f.Function == "deref" || f.Function == "value") {
					requireUnsafe(scope, f.Pos, "raw pointer read")
				}
				// Try builtin trait methods first (e.g., ptr.deref(), ptr.is_null())
				if code, ok := impl.TryBuiltinMethod(varName, valueInfo.GeckoType, f.Function, f.Arguments, scope); ok {
					return code
				}

				if valueInfo.GeckoType.Pointer && !variable.IsReceiver {
					requireUnsafe(scope, f.Pos, "method call through a raw pointer")
				}
				typeName := valueInfo.GeckoType.Type
				resolver := NewMethodResolver(impl)

				// Check if this is a generic type parameter with a trait constraint
				if constrainedName, ok := resolver.ResolveConstrainedGeneric(typeName, f.Function, scope); ok {
					baseFuncName = constrainedName
					if variable.IsPointer {
						selfArg = varName
					} else {
						selfArg = "&" + varName
					}
				}

				// If not a constrained generic, try regular class/trait lookup
				if baseFuncName == "" {
					resolution := resolver.ResolveMethod(valueInfo.GeckoType, f.Function, scope)
					if resolution.Found {
						// Calling an @unsafe method requires an @unsafe region at the call site.
						requireUnsafeCall(resolution.Method, scope, f.Pos)
						// Check method visibility for cross-module calls
						if resolution.Method != nil {
							if visErr := resolution.Method.CheckVisibility(scope); visErr != "" {
								scope.ErrorScope.NewCompileTimeError(
									"Visibility Error",
									visErr,
									f.Pos,
								)
							}
						}
						baseFuncName = resolution.MethodName
						if variable.IsPointer {
							selfArg = varName
						} else {
							selfArg = "&" + varName
						}
					}
				}
			}

			// If no class/trait method found, fall back to direct call
			if baseFuncName == "" {
				baseFuncName = f.Module + "__" + f.Function
			}
		} else {
			// Module-prefixed call: module.function()
			rootScope := scope
			for rootScope.Parent != nil {
				rootScope = rootScope.Parent
			}

			if importedModule, ok := rootScope.Children[f.Module]; ok {
				mth := importedModule.ResolveMethod(f.Function)
				if !mth.IsNil() {
					requireUnsafeCall(mth.Unwrap(), scope, f.Pos)
					baseFuncName = mth.Unwrap().CIdentifier()
				} else {
					baseFuncName = f.Module + "__" + f.Function
				}
			} else {
				baseFuncName = f.Module + "__" + f.Function
			}
		}
	} else {
		// Local function call
		mth := scope.ResolveMethod(f.Function)
		if !mth.IsNil() {
			requireUnsafeCall(mth.Unwrap(), scope, f.Pos)
			baseFuncName = mth.Unwrap().CIdentifier()
		} else {
			baseFuncName = f.Function
		}
	}

	// Handle generic function instantiation
	if len(f.TypeArgs) > 0 {
		typeArgStrs := make([]string, len(f.TypeArgs))
		for i, typeArg := range f.TypeArgs {
			typeArgStrs[i] = TypeRefToCType(typeArg, scope)
		}
		funcName = Generics.RequestMethodInstantiation(baseFuncName, typeArgStrs)
	} else {
		funcName = baseFuncName
	}

	args := ""
	// If this is a trait method call, add self as first argument
	if selfArg != "" {
		args = selfArg
	}
	for i, arg := range f.Arguments {
		if args != "" || i > 0 {
			args += ", "
		}
		if arg.Value != nil {
			// Check if argument is a lambda with captures
			if lambda := arg.Value.GetLambda(); lambda != nil {
				key := fmt.Sprintf("%d:%d", lambda.Pos.Line, lambda.Pos.Column)
				if initCode, ok := PendingCaptureInit[key]; ok && initCode != "" {
					info := CGetScopeInformation(scope)
					info.Code += initCode
					delete(PendingCaptureInit, key)
				}
			}
			argExpr := impl.ExpressionToCString(arg.Value, scope)
			if arg.Out {
				args += "&" + argExpr
			} else {
				args += impl.moveCallArgument(arg.Value, argExpr, scope)
			}
		}
	}

	call := funcName + "(" + args + ")"
	if f.Module != "" {
		if variable := scope.ResolveSymbolAsVariable(f.Module); !variable.IsNil() {
			v := variable.Unwrap()
			if v.DropFlag != "" {
				if hook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookDrop); hook != nil && len(hook.Methods) > 0 && f.Function == hook.Methods[0] {
					if CurrentTypeState != nil && !CGetScopeInformation(scope).PreparingDefer {
						CurrentTypeState.SetMoved(v.GetFullName())
					}
					return "({ " + v.DropFlag + " = 0; " + call + "; })"
				}
			}
		}
	}
	return call
}
