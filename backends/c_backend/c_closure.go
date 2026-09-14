package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

var closureCounter int

// collectCapturedVariables finds all variable references in a lambda body
// that resolve to the enclosing scope.
func collectCapturedVariables(lambda *tokens.Lambda, scope *ast.Ast) []*ClosureCaptureField {
	if lambda == nil {
		return nil
	}

	captured := make(map[string]*ClosureCaptureField)
	paramNames := make(map[string]bool)
	for _, param := range lambda.Params {
		paramNames[param.Name] = true
	}

	tryCapture := func(symbolName string) {
		if paramNames[symbolName] || symbolName == "nil" || symbolName == "null" ||
			symbolName == "true" || symbolName == "false" || symbolName == "self" {
			return
		}
		varOpt := scope.ResolveSymbolAsVariable(symbolName)
		if !varOpt.IsNil() {
			variable := varOpt.Unwrap()
			fullName := variable.GetFullName()
			if _, exists := captured[fullName]; !exists {
				cType := "void*"
				if valInfo, ok := (*CProgramValues)[fullName]; ok && valInfo != nil && valInfo.CType != "" {
					cType = valInfo.CType
				}
				captured[fullName] = &ClosureCaptureField{
					VarName:   variable.Name,
					FullName:  fullName,
					CType:     cType,
					IsPointer: variable.IsPointer,
					Scope:     variable.Parent,
				}
			}
		}
	}

	extractSymbolsFromLambda(lambda, tryCapture)

	var result []*ClosureCaptureField
	for _, field := range captured {
		result = append(result, field)
	}
	return result
}

// extractSymbolsFromLambda extracts all symbol names from a lambda's body
func extractSymbolsFromLambda(lambda *tokens.Lambda, visit func(string)) {
	for _, entry := range lambda.Body {
		extractSymbolsFromEntry(entry, visit)
	}
}

func extractSymbolsFromEntry(entry *tokens.Entry, visit func(string)) {
	if entry == nil {
		return
	}
	if entry.Return != nil {
		extractSymbolsFromExpr(entry.Return, visit)
	}
	if entry.Assignment != nil && entry.Assignment.Value != nil {
		extractSymbolsFromExpr(entry.Assignment.Value, visit)
	}
	if entry.FuncCall != nil {
		extractSymbolsFromFuncCall(entry.FuncCall, visit)
	}
	if entry.If != nil {
		extractSymbolsFromIf(entry.If, visit)
	}
	if entry.Match != nil {
		extractSymbolsFromExpr(entry.Match.Scrutinee, visit)
		for _, mc := range entry.Match.Cases {
			extractSymbolsFromMatchPattern(mc.Pattern, visit)
			if mc.Guard != nil {
				extractSymbolsFromExpr(mc.Guard, visit)
			}
			if mc.Body != nil {
				for _, e := range mc.Body.Entries {
					extractSymbolsFromEntry(e, visit)
				}
				if mc.Body.Expr != nil {
					extractSymbolsFromExpr(mc.Body.Expr, visit)
				}
			}
		}
	}
	if entry.Defer != nil && entry.Defer.Expression != nil {
		extractSymbolsFromExpr(entry.Defer.Expression, visit)
	}
	if entry.Method != nil {
		for _, e := range entry.Method.Value {
			extractSymbolsFromEntry(e, visit)
		}
	}
	if entry.Loop != nil {
		extractSymbolsFromLoop(entry.Loop, visit)
	}
	if entry.MethodCall != nil {
		extractSymbolsFromMethodCall(entry.MethodCall, visit)
	}
	if entry.Intrinsic != nil {
		for _, arg := range entry.Intrinsic.Args {
			extractSymbolsFromExpr(arg, visit)
		}
	}
}

func extractSymbolsFromFuncCall(fc *tokens.FuncCall, visit func(string)) {
	if fc == nil {
		return
	}
	if fc.Module != "" {
		visit(fc.Module)
	}
	if fc.Function != "" {
		visit(fc.Function)
	}
	for _, arg := range fc.Arguments {
		if arg.Value != nil {
			extractSymbolsFromExpr(arg.Value, visit)
		}
		if arg.SubCall != nil {
			extractSymbolsFromFuncCall(arg.SubCall, visit)
		}
	}
}

func extractSymbolsFromMethodCall(mc *tokens.MethodCall, visit func(string)) {
	if mc == nil {
		return
	}
	visit(mc.Base)
	for _, chain := range mc.Chain {
		if chain.HasParens {
			for _, arg := range chain.Args {
				if arg.Value != nil {
					extractSymbolsFromExpr(arg.Value, visit)
				}
			}
		}
	}
}

