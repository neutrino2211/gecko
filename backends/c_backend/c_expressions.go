package cbackend

import (
	"strconv"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// ExpressionToCString converts an expression to C code
func (impl *CBackendImplementation) ExpressionToCString(e *tokens.Expression, scope *ast.Ast) string {
	if e == nil {
		return ""
	}
	return impl.LogicalOrToCString(e.LogicalOr, scope)
}

// LogicalOrToCString converts logical OR expressions
func (impl *CBackendImplementation) LogicalOrToCString(lo *tokens.LogicalOr, scope *ast.Ast) string {
	if lo == nil {
		return ""
	}

	base := impl.LogicalAndToCString(lo.LogicalAnd, scope)

	if lo.Next != nil {
		base += " || " + impl.LogicalOrToCString(lo.Next, scope)
	}

	return base
}

// LogicalAndToCString converts logical AND expressions
func (impl *CBackendImplementation) LogicalAndToCString(la *tokens.LogicalAnd, scope *ast.Ast) string {
	if la == nil {
		return ""
	}

	base := impl.EqualityToCString(la.Equality, scope)

	if la.Next != nil {
		base += " && " + impl.LogicalAndToCString(la.Next, scope)
	}

	return base
}

// EqualityToCString converts equality expressions
func (impl *CBackendImplementation) EqualityToCString(eq *tokens.Equality, scope *ast.Ast) string {
	if eq == nil {
		return ""
	}

	base := impl.ComparisonToCString(eq.Comparison, scope)

	if eq.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfComparison(eq.Comparison, scope)
		rightCode := impl.EqualityToCString(eq.Next, scope)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, eq.Op, scope); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + eq.Op + " " + rightCode
	}

	return base
}

// ComparisonToCString converts comparison expressions
func (impl *CBackendImplementation) ComparisonToCString(c *tokens.Comparison, scope *ast.Ast) string {
	if c == nil {
		return ""
	}

	base := impl.AdditionToCString(c.Addition, scope)

	if c.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfAddition(c.Addition, scope)
		rightCode := impl.ComparisonToCString(c.Next, scope)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, c.Op, scope); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + c.Op + " " + rightCode
	}

	return base
}

// AdditionToCString converts addition expressions
func (impl *CBackendImplementation) AdditionToCString(a *tokens.Addition, scope *ast.Ast) string {
	if a == nil {
		return ""
	}

	base := impl.MultiplicationToCString(a.Multiplication, scope)

	if a.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfMultiplication(a.Multiplication, scope)
		rightCode := impl.AdditionToCString(a.Next, scope)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, a.Op, scope); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + a.Op + " " + rightCode
	}

	return base
}

// MultiplicationToCString converts multiplication expressions
func (impl *CBackendImplementation) MultiplicationToCString(m *tokens.Multiplication, scope *ast.Ast) string {
	if m == nil {
		return ""
	}

	base := impl.UnaryToCString(m.Unary, scope)

	if m.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfUnary(m.Unary, scope)
		rightCode := impl.MultiplicationToCString(m.Next, scope)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, m.Op, scope); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + m.Op + " " + rightCode
	}

	return base
}

// UnaryToCString converts unary expressions
func (impl *CBackendImplementation) UnaryToCString(u *tokens.Unary, scope *ast.Ast) string {
	if u == nil {
		return ""
	}

	var base string

	if u.Unary != nil {
		innerCode := impl.UnaryToCString(u.Unary, scope)
		innerType := impl.GetTypeOfUnary(u.Unary, scope)

		// Try operator overloading for unary operators
		if traitCall, ok := impl.GetUnaryOperatorTraitMethodCall(innerCode, innerType, u.Op, scope); ok {
			base = traitCall
		} else {
			base = u.Op + innerCode
		}
	} else if u.Primary != nil {
		base = impl.PrimaryToCString(u.Primary, scope)
	}

	// Handle cast expression (e.g., "value as *uint16" or "ptr as uint64")
	if u.Cast != nil {
		cType := TypeRefToCType(u.Cast.Type, scope)
		base = "((" + cType + ")(" + base + "))"
	}

	return base
}

// PrimaryToCString converts primary expressions
func (impl *CBackendImplementation) PrimaryToCString(p *tokens.Primary, scope *ast.Ast) string {
	if p == nil {
		return ""
	}

	if p.SubExpression != nil {
		return "(" + impl.ExpressionToCString(p.SubExpression, scope) + ")"
	}

	if p.Literal != nil {
		return impl.LiteralToCString(p.Literal, scope)
	}

	return ""
}

