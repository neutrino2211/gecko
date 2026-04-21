package cbackend

import (
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

// ExpressionToCString converts an expression to C code
func (impl *CBackendImplementation) ExpressionToCString(e *tokens.Expression, scope *ast.Ast) string {
	if e == nil {
		return ""
	}
	return impl.OrExpressionToCString(e.OrExpr, scope)
}

// OrExpressionToCString handles the 'or' keyword for error handling
func (impl *CBackendImplementation) OrExpressionToCString(o *tokens.OrExpression, scope *ast.Ast) string {
	if o == nil {
		return ""
	}

	base := impl.LogicalOrToCString(o.LogicalOr, scope)

	// Handle 'or' keyword for fallback values
	if o.Or != nil {
		leftType := impl.GetTypeOfLogicalOr(o.LogicalOr, scope)
		defaultValue := impl.OrExpressionToCString(o.Or, scope)

		// Check if this type implements @or_hook (Orable trait)
		if leftType != nil && leftType.Type != "" {
			baseTypeName := leftType.Type
			typeName := baseTypeName

			// Collect C type args for mangling
			var typeArgStrs []string
			if len(leftType.TypeArgs) > 0 {
				typeArgStrs = make([]string, len(leftType.TypeArgs))
				for i, arg := range leftType.TypeArgs {
					if cType, ok := GeckoToCType[arg.Type]; ok {
						typeArgStrs[i] = cType
					} else {
						typeArgStrs[i] = arg.Type
					}
				}
				typeName = mangleName(baseTypeName, typeArgStrs)
			}

			orHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookOr)
			if orHook != nil {
				_, found := impl.GetOperatorTraitName(typeName, orHook.TraitName, scope)
				if found && len(orHook.Methods) > 0 {
					methodName := orHook.Methods[0]

					// Build the substituted trait name (e.g., Orable__T -> Orable__int32_t)
					mangledTraitName := orHook.TraitName
					if len(typeArgStrs) > 0 {
						for _, arg := range typeArgStrs {
							mangledTraitName += "__" + arg
						}
					}

					// Only add module prefix for generic types (detected by having type args)
					var fullMethodName string
					if len(typeArgStrs) > 0 {
						modulePrefix := scope.GetRoot().Scope
						if modulePrefix != "" {
							modulePrefix += "__"
						}
						fullMethodName = modulePrefix + typeName + "__" + mangledTraitName + "__" + methodName
					} else {
						fullMethodName = typeName + "__" + mangledTraitName + "__" + methodName
					}
					return fullMethodName + "(&(" + base + "), " + defaultValue + ")"
				}
			}
		}

		// Fallback: just use C's ternary (for simple cases like pointers)
		return "(" + base + " ? " + base + " : " + defaultValue + ")"
	}

	return base
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

		// Handle 'try' operator for error handling
		if u.Op == "try" {
			if traitCall, ok := impl.GetTryOperatorCall(innerCode, innerType, scope, u.Pos); ok {
				base = traitCall
			} else {
				// Fallback: just return the inner expression (for types without @try_hook)
				base = innerCode
			}
		} else if traitCall, ok := impl.GetUnaryOperatorTraitMethodCall(innerCode, innerType, u.Op, scope); ok {
			// Try operator overloading for other unary operators
			base = traitCall
		} else {
			base = u.Op + innerCode
		}
	} else if u.Primary != nil {
		base = impl.PrimaryToCString(u.Primary, scope)
	}

	// Handle cast expression (e.g., "value as *uint16" or "ptr as uint64")
	if u.Cast != nil {
		// Validate the cast target type
		if u.Cast.Type != nil {
			u.Cast.Type.Check(scope)
		}
		cType := TypeRefToCType(u.Cast.Type, scope)
		base = "((" + cType + ")(" + base + "))"
	}

	return base
}

// tryTempCounter generates unique names for try temporaries
var tryTempCounter int = 0

// getCurrentFuncReturnType walks up the scope hierarchy to find the current function's return type
func getCurrentFuncReturnType(scope *ast.Ast) *tokens.TypeRef {
	currentScope := scope
	for currentScope != nil {
		info, ok := (*CScopeDataMap)[currentScope.GetFullName()]
		if ok && info.CurrentFuncReturnType != nil {
			return info.CurrentFuncReturnType
		}
		currentScope = currentScope.Parent
	}
	return nil
}