func extractSymbolsFromLoop(loop *tokens.Loop, visit func(string)) {
	if loop == nil {
		return
	}
	if loop.ForExpression != nil {
		extractSymbolsFromExpr(loop.ForExpression, visit)
	}
	if loop.ForIn != nil {
		if loop.ForIn.SourceArray != nil {
			extractSymbolsFromExpr(loop.ForIn.SourceArray, visit)
		}
	}
	if loop.ForOf != nil {
		if loop.ForOf.SourceArray != nil {
			extractSymbolsFromExpr(loop.ForOf.SourceArray, visit)
		}
	}
	if loop.WhileExpr != nil {
		extractSymbolsFromExpr(loop.WhileExpr, visit)
	}
	for _, e := range loop.Value {
		extractSymbolsFromEntry(e, visit)
	}
}

func extractSymbolsFromIf(ifStmt *tokens.If, visit func(string)) {
	if ifStmt == nil {
		return
	}
	extractSymbolsFromExpr(ifStmt.Expression, visit)
	for _, e := range ifStmt.Value {
		extractSymbolsFromEntry(e, visit)
	}
	if ifStmt.ElseIf != nil {
		extractSymbolsFromElseIf(ifStmt.ElseIf, visit)
	}
	if ifStmt.Else != nil {
		for _, e := range ifStmt.Else.Value {
			extractSymbolsFromEntry(e, visit)
		}
	}
}

func extractSymbolsFromElseIf(elseIf *tokens.ElseIf, visit func(string)) {
	if elseIf == nil {
		return
	}
	extractSymbolsFromExpr(elseIf.Expression, visit)
	for _, e := range elseIf.Value {
		extractSymbolsFromEntry(e, visit)
	}
	if elseIf.ElseIf != nil {
		extractSymbolsFromElseIf(elseIf.ElseIf, visit)
	}
	if elseIf.Else != nil {
		for _, e := range elseIf.Else.Value {
			extractSymbolsFromEntry(e, visit)
		}
	}
}

func extractSymbolsFromMatchPattern(pattern *tokens.MatchPattern, visit func(string)) {
	if pattern == nil {
		return
	}
	for _, alt := range pattern.Alts {
		if alt == nil || alt.Inner == nil {
			continue
		}
		inner := alt.Inner
		if inner.Literal != nil {
			extractSymbolsFromPrimaryForPattern(inner.Literal, visit)
		}
		if inner.Range != nil {
			extractSymbolsFromPrimaryForPattern(inner.Range.Start, visit)
			extractSymbolsFromPrimaryForPattern(inner.Range.End, visit)
		}
		if inner.Destructure != nil {
			for _, f := range inner.Destructure.Fields {
				if f.Pattern != nil {
					extractSymbolsFromMatchPattern(f.Pattern, visit)
				}
			}
		}
	}
}

func extractSymbolsFromPrimaryForPattern(p *tokens.Primary, visit func(string)) {
	if p == nil {
		return
	}
	if p.Literal != nil && p.Literal.Symbol != "" {
		visit(p.Literal.Symbol)
	}
	if p.SubExpression != nil {
		extractSymbolsFromExpr(p.SubExpression, visit)
	}
}

func extractSymbolsFromExpr(expr *tokens.Expression, visit func(string)) {
	if expr == nil || expr.Cond == nil || expr.Cond.LogicalOr == nil {
		return
	}
	extractSymbolsFromOrExpr(expr.Cond.LogicalOr, visit)
}

func extractSymbolsFromOrExpr(oe *tokens.OrExpression, visit func(string)) {
	if oe == nil {
		return
	}
	extractSymbolsFromLogicalOr(oe.LogicalOr, visit)
	if oe.Or != nil {
		extractSymbolsFromOrExpr(oe.Or, visit)
	}
}

func extractSymbolsFromLogicalOr(lo *tokens.LogicalOr, visit func(string)) {
	if lo == nil {
		return
	}
	extractSymbolsFromLogicalAnd(lo.LogicalAnd, visit)
	if lo.Next != nil {
		extractSymbolsFromLogicalOr(lo.Next, visit)
	}
}

func extractSymbolsFromLogicalAnd(la *tokens.LogicalAnd, visit func(string)) {
	if la == nil {
		return
	}
	extractSymbolsFromEquality(la.Equality, visit)
	if la.Next != nil {
		extractSymbolsFromLogicalAnd(la.Next, visit)
	}
}