// LiteralToCString converts literals to C code
func (impl *CBackendImplementation) LiteralToCString(l *tokens.Literal, scope *ast.Ast) string {
	if l == nil {
		return ""
	}

	base := ""

	if l.Bool != "" {
		if l.Bool == "true" {
			base = "1"
		} else {
			base = "0"
		}
	} else if l.String != "" {
		// String literal - the & in gecko means we want a pointer to the string
		// In C, string literals are already const char*
		base = l.String
	} else if l.Symbol != "" {
		if l.SymbolModule != "" {
			// Could be module.SYMBOL or struct.field
			// First check if SymbolModule is a local variable (struct field access)
			structVar := scope.ResolveSymbolAsVariable(l.SymbolModule)
			if !structVar.IsNil() {
				variable := structVar.Unwrap()
				varName := variable.Name
				if !variable.IsArgument && variable.Parent != scope {
					varName = variable.GetFullName()
				}
				// Check if it's a pointer - use -> instead of .
				if variable.IsPointer {
					base = varName + "->" + l.Symbol
				} else {
					base = varName + "." + l.Symbol
				}
			} else {
				// Module-prefixed symbol: module.SYMBOL
				rootScope := scope
				for rootScope.Parent != nil {
					rootScope = rootScope.Parent
				}

				if importedModule, ok := rootScope.Children[l.SymbolModule]; ok {
					symbolVariable := importedModule.ResolveSymbolAsVariable(l.Symbol)
					if !symbolVariable.IsNil() {
						variable := symbolVariable.Unwrap()
						base = variable.GetFullName()
					} else {
						base = l.SymbolModule + "__" + l.Symbol
					}
				} else {
					base = l.SymbolModule + "__" + l.Symbol
				}
			}
		} else {
			// Local symbol - try variable first, then function
			symbolVariable := scope.ResolveSymbolAsVariable(l.Symbol)
			if !symbolVariable.IsNil() {
				variable := symbolVariable.Unwrap()
				// For local variables and arguments, use just the name (not the full qualified name)
				if variable.IsArgument || variable.Parent == scope {
					base = variable.Name
				} else {
					base = variable.GetFullName()
				}
			} else {
				// Try to resolve as a function reference (for function pointers)
				symbolMethod := scope.ResolveMethod(l.Symbol)
				if !symbolMethod.IsNil() {
					method := symbolMethod.Unwrap()
					base = method.GetFullName()
				} else {
					// Could be an unknown symbol, use as-is
					base = l.Symbol
				}
			}
		}
	} else if l.Number != "" {
		base = l.Number
	} else if l.Intrinsic != nil {
		base = impl.IntrinsicToCString(l.Intrinsic, scope)
	} else if l.FuncCall != nil {
		base = impl.FuncCallToCString(l.FuncCall, scope)
	} else if len(l.Array) > 0 {
		base = "{"
		for i, arrayLit := range l.Array {
			if i > 0 {
				base += ", "
			}
			base += impl.LiteralToCString(arrayLit, scope)
		}
		base += "}"
	} else if l.IsStructLiteral() {
		// Handle struct literal: TypeName { field: value, ... }
		// Generate C99 compound literal: (TypeName){ .field = value, ... }
		base = "(" + l.StructType + "){ "
		for i, kv := range l.StructFields {
			if i > 0 {
				base += ", "
			}
			base += "." + kv.Key + " = " + impl.ExpressionToCString(kv.Value, scope)
		}
		base += " }"
	}

	// Handle array indexing
	if l.ArrayIndex != nil {
		base += "[" + impl.LiteralToCString(l.ArrayIndex, scope) + "]"
	}

	// Handle chained access (.field or .method())
	if len(l.Chain) > 0 {
		base = impl.processChain(base, l, scope)
	}

	// Handle address-of operator
	if l.IsPointer {
		base = "&" + base
	}

	return base
}

