// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strconv"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

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
	} else if l.HasStringLiteral() {
		// String literal - the & in gecko means we want a pointer to the string
		// In C, string literals are already const char*
		cLiteral, err := l.CStringLiteral()
		if err != nil {
			scope.ErrorScope.NewCompileTimeError("String Escape", "unable to escape the string provided "+err.Error(), l.Pos)
			base = "\"\""
		} else {
			base = cLiteral
		}
	} else if l.Symbol != "" {
		symbolName := l.Symbol
		if l.SymbolModule != "" {
			// Could be module.SYMBOL or struct.field
			// First check if SymbolModule is a local variable (struct field access)
			structVar := scope.ResolveSymbolAsVariable(l.SymbolModule)
			if !structVar.IsNil() {
				variable := structVar.Unwrap()
				reportUseAfterMoveIfNeeded(scope, variable.GetFullName(), l.SymbolModule, l.Pos)
				info := CGetScopeInformation(scope)
				varName := resolveVariableWithIdentifier(variable, scope, info)
				// Check if it's a pointer - use -> instead of .
				if variable.IsPointer {
					if !variable.IsReceiver {
						requireUnsafe(scope, l.Pos, "raw pointer field access")
					}
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
			// Handle null pointer literals
			if symbolName == "nil" || symbolName == "null" {
				base = "NULL"
			} else {
				// Local symbol - try variable first, then function
				symbolVariable := scope.ResolveSymbolAsVariable(symbolName)
				if !symbolVariable.IsNil() {
					variable := symbolVariable.Unwrap()
					reportUseAfterMoveIfNeeded(scope, variable.GetFullName(), symbolName, l.Pos)
					info := CGetScopeInformation(scope)
					base = resolveVariableWithIdentifier(variable, scope, info)
				} else {
					// Try to resolve as a function reference (for function pointers)
					symbolMethod := scope.ResolveMethod(symbolName)
					if !symbolMethod.IsNil() {
						method := symbolMethod.Unwrap()
						base = method.CIdentifier()
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
	} else if l.Lambda != nil {
		base = impl.LambdaToCString(l.Lambda, scope)
	} else if l.Match != nil {
		base = impl.MatchAsExpressionToCString(l.Match, scope)
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
			typeArgStrs := make([]string, len(l.StructTypeArgs))
			for i, arg := range l.StructTypeArgs {
				typeArgStrs[i] = TypeRefToCType(arg, scope)
			}
			mangledType = mangleName(l.StructType, typeArgStrs)
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
		baseLiteral := *l
		baseLiteral.ArrayIndex = nil
		baseLiteral.IsPointer = false
		if t := impl.GetTypeOfLiteral(&baseLiteral, scope); t != nil && t.Pointer {
			requireUnsafe(scope, l.Pos, "raw pointer indexing")
		}
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
				indexHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookIndex)
				if indexHook != nil && len(indexHook.Methods) > 0 {
					for traitName := range class.Traits {
						if TraitMatchesOrExtends(traitName, indexHook.TraitName) {
							methodName := indexHook.Methods[0]
							mangledMethod := indexedTypeName + "__" + traitName + "__" + methodName
							base = mangledMethod + "(&" + base + ", " + indexExpr + ")"
							indexed = true
							break
						}
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

	// Increment/decrement in expression context is an error
	if l.IncDec != nil {
		scope.ErrorScope.NewError(
			"E0001",
			"Increment/Decrement as Expression",
			"++ and -- operators cannot be used as expressions",
			l.IncDec.Pos,
			"use `x = x + 1` or `x = x - 1` instead",
		)
	}

	return base
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
