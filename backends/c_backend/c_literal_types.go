// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// isPrimitiveType checks if a type name is a primitive type
func isPrimitiveType(typeName string) bool {
	primitives := map[string]bool{
		"int":     true,
		"int8":    true,
		"int16":   true,
		"int32":   true,
		"int64":   true,
		"uint":    true,
		"uint8":   true,
		"uint16":  true,
		"uint32":  true,
		"uint64":  true,
		"bool":    true,
		"float":   true,
		"float32": true,
		"float64": true,
		"string":  true,
		"void":    true,
	}
	return primitives[typeName]
}

func cloneTypeRefShallow(t *tokens.TypeRef) *tokens.TypeRef {
	if t == nil {
		return nil
	}
	return &tokens.TypeRef{
		Array:    t.Array,
		Size:     t.Size,
		FuncType: t.FuncType,
		Module:   t.Module,
		Type:     t.Type,
		TypeArgs: t.TypeArgs,
		Trait:    t.Trait,
		Const:    t.Const,
		Volatile: t.Volatile,
		Pointer:  t.Pointer,
		NonNull:  t.NonNull,
	}
}

func cloneTypeRefForInference(t *tokens.TypeRef) *tokens.TypeRef {
	if t == nil {
		return nil
	}
	out := cloneTypeRefShallow(t)
	if len(t.TypeArgs) > 0 {
		out.TypeArgs = make([]*tokens.TypeRef, len(t.TypeArgs))
		for i, arg := range t.TypeArgs {
			out.TypeArgs[i] = cloneTypeRefForInference(arg)
		}
	}
	return out
}

// refineTypeWithTypeState upgrades pointer nullability for a symbol if flow analysis
// has proven it non-null in the current branch.
func refineTypeWithTypeState(symbol string, baseType *tokens.TypeRef) *tokens.TypeRef {
	t := cloneTypeRefShallow(baseType)
	if t == nil || !t.Pointer || t.NonNull || CurrentTypeState == nil {
		return t
	}
	refined := CurrentTypeState.Lookup(symbol)
	if refined != nil && refined.IsNonNull {
		t.NonNull = true
	}
	return t
}

func reportUseAfterMoveIfNeeded(scope *ast.Ast, fullVarName string, symbol string, pos lexer.Position) {
	if CurrentTypeState == nil || scope == nil || scope.ErrorScope == nil {
		return
	}
	if !CurrentTypeState.IsMoved(fullVarName) {
		return
	}
	scope.ErrorScope.NewCompileTimeError(
		"Move Error",
		"use after move: '"+symbol+"' has been moved\nhelp: reinitialize '"+symbol+"' before use or use Clone/Rc for shared ownership",
		pos,
	)
}

// GetTypeOfLiteral attempts to determine the type of a literal expression
func (impl *CBackendImplementation) GetTypeOfLiteral(l *tokens.Literal, scope *ast.Ast) *tokens.TypeRef {
	if l == nil {
		return nil
	}
	if CurrentSemanticProgram != nil {
		if inferred := CurrentSemanticProgram.TypeOfLiteral(l); inferred != nil {
			return inferred
		}
	}

	// Handle address-of operator (&variable)
	// Get the base type first, then wrap in pointer if needed
	baseType := impl.getBaseLiteralType(l, scope)
	if baseType != nil && l.IsPointer {
		// &variable produces a non-null pointer (address of local can't be null)
		return &tokens.TypeRef{
			Type:    baseType.Type,
			Pointer: true,
			NonNull: true,
		}
	}
	return baseType
}

