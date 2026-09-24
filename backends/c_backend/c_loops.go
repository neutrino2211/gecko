// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

// inferMethodCallType infers the return type of a method call expression like s.iter()
func (impl *CBackendImplementation) inferMethodCallType(expr *tokens.Expression, scope *ast.Ast) *tokens.TypeRef {
	if expr == nil || expr.GetLogicalOr() == nil {
		return nil
	}

	// Navigate to the literal
	lo := expr.GetLogicalOr()
	if lo.LogicalAnd == nil {
		return nil
	}
	la := lo.LogicalAnd
	if la.Equality == nil {
		return nil
	}
	eq := la.Equality
	if eq.Comparison == nil {
		return nil
	}
	cmp := eq.Comparison
	if cmp.Addition == nil {
		return nil
	}
	add := cmp.Addition
	if add.Multiplication == nil {
		return nil
	}
	mul := add.Multiplication
	if mul.Unary == nil {
		return nil
	}
	un := mul.Unary
	if un.Primary == nil {
		return nil
	}
	prim := un.Primary
	if prim.Literal == nil {
		return nil
	}
	lit := prim.Literal

	// Check if lit is nil
	if lit == nil {
		return nil
	}

	// Check for FuncCall directly on the literal (e.g., s.iter() parsed as FuncCall)
	if lit.FuncCall != nil {
		return impl.GetTypeOfFuncCall(lit.FuncCall, scope)
	}

	// Check for symbol with method call chain: s.iter()
	if lit.Symbol != "" && len(lit.Chain) > 0 {
		lastChain := lit.Chain[len(lit.Chain)-1]
		if lastChain.IsMethodCall() {
			funcCall := &tokens.FuncCall{
				Module:   lit.Symbol,
				Function: lastChain.Name,
			}
			return impl.GetTypeOfFuncCall(funcCall, scope)
		}
	}

	return nil
}