func extractSymbolsFromEquality(eq *tokens.Equality, visit func(string)) {
	if eq == nil {
		return
	}
	extractSymbolsFromComparison(eq.Comparison, visit)
	if eq.Next != nil {
		extractSymbolsFromEquality(eq.Next, visit)
	}
}

func extractSymbolsFromComparison(comp *tokens.Comparison, visit func(string)) {
	if comp == nil {
		return
	}
	extractSymbolsFromAddition(comp.Addition, visit)
	if comp.Next != nil {
		extractSymbolsFromComparison(comp.Next, visit)
	}
}

func extractSymbolsFromAddition(add *tokens.Addition, visit func(string)) {
	if add == nil {
		return
	}
	extractSymbolsFromMul(add.Multiplication, visit)
	if add.Next != nil {
		extractSymbolsFromAddition(add.Next, visit)
	}
}

func extractSymbolsFromMul(mul *tokens.Multiplication, visit func(string)) {
	if mul == nil {
		return
	}
	extractSymbolsFromUnary(mul.Unary, visit)
	if mul.Next != nil {
		extractSymbolsFromMul(mul.Next, visit)
	}
}

func extractSymbolsFromUnary(u *tokens.Unary, visit func(string)) {
	if u == nil {
		return
	}
	if u.Cast != nil {
		// Cast applies to the inner expression (Unary or Primary)
		if u.Unary != nil {
			extractSymbolsFromUnary(u.Unary, visit)
		} else if u.Primary != nil {
			extractSymbolsFromPrimary(u.Primary, visit)
		}
	} else if u.Unary != nil {
		extractSymbolsFromUnary(u.Unary, visit)
	} else if u.Primary != nil {
		extractSymbolsFromPrimary(u.Primary, visit)
	}
}

func extractSymbolsFromPrimary(p *tokens.Primary, visit func(string)) {
	if p == nil {
		return
	}
	if p.SubExpression != nil {
		extractSymbolsFromExpr(p.SubExpression, visit)
	}
	if p.Literal != nil {
		extractSymbolsFromLiteral(p.Literal, visit)
	}
}

func extractSymbolsFromLiteral(lit *tokens.Literal, visit func(string)) {
	if lit == nil {
		return
	}
	if lit.Symbol != "" {
		visit(lit.Symbol)
	}
	if lit.SymbolModule != "" {
		visit(lit.SymbolModule)
	}
	for _, chain := range lit.Chain {
		if chain.HasParens {
			for _, arg := range chain.Args {
				if arg.Value != nil {
					extractSymbolsFromExpr(arg.Value, visit)
				}
			}
		}
	}
	if lit.ArrayIndex != nil {
		extractSymbolsFromExpr(lit.ArrayIndex, visit)
	}
	for _, kv := range lit.StructFields {
		if kv.Value != nil {
			extractSymbolsFromExpr(kv.Value, visit)
		}
	}
	for _, arrLit := range lit.Array {
		extractSymbolsFromLiteral(arrLit, visit)
	}
}

// generateCaptureStruct creates a capture struct for a lambda that captures variables
func generateCaptureStruct(fields []*ClosureCaptureField, scope *ast.Ast) (structName, structDef, paramName string) {
	closureCounter++
	structName = fmt.Sprintf("__cap_%d", closureCounter)
	paramName = "__cap"

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("struct %s {\n", structName))
	for _, field := range fields {
		sb.WriteString(fmt.Sprintf("    %s %s;\n", field.CType, field.VarName))
	}
	sb.WriteString("};\n")

	structDef = sb.String()
	return
}

// generateCaptureInit generates C code to initialize a capture struct
func generateCaptureInit(fields []*ClosureCaptureField, structVarName string) string {
	var sb strings.Builder
	for _, field := range fields {
		sb.WriteString(fmt.Sprintf("    %s.%s = %s;\n", structVarName, field.VarName, field.VarName))
	}
	return sb.String()
}

// generateCaptureAccess generates C code to access a captured variable through the capture struct
func generateCaptureAccess(field *ClosureCaptureField, capGlobalName string) string {
	return fmt.Sprintf("%s.%s", capGlobalName, field.VarName)
}

// resolveVariableWithIdentifier resolves a variable and returns the appropriate C identifier,
// checking for closure captures first.
func resolveVariableWithIdentifier(variable *ast.Variable, scope *ast.Ast, info *CScopeInformation) string {
	if info.ClosureCaptures != nil && variable != nil {
		fullName := variable.GetFullName()
		if field, ok := info.ClosureCaptures.FieldMap[fullName]; ok {
			return generateCaptureAccess(field, info.ClosureCaptures.GlobalSlot)
		}
	}
	return CVariableIdentifier(variable)
}
