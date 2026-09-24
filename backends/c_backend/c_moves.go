// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

func (impl *CBackendImplementation) isImplicitCopyTypeForMoves(t *tokens.TypeRef, scope *ast.Ast) bool {
	if t == nil {
		return true
	}
	// Raw pointers and nullable/non-null pointers are copy-like handles.
	if t.Pointer {
		return true
	}

	normalized := normalizeTypeName(t.Type)
	switch normalized {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool", "string", "void":
		return true
	}

	if isTypeParameter(t.Type) || t.Type == "generic" {
		return dropMethodForType(t, scope) == ""
	}

	return impl.TypeImplementsTrait(t, "Copy", scope)
}

func extractPlainSymbolFromExpression(expr *tokens.Expression) (string, bool) {
	if expr == nil || expr.GetLogicalOr() == nil {
		return "", false
	}
	lo := expr.GetLogicalOr()
	if lo.Next != nil || lo.LogicalAnd == nil {
		return "", false
	}
	la := lo.LogicalAnd
	if la.Next != nil || la.Equality == nil {
		return "", false
	}
	eq := la.Equality
	if eq.Next != nil || eq.Op != "" || eq.Comparison == nil {
		return "", false
	}
	c := eq.Comparison
	if c.Next != nil || c.Op != "" || c.Addition == nil {
		return "", false
	}
	a := c.Addition
	if a.Next != nil || a.Op != "" || a.Multiplication == nil {
		return "", false
	}
	m := a.Multiplication
	if m.Next != nil || m.Op != "" || m.Unary == nil {
		return "", false
	}
	u := m.Unary
	if u.Op != "" || u.Unary != nil || u.Cast != nil || u.Primary == nil {
		return "", false
	}
	p := u.Primary
	if p.SubExpression != nil {
		return extractPlainSymbolFromExpression(p.SubExpression)
	}
	if p.Literal == nil {
		return "", false
	}
	l := p.Literal
	if l.Symbol == "" || l.SymbolModule != "" || l.IsPointer || l.ArrayIndex != nil || len(l.Chain) > 0 || l.FuncCall != nil || l.Intrinsic != nil {
		return "", false
	}
	return l.Symbol, true
}

func (impl *CBackendImplementation) ApplyMoveFromExpression(expr *tokens.Expression, scope *ast.Ast) {
	if CurrentTypeState == nil || expr == nil || scope == nil {
		return
	}

	symbol, ok := extractPlainSymbolFromExpression(expr)
	if !ok {
		return
	}

	varOpt := scope.ResolveSymbolAsVariable(symbol)
	if varOpt.IsNil() {
		return
	}
	v := varOpt.Unwrap()
	if v.IsGlobal || v.IsExternal {
		return
	}

	valueInfo, hasInfo := (*CProgramValues)[v.GetFullName()]
	if !hasInfo || valueInfo == nil || valueInfo.GeckoType == nil {
		return
	}
	if impl.isImplicitCopyTypeForMoves(valueInfo.GeckoType, scope) {
		return
	}

	CurrentTypeState.SetMoved(v.GetFullName())
	if v.DropFlag != "" {
		CGetScopeInformation(scope).Code += v.DropFlag + " = 0;\n"
	}
}

func (impl *CBackendImplementation) moveCallArgument(expr *tokens.Expression, value string, scope *ast.Ast) string {
	symbol, ok := extractPlainSymbolFromExpression(expr)
	if !ok {
		return value
	}
	variable := scope.ResolveSymbolAsVariable(symbol)
	if variable.IsNil() {
		return value
	}
	v := variable.Unwrap()
	if v.IsGlobal || v.IsExternal || v.DropFlag == "" {
		return value
	}
	valueInfo := (*CProgramValues)[v.GetFullName()]
	if valueInfo == nil || impl.isImplicitCopyTypeForMoves(valueInfo.GeckoType, scope) {
		return value
	}
	if CurrentTypeState != nil {
		CurrentTypeState.SetMoved(v.GetFullName())
	}
	return "({ __auto_type __moved_arg = (" + value + "); " + v.DropFlag + " = 0; __moved_arg; })"
}

func (impl *CBackendImplementation) moveUnaryValue(unary *tokens.Unary, value string, scope *ast.Ast) string {
	if unary == nil {
		return value
	}
	expr := &tokens.Expression{Cond: &tokens.ConditionalExpr{LogicalOr: &tokens.OrExpression{LogicalOr: &tokens.LogicalOr{LogicalAnd: &tokens.LogicalAnd{Equality: &tokens.Equality{Comparison: &tokens.Comparison{Addition: &tokens.Addition{Multiplication: &tokens.Multiplication{Unary: unary}}}}}}}}}
	return impl.moveCallArgument(expr, value, scope)
}

func (impl *CBackendImplementation) moveLogicalOrValue(logicalOr *tokens.LogicalOr, value string, scope *ast.Ast) string {
	if logicalOr == nil {
		return value
	}
	expr := &tokens.Expression{Cond: &tokens.ConditionalExpr{LogicalOr: &tokens.OrExpression{LogicalOr: logicalOr}}}
	return impl.moveCallArgument(expr, value, scope)
}

func (impl *CBackendImplementation) intrinsicMove(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 1 || len(i.TypeArgs) != 0 {
		scope.ErrorScope.NewCompileTimeError("Move Error", "@move requires one local binding", i.Pos)
		return "0"
	}
	symbol, ok := extractPlainSymbolFromExpression(i.Args[0])
	if !ok {
		scope.ErrorScope.NewCompileTimeError("Move Error", "@move requires a named local binding", i.Pos)
		return "0"
	}
	variable := scope.ResolveSymbolAsVariable(symbol)
	if variable.IsNil() || variable.Unwrap().IsGlobal || variable.Unwrap().IsExternal {
		scope.ErrorScope.NewCompileTimeError("Move Error", "@move requires a local binding", i.Pos)
		return "0"
	}
	v := variable.Unwrap()
	value := impl.ExpressionToCString(i.Args[0], scope)
	if CurrentTypeState != nil {
		CurrentTypeState.SetMoved(v.GetFullName())
	}
	if v.DropFlag == "" {
		return value
	}
	return "({ __auto_type __moved_value = (" + value + "); " + v.DropFlag + " = 0; __moved_value; })"
}