// generateForInLoop handles for-in loop iteration using the Iterator trait
// for x in collection { ... } desugars to:
//
//	{
//	    CollType __iter = collection;
//	    while (CollType__Iterator__has_next(&__iter)) {
//	        ElemType x = CollType__Iterator__next(&__iter);
//	        ...
//	    }
//	}
func (impl *CBackendImplementation) generateForInLoop(scope *ast.Ast, l *tokens.Loop) {
	info := CGetScopeInformation(scope)
	forIn := l.ForIn

	// Get the loop variable name and optional type
	varName := forIn.Variable.Name
	varType := forIn.Variable.Type

	// Get the iterator expression
	iterExpr := impl.ExpressionToCString(forIn.SourceArray, scope)

	// Infer the iterator type using CProgramValues for type lookup
	resolveSymbol := func(name string) *tokens.TypeRef {
		varOpt := scope.ResolveSymbolAsVariable(name)
		if !varOpt.IsNil() {
			v := varOpt.Unwrap()
			// Look up the type in CProgramValues
			if valInfo, ok := (*CProgramValues)[v.GetFullName()]; ok && valInfo.GeckoType != nil {
				return valInfo.GeckoType
			}
			// Fallback: construct TypeRef from variable info
			return &tokens.TypeRef{Pointer: v.IsPointer}
		}
		return nil
	}
	iterType := tokens.InferType(forIn.SourceArray, resolveSymbol)

	// If basic inference failed, check for method call expressions (e.g., s.iter())
	if iterType == nil {
		iterType = impl.inferMethodCallType(forIn.SourceArray, scope)
	}

	if iterType == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Type Inference Error",
			"Cannot infer type of iterator expression in for-in loop",
			forIn.Pos,
		)
		return
	}

	// Get the iterator type name (strip pointer if present)
	iterTypeName := iterType.Type
	if iterType.Pointer {
		iterTypeName = strings.TrimSuffix(iterTypeName, "*")
	}

	// Look up the class to find Iterator trait implementation
	classOpt := scope.ResolveClass(iterTypeName)
	if classOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError(
			"Iterator Error",
			"Type '"+iterTypeName+"' is not a class that can be iterated",
			forIn.Pos,
		)
		return
	}
	class := classOpt.Unwrap()

	// Get the iterator hook to find trait and method names.
	iterHook := hooks.GetHookRegistry().GetHook(scope.GetRoot().Scope, hooks.HookIterator)
	requiredIteratorTrait := "Iterator"
	hasNextMethod := "has_next"
	nextMethod := "next"
	if iterHook != nil {
		if iterHook.TraitName != "" {
			requiredIteratorTrait = iterHook.TraitName
		}
		if len(iterHook.Methods) >= 2 {
			// Hook should define [next, has_next] methods.
			nextMethod = iterHook.Methods[0]
			hasNextMethod = iterHook.Methods[1]
		}
	}

	// Find a trait implementation that satisfies the iterator requirement.
	var iteratorTraitName string
	var elementType string
	for traitName := range class.Traits {
		if TraitMatchesOrExtends(traitName, requiredIteratorTrait) {
			iteratorTraitName = traitName
			// Extract element type from Iterator<T>
			if strings.Contains(traitName, "__") {
				// Generic instantiation like Iterator__int32
				parts := strings.SplitN(traitName, "__", 2)
				if len(parts) == 2 {
					elementType = parts[1]
				}
			}
			break
		}
	}

	if iteratorTraitName == "" {
		scope.ErrorScope.NewCompileTimeError(
			"Iterator Error",
			"Type '"+iterTypeName+"' does not implement trait '"+requiredIteratorTrait+"'",
			forIn.Pos,
		)
		return
	}

	// Determine element type for the loop variable
	var elemCType string
	if varType != nil {
		// Explicit type annotation on loop variable
		elemCType = TypeRefToCType(varType, scope)
	} else if elementType != "" {
		// Inferred from Iterator<T>
		if cType, ok := GeckoToCType[elementType]; ok {
			elemCType = cType
		} else {
			elemCType = elementType
		}
	} else {
		// Fallback - try to get from trait methods
		if traitMethods, ok := class.Traits[iteratorTraitName]; ok && traitMethods != nil {
			for _, m := range *traitMethods {
				if m.Name == nextMethod && m.Type != "" {
					if cType, ok := GeckoToCType[m.Type]; ok {
						elemCType = cType
					} else {
						elemCType = m.Type
					}
					break
				}
			}
		}
	}

	if elemCType == "" {
		elemCType = "int" // Fallback
	}

	// Get C type for iterator
	var iterCType string
	if cType, ok := GeckoToCType[iterTypeName]; ok {
		iterCType = cType
	} else {
		iterCType = iterTypeName
	}

	// Use bare class name for method calls (matches trait implementation naming)
	// Trait methods are mangled as: ClassName__TraitName__MethodName
	classBaseName := iterTypeName

	// Generate mangled method names
	hasNextMangled := classBaseName + "__" + iteratorTraitName + "__" + hasNextMethod
	nextMangled := classBaseName + "__" + iteratorTraitName + "__" + nextMethod

	// Generate the loop structure
	info.Code += "    {\n"

	// Copy iterator to local variable (for mutable state)
	info.Code += fmt.Sprintf("        %s __iter = %s;\n", iterCType, iterExpr)

	// Generate while loop with has_next condition
	info.Code += fmt.Sprintf("        while (%s(&__iter)) {\n", hasNextMangled)

	// Declare loop variable with value from next()
	info.Code += fmt.Sprintf("            %s %s = %s(&__iter);\n", elemCType, varName, nextMangled)

	// Create a child scope for the loop body to register the loop variable
	loopScope := newLexicalScope(scope, true)

	// Register the loop variable in the loop scope
	loopVar := ast.Variable{
		Name:   varName,
		Parent: loopScope,
	}
	loopScope.Variables[varName] = loopVar

	// Store element type info in CProgramValues
	(*CProgramValues)[loopVar.GetFullName()] = &CValueInformation{
		CType:     elemCType,
		GeckoType: &tokens.TypeRef{Type: elementType},
	}

	// Initialize scope info for the loop scope using CGetScopeInformation
	CGetScopeInformation(loopScope)

	// Process loop body entries in the loop scope
	for _, entry := range l.Value {
		impl.processEntry(loopScope, entry)
	}

	// Append the loop body code
	loopInfo := CGetScopeInformation(loopScope)
	impl.emitDefers(loopScope, loopInfo)
	impl.generateDropCalls(loopScope, loopInfo)
	info.Code += loopInfo.Code

	info.Code += "        }\n"
	info.Code += "    }\n"
}
