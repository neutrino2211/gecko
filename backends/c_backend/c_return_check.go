// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// CheckReturnType validates that a return expression matches the function's declared return type
func (impl *CBackendImplementation) CheckReturnType(expr *tokens.Expression, scope *ast.Ast) {
	if expr == nil || scope.ErrorScope == nil {
		return
	}

	impl.CheckReturnAddressEscape(expr, scope)

	// Walk up scope chain to find the function's return type
	// Return statements can be nested inside if/while/for blocks
	var expectedType *tokens.TypeRef
	currentScope := scope
	for currentScope != nil {
		info, ok := (*CScopeDataMap)[currentScope.GetFullName()]
		if ok && info.CurrentFuncReturnType != nil {
			expectedType = info.CurrentFuncReturnType
			break
		}
		// Also check if this scope has CurrentFunc set (indicates we're in a method)
		if ok && info.CurrentFunc != "" {
			// Found function scope but no return type = void function
			break
		}
		currentScope = currentScope.Parent
	}

	// If no function context found, skip check (shouldn't happen in valid code)
	if currentScope == nil {
		return
	}

	// Get the actual return expression type
	actualType := impl.GetTypeOfExpression(expr, scope)
	if actualType == nil {
		return // Can't determine type, skip check
	}

	// If no return type declared (void function), we shouldn't be returning a value
	if expectedType == nil {
		if actualType.Type != "void" {
			scope.ErrorScope.NewCompileTimeError(
				"Return Type Mismatch",
				"Cannot return value from void function",
				expr.Pos,
			)
		}
		return
	}

	// Check compatibility
	if !TypesAreCompatible(expectedType, actualType, scope) {
		scope.ErrorScope.NewCompileTimeError(
			"Return Type Mismatch",
			"Cannot return '"+FormatTypeRef(actualType)+"' from function expecting '"+FormatTypeRef(expectedType)+"'",
			expr.Pos,
		)
	} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
		scope.ErrorScope.NewCompileTimeWarning(
			"Lossy Conversion",
			"Return statement: "+warning,
			expr.Pos,
		)
	}
}

// CheckReturnAddressEscape prevents returning addresses of local/argument variables.
// Returning &local or &arg would create a dangling pointer once the function returns.
func (impl *CBackendImplementation) CheckReturnAddressEscape(expr *tokens.Expression, scope *ast.Ast) {
	varName, found := findAddressOfLocalSymbolInExpression(expr)
	if !found || varName == "" {
		return
	}

	varOpt := scope.ResolveSymbolAsVariable(varName)
	if varOpt.IsNil() {
		return
	}

	v := varOpt.Unwrap()
	// Globals and external symbols are allowed to escape by address.
	if v.IsGlobal || v.IsExternal {
		return
	}

	scope.ErrorScope.NewCompileTimeError(
		"Lifetime Error",
		fmt.Sprintf("cannot return address of local variable '%s'\nhelp: return an owning type (e.g. Box<T>/Rc<T>) or pass storage from the caller", varName),
		expr.Pos,
	)
}

func findAddressOfLocalSymbolInExpression(expr *tokens.Expression) (string, bool) {
	if expr == nil || expr.GetLogicalOr() == nil {
		return "", false
	}
	return findAddressOfLocalSymbolInLogicalOr(expr.GetLogicalOr())
}

func findAddressOfLocalSymbolInLogicalOr(lo *tokens.LogicalOr) (string, bool) {
	if lo == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInLogicalAnd(lo.LogicalAnd); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInLogicalOr(lo.Next)
}

func findAddressOfLocalSymbolInLogicalAnd(la *tokens.LogicalAnd) (string, bool) {
	if la == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInEquality(la.Equality); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInLogicalAnd(la.Next)
}

func findAddressOfLocalSymbolInEquality(eq *tokens.Equality) (string, bool) {
	if eq == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInComparison(eq.Comparison); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInEquality(eq.Next)
}

func findAddressOfLocalSymbolInComparison(c *tokens.Comparison) (string, bool) {
	if c == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInAddition(c.Addition); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInComparison(c.Next)
}

func findAddressOfLocalSymbolInAddition(a *tokens.Addition) (string, bool) {
	if a == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInMultiplication(a.Multiplication); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInAddition(a.Next)
}

func findAddressOfLocalSymbolInMultiplication(m *tokens.Multiplication) (string, bool) {
	if m == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInUnary(m.Unary); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInMultiplication(m.Next)
}

func findAddressOfLocalSymbolInUnary(u *tokens.Unary) (string, bool) {
	if u == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInPrimary(u.Primary); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInUnary(u.Unary)
}

func findAddressOfLocalSymbolInPrimary(p *tokens.Primary) (string, bool) {
	if p == nil {
		return "", false
	}
	if name, ok := findAddressOfLocalSymbolInLiteral(p.Literal); ok {
		return name, true
	}
	return findAddressOfLocalSymbolInExpression(p.SubExpression)
}

func findAddressOfLocalSymbolInLiteral(l *tokens.Literal) (string, bool) {
	if l == nil {
		return "", false
	}
	// Detect direct &symbol (not &module.symbol or &obj.field/index).
	if l.IsPointer && l.Symbol != "" && l.SymbolModule == "" && len(l.Chain) == 0 && l.ArrayIndex == nil {
		return l.Symbol, true
	}

	// Recurse through intrinsic arguments if present.
	if l.Intrinsic != nil {
		for _, arg := range l.Intrinsic.Args {
			if name, ok := findAddressOfLocalSymbolInExpression(arg); ok {
				return name, true
			}
		}
	}
	return "", false
}

// CheckVoidReturn validates that void return is used in void function
func (impl *CBackendImplementation) CheckVoidReturn(scope *ast.Ast, pos lexer.Position) {
	if scope.ErrorScope == nil {
		return
	}

	// Get the current function's return type from scope info
	info := CGetScopeInformation(scope)
	expectedType := info.CurrentFuncReturnType

	// If function has a return type, void return is an error
	if expectedType != nil && expectedType.Type != "void" {
		scope.ErrorScope.NewCompileTimeError(
			"Missing Return Value",
			"Function expects return type '"+FormatTypeRef(expectedType)+"' but returns void",
			pos,
		)
	}
}