func (impl *CBackendImplementation) getBaseLiteralType(l *tokens.Literal, scope *ast.Ast) *tokens.TypeRef {
	if l == nil {
		return nil
	}
	if l.Intrinsic != nil {
		i := l.Intrinsic
		switch i.Name {
		case "move":
			if len(i.Args) == 1 {
				return impl.GetTypeOfExpression(i.Args[0], scope)
			}
		case "alloc":
			return &tokens.TypeRef{Type: "void", Pointer: true}
		case "size_of", "align_of":
			return &tokens.TypeRef{Type: "uint64"}
		case "deref", "read_volatile":
			if len(i.Args) == 1 {
				t := cloneTypeRefForInference(impl.GetTypeOfExpression(i.Args[0], scope))
				if t != nil && t.Pointer {
					t.Pointer = false
					t.NonNull = false
					t.Const = false
					t.Volatile = false
					return t
				}
			}
		}
	}

	// Number literals - default to int32
	if l.Number != "" {
		return &tokens.TypeRef{Type: "int32"}
	}

	// Boolean literals
	if l.Bool != "" {
		return &tokens.TypeRef{Type: "bool"}
	}

	// String literals
	if l.HasStringLiteral() {
		return &tokens.TypeRef{Type: "string"}
	}

	// Null pointer literals (nil/null) are compatible with any pointer type.
	if (l.Symbol == "nil" || l.Symbol == "null") && l.SymbolModule == "" && len(l.Chain) == 0 {
		return &tokens.TypeRef{Type: "void", Pointer: true}
	}

	// Handle chain access FIRST (e.g., sq.area(), rect.width)
	// This must come before simple symbol lookup to handle method calls
	if len(l.Chain) > 0 {
		// Get the base type from the symbol
		var currentType *tokens.TypeRef
		if l.Symbol != "" {
			varOpt := scope.ResolveSymbolAsVariable(l.Symbol)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				fullName := variable.GetFullName()
				reportUseAfterMoveIfNeeded(scope, fullName, l.Symbol, l.Pos)
				if info, ok := (*CProgramValues)[fullName]; ok {
					currentType = refineTypeWithTypeState(l.Symbol, info.GeckoType)
				}
			}
		}

		// Walk through the chain to determine the final type
		for _, chain := range l.Chain {
			if currentType == nil {
				break
			}

			// Handle builtin pointer methods before class lookup
			if currentType.Pointer && chain.IsMethodCall() {
				switch chain.Name {
				case "read":
					// ptr.read() returns the element type (dereference)
					currentType = &tokens.TypeRef{Type: currentType.Type}
					continue
				case "write":
					// ptr.write(value) returns void
					currentType = &tokens.TypeRef{Type: "void"}
					continue
				}
			}

			typeName := currentType.Type
			rootScope := scope.GetRoot()
			classOpt := rootScope.ResolveClass(typeName)
			if classOpt.IsNil() {
				currentType = nil
				break
			}
			class := classOpt.Unwrap()

			if chain.IsMethodCall() {
				// Method call - look up return type
				// First check direct class methods
				if method, ok := class.Methods[chain.Name]; ok {
					currentType = &tokens.TypeRef{Type: method.Type}
					continue
				}

				// Then check trait methods
				for _, traitMethods := range class.Traits {
					if traitMethods == nil {
						continue
					}
					for _, method := range *traitMethods {
						// Trait method names are mangled: TypeName__TraitName__methodName
						// We need to match just the methodName part
						if len(method.Name) > len(chain.Name) {
							suffix := "__" + chain.Name
							if len(method.Name) >= len(suffix) && method.Name[len(method.Name)-len(suffix):] == suffix {
								currentType = &tokens.TypeRef{Type: method.Type}
								goto foundMethod
							}
						}
					}
				}
				// Method not found
				currentType = nil
			foundMethod:
			} else {
				// Field access - look up field type
				if fieldVar, ok := class.Variables[chain.Name]; ok {
					fieldFullName := fieldVar.GetFullName()
					if fieldInfo, ok := (*CProgramValues)[fieldFullName]; ok {
						currentType = fieldInfo.GeckoType
					} else {
						currentType = nil
					}
				} else {
					currentType = nil
				}
			}
		}

		return currentType
	}

	// Symbol - look up in scope (only if no chain)
	if l.Symbol != "" {
		if l.SymbolModule != "" {
			// module.field or var.field - get field type
			varOpt := scope.ResolveSymbolAsVariable(l.SymbolModule)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				fullName := variable.GetFullName()
				reportUseAfterMoveIfNeeded(scope, fullName, l.SymbolModule, l.Pos)
				if info, ok := (*CProgramValues)[fullName]; ok && info.GeckoType != nil {
					typeName := info.GeckoType.Type
					rootScope := scope.GetRoot()
					classOpt := rootScope.ResolveClass(typeName)
					if !classOpt.IsNil() {
						class := classOpt.Unwrap()
						if fieldVar, ok := class.Variables[l.Symbol]; ok {
							fieldFullName := fieldVar.GetFullName()
							if fieldInfo, ok := (*CProgramValues)[fieldFullName]; ok {
								return fieldInfo.GeckoType
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
				reportUseAfterMoveIfNeeded(scope, fullName, l.Symbol, l.Pos)
				if info, ok := (*CProgramValues)[fullName]; ok {
					return refineTypeWithTypeState(l.Symbol, info.GeckoType)
				}
			}
		}
	}

	// Struct literal
	if l.IsStructLiteral() {
		return &tokens.TypeRef{Type: l.StructType, TypeArgs: l.StructTypeArgs}
	}

	// Function call - would need return type analysis
	if l.FuncCall != nil {
		return impl.GetTypeOfFuncCall(l.FuncCall, scope)
	}

	return nil
}