// processChain handles chained field/method access like a.b.c() or ptr.field.method()
func (impl *CBackendImplementation) processChain(base string, l *tokens.Literal, scope *ast.Ast) string {
	// Track the current type as we traverse the chain
	var currentType *tokens.TypeRef
	var isPointer bool

	// Try to get the initial type from the literal
	if l.Symbol != "" {
		if l.SymbolModule != "" {
			// module.field pattern - base is already "module.field" or "var.field"
			// Get the type of the field, not the module/var
			varOpt := scope.ResolveSymbolAsVariable(l.SymbolModule)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				fullName := variable.GetFullName()
				if info, ok := (*CProgramValues)[fullName]; ok && info.GeckoType != nil {
					// Get the class to find the field type
					typeName := info.GeckoType.Type
					rootScope := scope.GetRoot()
					classOpt := rootScope.ResolveClass(typeName)
					if !classOpt.IsNil() {
						class := classOpt.Unwrap()
						if fieldVar, ok := class.Variables[l.Symbol]; ok {
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
			varOpt := scope.ResolveSymbolAsVariable(l.Symbol)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				fullName := variable.GetFullName()
				if info, ok := (*CProgramValues)[fullName]; ok {
					currentType = info.GeckoType
					isPointer = currentType != nil && currentType.Pointer
				}
			}
		}
	}

	result := base

	for _, chain := range l.Chain {
		if chain.IsMethodCall() {
			// Method call - check for builtin trait methods first
			if currentType != nil {
				if code, ok := impl.TryBuiltinMethod(result, currentType, chain.Name, chain.Args, scope); ok {
					result = code
					// After a method call, we don't know the return type without more analysis
					currentType = nil
					isPointer = false
					continue
				}
			}

			// Regular method call - convert to function call with self argument
			// For now, generate as obj.method(args) -> Type__method(&obj, args)
			args := ""
			for i, arg := range chain.Args {
				if i > 0 {
					args += ", "
				}
				args += impl.ExpressionToCString(arg.Value, scope)
			}

			// Try to find the method in the type's class or traits
			if currentType != nil {
				typeName := currentType.Type
				rootScope := scope.GetRoot()
				classOpt := rootScope.ResolveClass(typeName)
				if !classOpt.IsNil() {
					class := classOpt.Unwrap()

					// First check for direct class methods
					if method, ok := class.Methods[chain.Name]; ok {
						selfArg := result
						if !isPointer {
							selfArg = "&" + result
						}
						// Use the full method name from the AST
						methodName := class.GetFullName() + "__" + method.Name
						if args != "" {
							result = methodName + "(" + selfArg + ", " + args + ")"
						} else {
							result = methodName + "(" + selfArg + ")"
						}
						currentType = nil
						isPointer = false
						goto nextChain
					}

					// Then check trait methods
					for traitName, traitMethods := range class.Traits {
						if traitMethods == nil {
							continue
						}
						for _, method := range *traitMethods {
							expectedName := typeName + "__" + traitName + "__" + chain.Name
							if method.Name == expectedName {
								selfArg := result
								if !isPointer {
									selfArg = "&" + result
								}
								if args != "" {
									result = expectedName + "(" + selfArg + ", " + args + ")"
								} else {
									result = expectedName + "(" + selfArg + ")"
								}
								currentType = nil
								isPointer = false
								goto nextChain
							}
						}
					}
				}
			}

			// Fallback: generate as a regular function call
			if args != "" {
				result = chain.Name + "(" + result + ", " + args + ")"
			} else {
				result = chain.Name + "(" + result + ")"
			}
			currentType = nil
			isPointer = false
		} else {
			// Field access
			if isPointer {
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

// FuncCallToCString converts function calls to C code
func (impl *CBackendImplementation) FuncCallToCString(f *tokens.FuncCall, scope *ast.Ast) string {
	if f == nil {
		return ""
	}

	// Type check the function call arguments
	impl.CheckFunctionCallTypes(f, scope)

	var funcName string
	var baseFuncName string
	var selfArg string

	// Handle static type calls: Type<Args>::function()
	if f.StaticType != "" {
		// Build the mangled type name with type arguments
		typeName := f.StaticType
		rootScope := scope.GetRoot()
		isGenericInstance := len(f.StaticTypeArgs) > 0

		if isGenericInstance {
			// Generic type instantiation - build mangled name
			typeArgStrs := make([]string, len(f.StaticTypeArgs))
			for i, typeArg := range f.StaticTypeArgs {
				typeArgStrs[i] = TypeRefToCType(typeArg, scope)
			}
			typeName = typeName + "__" + strings.Join(typeArgStrs, "__")
			// For generic instances, just use the mangled name directly
			baseFuncName = typeName + "__" + f.Function
		} else {
			// Non-generic type - look up the class to check for direct methods and trait methods
			classOpt := rootScope.ResolveClass(f.StaticType)
			if !classOpt.IsNil() {
				class := classOpt.Unwrap()
				fullTypeName := class.GetFullName()

				// First check for direct class method
				if _, ok := class.Methods[f.Function]; ok {
					baseFuncName = fullTypeName + "__" + f.Function
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
								baseFuncName = expectedName
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
				// Class not found - use default mangling
				baseFuncName = typeName + "__" + f.Function
			}
		}
	} else if f.Module != "" {
		// Could be module.function() or variable.method() (trait call)
		// First check if it's a local variable
		varOpt := scope.ResolveSymbolAsVariable(f.Module)
		if !varOpt.IsNil() {
			// It's a variable - check for trait method call
			variable := varOpt.Unwrap()
			varName := variable.Name
			if !variable.IsArgument && variable.Parent != scope {
				varName = variable.GetFullName()
			}

			// Get the variable's type to find trait methods
			fullVarName := variable.GetFullName()
			valueInfo, hasInfo := (*CProgramValues)[fullVarName]
			if hasInfo && valueInfo.GeckoType != nil {
				// Try builtin trait methods first (e.g., ptr.deref(), ptr.is_null())
				if code, ok := impl.TryBuiltinMethod(varName, valueInfo.GeckoType, f.Function, f.Arguments, scope); ok {
					return code
				}


				typeName := valueInfo.GeckoType.Type

				// Check if this is a generic type parameter with a trait constraint
				if CurrentMonomorphContext != nil {
					if concreteType, ok := CurrentMonomorphContext.GetConcreteTypeForParam(typeName); ok {
						if traitName, hasTrait := CurrentMonomorphContext.GetTraitConstraint(typeName); hasTrait {
							// This is a constrained type parameter - use the concrete type's trait method
							expectedName := concreteType + "__" + traitName + "__" + f.Function
							baseFuncName = expectedName
							// Add self as first argument (always pointer for trait methods)
							if variable.IsPointer {
								selfArg = varName
							} else {
								selfArg = "&" + varName
							}
						}
					}
				}

				// If not a constrained generic, try regular class/trait lookup
				if baseFuncName == "" {
					// Look up the class in root scope (and imported modules)
					rootScope := scope
					for rootScope.Parent != nil {
						rootScope = rootScope.Parent
					}

					// For generic class instances, build the mangled class name
					lookupName := typeName
					isGenericInstance := len(valueInfo.GeckoType.TypeArgs) > 0
					if isGenericInstance {
						typeArgStrs := make([]string, len(valueInfo.GeckoType.TypeArgs))
						for i, typeArg := range valueInfo.GeckoType.TypeArgs {
							typeArgStrs[i] = TypeRefToCType(typeArg, scope)
						}
						lookupName = typeName + "__" + strings.Join(typeArgStrs, "__")
					} else if strings.Contains(typeName, "__") {
						// Type name is already mangled (e.g., self in generic class method)
						isGenericInstance = true
						lookupName = typeName
					}

					classOpt := rootScope.ResolveClass(lookupName)
					// If not found in current scope, search imported modules (children)
					if classOpt.IsNil() {
						for _, child := range rootScope.Children {
							classOpt = child.ResolveClass(lookupName)
							if !classOpt.IsNil() {
								break
							}
						}
					}

					// For generic class instances, directly construct the method name
					// The method name is always className__methodName (without scope prefix)
					if isGenericInstance {
						// This is a generic class instance - method name is className__methodName
						baseFuncName = lookupName + "__" + f.Function
						if variable.IsPointer {
							selfArg = varName
						} else {
							selfArg = "&" + varName
						}
					} else if !classOpt.IsNil() {
						class := classOpt.Unwrap()

						// First check for direct class methods
						if method, ok := class.Methods[f.Function]; ok {
							baseFuncName = class.GetFullName() + "__" + method.Name
							// Add self as first argument
							if variable.IsPointer {
								selfArg = varName
							} else {
								selfArg = "&" + varName
							}
						}

						// Then search traits for the method
						if baseFuncName == "" {
							for traitName, traitMethods := range class.Traits {
								if traitMethods == nil {
									continue
								}
								for _, method := range *traitMethods {
									expectedName := typeName + "__" + traitName + "__" + f.Function
									if method.Name == expectedName {
										baseFuncName = method.Name
										// Add self as first argument
										if variable.IsPointer {
											selfArg = varName
										} else {
											selfArg = "&" + varName
										}
										break
									}
								}
								if baseFuncName != "" {
									break
								}
							}
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
					baseFuncName = mth.Unwrap().GetFullName()
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
			baseFuncName = mth.Unwrap().GetFullName()
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
			args += impl.ExpressionToCString(arg.Value, scope)
		}
	}

	return funcName + "(" + args + ")"
}

// escapeString escapes special characters in a string for C
func escapeString(s string) string {
	// The string comes with quotes, so we need to handle it
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return strconv.Quote(unquoted)
}