// GetTryOperatorCall generates code for the 'try' operator with early return semantics
func (impl *CBackendImplementation) GetTryOperatorCall(operandCode string, operandType *tokens.TypeRef, scope *ast.Ast, pos lexer.Position) (string, bool) {
	if operandType == nil || operandType.Type == "" {
		return "", false
	}

	// Get the try hook (requires both has_value and try_unwrap methods)
	tryHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookTry)
	if tryHook == nil || len(tryHook.Methods) < 2 {
		return "", false
	}

	baseTypeName := operandType.Type
	typeName := baseTypeName

	// Collect C type args for mangling
	var typeArgStrs []string
	if len(operandType.TypeArgs) > 0 {
		typeArgStrs = make([]string, len(operandType.TypeArgs))
		for i, arg := range operandType.TypeArgs {
			if cType, ok := GeckoToCType[arg.Type]; ok {
				typeArgStrs[i] = cType
			} else {
				typeArgStrs[i] = arg.Type
			}
		}
		typeName = mangleName(baseTypeName, typeArgStrs)
	}

	// Check if the operand type implements Tryable
	_, found := impl.GetOperatorTraitName(typeName, tryHook.TraitName, scope)
	if !found {
		return "", false
	}

	// Check if the current function's return type implements Tryable
	funcReturnType := getCurrentFuncReturnType(scope)
	if funcReturnType == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Try Expression Error",
			"'try' can only be used inside a function",
			pos,
		)
		return "", false
	}

	// Build the return type name for trait lookup
	returnTypeName := funcReturnType.Type
	var returnTypeArgStrs []string
	if len(funcReturnType.TypeArgs) > 0 {
		returnTypeArgStrs = make([]string, len(funcReturnType.TypeArgs))
		for i, arg := range funcReturnType.TypeArgs {
			if cType, ok := GeckoToCType[arg.Type]; ok {
				returnTypeArgStrs[i] = cType
			} else {
				returnTypeArgStrs[i] = arg.Type
			}
		}
		returnTypeName = mangleName(funcReturnType.Type, returnTypeArgStrs)
	}

	// Check if the function's return type implements Tryable
	_, returnTypeHasTryable := impl.GetOperatorTraitName(returnTypeName, tryHook.TraitName, scope)
	if !returnTypeHasTryable {
		scope.ErrorScope.NewCompileTimeError(
			"Try Expression Error",
			"'try' can only be used in functions that return a type implementing Tryable (e.g., Option<T>, Result<T, E>). "+
				"Function returns '"+funcReturnType.Type+"' which does not implement @try_hook trait.",
			pos,
		)
		return "", false
	}

	// Build method names: has_value and try_unwrap
	hasValueMethod := tryHook.Methods[0]  // has_value
	tryUnwrapMethod := tryHook.Methods[1] // try_unwrap

	// Build the substituted trait name (e.g., Tryable__T -> Tryable__int32_t)
	mangledTraitName := tryHook.TraitName
	if len(typeArgStrs) > 0 {
		for _, arg := range typeArgStrs {
			mangledTraitName += "__" + arg
		}
	}

	// Build method prefix (with or without module prefix for generic types)
	var methodPrefix string
	if len(typeArgStrs) > 0 {
		modulePrefix := scope.GetRoot().Scope
		if modulePrefix != "" {
			modulePrefix += "__"
		}
		methodPrefix = modulePrefix + typeName + "__" + mangledTraitName + "__"
	} else {
		methodPrefix = typeName + "__" + mangledTraitName + "__"
	}

	hasValueCall := methodPrefix + hasValueMethod
	tryUnwrapCall := methodPrefix + tryUnwrapMethod

	// Get the C type for the operand (use mangled typeName for generic types)
	cTypeName := typeName

	// Generate unique temp variable name
	tryTempCounter++
	tempVar := "__try_tmp_" + strconv.Itoa(tryTempCounter)

	// Generate GCC statement expression with early return:
	// ({ Type tmp = expr; if (!has_value(&tmp)) return tmp; try_unwrap(&tmp); })
	code := "({ " + cTypeName + " " + tempVar + " = " + operandCode + "; " +
		"if (!" + hasValueCall + "(&" + tempVar + ")) return " + tempVar + "; " +
		tryUnwrapCall + "(&" + tempVar + "); })"

	return code, true
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
		symbolName := l.Symbol
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
					base = varName + "->" + symbolName
				} else {
					base = varName + "." + symbolName
				}
			} else {
				// Module-prefixed symbol: module.SYMBOL
				rootScope := scope
				for rootScope.Parent != nil {
					rootScope = rootScope.Parent
				}

				if importedModule, ok := rootScope.Children[l.SymbolModule]; ok {
					symbolVariable := importedModule.ResolveSymbolAsVariable(symbolName)
					if !symbolVariable.IsNil() {
						variable := symbolVariable.Unwrap()
						base = variable.GetFullName()
					} else {
						base = l.SymbolModule + "__" + symbolName
					}
				} else {
					base = l.SymbolModule + "__" + symbolName
				}
			}
		} else {
			// Handle nil literal
			if symbolName == "nil" {
				base = "NULL"
			} else {
				// Local symbol - try variable first, then function
				symbolVariable := scope.ResolveSymbolAsVariable(symbolName)
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
					symbolMethod := scope.ResolveMethod(symbolName)
					if !symbolMethod.IsNil() {
						method := symbolMethod.Unwrap()
						base = method.GetFullName()
					} else {
						// Could be an unknown symbol, use as-is
						base = symbolName
					}
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
		// Type check struct literal fields
		impl.CheckStructLiteralTypes(l.StructType, l.StructFields, scope, l.Pos)

		// Generate C99 compound literal: (TypeName){ .field = value, ... }
		// For generic types, mangle: Box<T> -> Box__T
		mangledType := l.StructType
		if len(l.StructTypeArgs) > 0 {
			for _, arg := range l.StructTypeArgs {
				mangledType += "__" + TypeRefToCType(arg, scope)
			}
		}
		base = "(" + mangledType + "){ "
		for i, kv := range l.StructFields {
			if i > 0 {
				base += ", "
			}
			base += "." + kv.Key + " = " + impl.ExpressionToCString(kv.Value, scope)
		}
		base += " }"
	}

	// Handle chained access (.field or .method()) BEFORE array indexing
	// This ensures self.buffer[i] becomes self->buffer[i], not self[i]->buffer
	if len(l.Chain) > 0 {
		base = impl.processChain(base, l, scope)
	}

	// Handle array indexing - check for Index trait first
	if l.ArrayIndex != nil {
		indexed := false
		indexExpr := impl.ExpressionToCString(l.ArrayIndex, scope)

		// Try to get the type of the indexed expression
		// For chain access like self.buffer[i], we need the type after the chain
		var indexedTypeName string
		if len(l.Chain) > 0 {
			// Get the type of the last chain element
			indexedType := impl.GetTypeOfLiteral(l, scope)
			if indexedType != nil {
				// ArrayIndex applies to the result, so we need the element type, not array type
				// For now, skip Index trait check for chained access
				indexedTypeName = ""
			}
		} else if l.Symbol != "" {
			varOpt := scope.ResolveSymbolAsVariable(l.Symbol)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				fullName := variable.GetFullName()
				if valueInfo, ok := (*CProgramValues)[fullName]; ok && valueInfo.GeckoType != nil {
					indexedTypeName = valueInfo.GeckoType.Type
				}
			}
		}

		// Check for Index trait
		if indexedTypeName != "" {
			if classOpt := scope.ResolveClass(indexedTypeName); !classOpt.IsNil() {
				class := classOpt.Unwrap()
				// Check for Index trait with any type argument
				for traitName := range class.Traits {
					if len(traitName) >= 5 && traitName[:5] == "Index" {
						// Found Index trait - get the index method from any module (trait may be imported)
						indexHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookIndex)
						if indexHook != nil && len(indexHook.Methods) > 0 {
							methodName := indexHook.Methods[0]
							mangledMethod := indexedTypeName + "__" + traitName + "__" + methodName
							base = mangledMethod + "(&" + base + ", " + indexExpr + ")"
							indexed = true
						}
						break
					}
				}
			}
		}

		if !indexed {
			base += "[" + indexExpr + "]"
		}
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
	var isModule bool        // Track if base is a module, not a variable
	var moduleName string    // The module name for module.constant/function patterns

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
				if _, ok := checkScope.Children[symbolName]; ok {
					isModule = true
					moduleName = symbolName
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
					args += impl.ExpressionToCString(arg.Value, scope)
				}
				// Generate: module__func(args)
				funcName := moduleName + "__" + chain.Name
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
				fullName := variable.GetFullName()
				// First check if the variable itself is a pointer (like self in trait methods)
				isPointer = variable.IsPointer
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

	for _, chain := range l.Chain {
		if chain.IsMethodCall() {
			// Method call - check for builtin trait methods first
			if currentType != nil {
				if code, ok := impl.TryBuiltinMethod(result, currentType, chain.Name, chain.GetArgs(), scope); ok {
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
			for i, arg := range chain.GetArgs() {
				if i > 0 {
					args += ", "
				}
				args += impl.ExpressionToCString(arg.Value, scope)
			}

			// Try to find the method in the type's class or traits
			if currentType != nil {
				resolver := NewMethodResolver(impl)
				resolution := resolver.ResolveMethod(currentType, chain.Name, scope)

				if resolution.Found {
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

	// Handle static type calls: module.Type<Args>::function() or Type::function()
	if f.StaticType != "" {
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
			lookupScope = rootScope
		}

		if isGenericInstance {
			// Generic type instantiation - build mangled name
			typeArgStrs := make([]string, len(f.StaticTypeArgs))
			for i, typeArg := range f.StaticTypeArgs {
				typeArgStrs[i] = TypeRefToCType(typeArg, scope)
			}

			// Validate class type parameter constraints
			impl.ValidateClassTypeArgs(f.StaticType, f.StaticTypeArgs, scope, f.Pos)

			typeName = typeName + "__" + strings.Join(typeArgStrs, "__")

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
						// Check method visibility for cross-module calls
						if visErr := method.CheckVisibility(scope); visErr != "" {
							scope.ErrorScope.NewCompileTimeError(
								"Visibility Error",
								visErr,
								f.Pos,
							)
						}
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
				resolver := NewMethodResolver(impl)

				// Check if this is a generic type parameter with a trait constraint
				if constrainedName, ok := resolver.ResolveConstrainedGeneric(typeName, f.Function); ok {
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
