// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) extractConditionFacts(expr *tokens.Expression, env *flowEnv) conditionFacts {
	expr = unwrapParenthesizedExpression(expr)
	if expr == nil || expr.GetLogicalOr() == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	return a.factsFromLogicalOr(expr.GetLogicalOr(), env)
}

func (a *analyzer) factsFromLogicalOr(lo *tokens.LogicalOr, env *flowEnv) conditionFacts {
	if lo == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	left := a.factsFromLogicalAnd(lo.LogicalAnd, env)
	if lo.Next == nil {
		return left
	}
	right := a.factsFromLogicalOr(lo.Next, env)
	return conditionFacts{
		trueNonNull:  intersectFactMaps(left.trueNonNull, right.trueNonNull),
		falseNonNull: unionFactMaps(left.falseNonNull, right.falseNonNull),
	}
}

func (a *analyzer) factsFromLogicalAnd(la *tokens.LogicalAnd, env *flowEnv) conditionFacts {
	if la == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	left := a.factsFromEquality(la.Equality, env)
	if la.Next == nil {
		return left
	}
	right := a.factsFromLogicalAnd(la.Next, env)
	return conditionFacts{
		trueNonNull:  unionFactMaps(left.trueNonNull, right.trueNonNull),
		falseNonNull: intersectFactMaps(left.falseNonNull, right.falseNonNull),
	}
}

func (a *analyzer) factsFromEquality(eq *tokens.Equality, env *flowEnv) conditionFacts {
	if eq == nil {
		return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	}
	if eq.Next != nil && (eq.Op == "!=" || eq.Op == "==") {
		leftSymbol := extractSymbolFromComparison(eq.Comparison)
		rightSymbol := extractSymbolFromEquality(eq.Next)
		leftNil := isNilFromComparison(eq.Comparison)
		rightNil := isNilFromEquality(eq.Next)

		if leftSymbol != "" && rightNil {
			return a.factsFromNullComparison(leftSymbol, eq.Op == "!=", env)
		}
		if rightSymbol != "" && leftNil {
			return a.factsFromNullComparison(rightSymbol, eq.Op == "!=", env)
		}
	}

	if info := a.factsFromIntrinsicEquality(eq, env); info.trueNonNull != nil {
		return info
	}

	// Nested comparisons don't carry non-null facts by default.
	return conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
}

func (a *analyzer) factsFromNullComparison(symbol string, isNotNull bool, env *flowEnv) conditionFacts {
	out := conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	if symbol == "" {
		return out
	}
	bound := env.lookup(symbol)
	if bound == nil {
		if sid, ok := a.program.globalSymbolIDs[symbol]; ok {
			bound = &boundVar{Name: symbol, SymbolID: sid}
		}
	}
	if bound == nil || bound.SymbolID == 0 {
		return out
	}
	if isNotNull {
		out.trueNonNull[bound.SymbolID] = true
	} else {
		out.falseNonNull[bound.SymbolID] = true
	}
	return out
}

func (a *analyzer) factsFromIntrinsicEquality(eq *tokens.Equality, env *flowEnv) conditionFacts {
	out := conditionFacts{trueNonNull: map[int64]bool{}, falseNonNull: map[int64]bool{}}
	primary := primaryFromEquality(eq)
	if primary == nil || primary.Literal == nil || primary.Literal.Intrinsic == nil {
		return conditionFacts{}
	}
	intr := primary.Literal.Intrinsic
	if intr.Name != "is_not_null" && intr.Name != "is_null" {
		return conditionFacts{}
	}
	if len(intr.Args) != 1 {
		return conditionFacts{}
	}
	symbol := extractSymbolFromExpression(intr.Args[0])
	if symbol == "" {
		return conditionFacts{}
	}
	bound := env.lookup(symbol)
	if bound == nil {
		if sid, ok := a.program.globalSymbolIDs[symbol]; ok {
			bound = &boundVar{Name: symbol, SymbolID: sid}
		}
	}
	if bound == nil || bound.SymbolID == 0 {
		return conditionFacts{}
	}
	if intr.Name == "is_not_null" {
		out.trueNonNull[bound.SymbolID] = true
	} else {
		out.falseNonNull[bound.SymbolID] = true
	}
	return out
}

func unionFactMaps(a, b map[int64]bool) map[int64]bool {
	out := make(map[int64]bool)
	for id, ok := range a {
		if ok {
			out[id] = true
		}
	}
	for id, ok := range b {
		if ok {
			out[id] = true
		}
	}
	return out
}

func intersectFactMaps(a, b map[int64]bool) map[int64]bool {
	if len(a) == 0 || len(b) == 0 {
		return map[int64]bool{}
	}
	out := make(map[int64]bool)
	for id, ok := range a {
		if ok && b[id] {
			out[id] = true
		}
	}
	return out
}

func primaryFromEquality(eq *tokens.Equality) *tokens.Primary {
	if eq == nil || eq.Next != nil || eq.Comparison == nil {
		return nil
	}
	cmp := eq.Comparison
	if cmp.Next != nil || cmp.Addition == nil {
		return nil
	}
	add := cmp.Addition
	if add.Next != nil || add.Multiplication == nil {
		return nil
	}
	mul := add.Multiplication
	if mul.Next != nil || mul.Unary == nil {
		return nil
	}
	if mul.Unary.Primary == nil {
		return nil
	}
	return mul.Unary.Primary
}

func extractSymbolFromExpression(expr *tokens.Expression) string {
	expr = unwrapParenthesizedExpression(expr)
	if expr == nil || expr.GetLogicalOr() == nil {
		return ""
	}
	lo := expr.GetLogicalOr()
	if lo.Next != nil || lo.LogicalAnd == nil || lo.LogicalAnd.Next != nil || lo.LogicalAnd.Equality == nil {
		return ""
	}
	return extractSymbolFromEquality(lo.LogicalAnd.Equality)
}

func extractSymbolFromEquality(eq *tokens.Equality) string {
	if eq == nil || eq.Next != nil || eq.Comparison == nil {
		return ""
	}
	return extractSymbolFromComparison(eq.Comparison)
}

func extractSymbolFromComparison(cmp *tokens.Comparison) string {
	if cmp == nil || cmp.Next != nil || cmp.Addition == nil {
		return ""
	}
	add := cmp.Addition
	if add.Next != nil || add.Multiplication == nil {
		return ""
	}
	mul := add.Multiplication
	if mul.Next != nil || mul.Unary == nil || mul.Unary.Primary == nil || mul.Unary.Primary.Literal == nil {
		return ""
	}
	lit := mul.Unary.Primary.Literal
	if lit.Symbol != "" && lit.SymbolModule == "" && len(lit.Chain) == 0 && lit.ArrayIndex == nil {
		return lit.Symbol
	}
	return ""
}

func isNilFromEquality(eq *tokens.Equality) bool {
	if eq == nil || eq.Next != nil || eq.Comparison == nil {
		return false
	}
	return isNilFromComparison(eq.Comparison)
}

func isNilFromComparison(cmp *tokens.Comparison) bool {
	if cmp == nil || cmp.Next != nil || cmp.Addition == nil {
		return false
	}
	add := cmp.Addition
	if add.Next != nil || add.Multiplication == nil {
		return false
	}
	mul := add.Multiplication
	if mul.Next != nil || mul.Unary == nil || mul.Unary.Primary == nil || mul.Unary.Primary.Literal == nil {
		return false
	}
	lit := mul.Unary.Primary.Literal
	if (lit.Symbol == "nil" || lit.Symbol == "null") && lit.SymbolModule == "" && len(lit.Chain) == 0 && lit.ArrayIndex == nil {
		return true
	}
	// Allow C-style null pointer comparisons (`ptr != 0 as T*`) for narrowing.
	if lit.Number == "0" && lit.Symbol == "" && lit.SymbolModule == "" && len(lit.Chain) == 0 && lit.ArrayIndex == nil {
		if mul.Unary.Cast != nil && mul.Unary.Cast.Type != nil && mul.Unary.Cast.Type.Pointer {
			return true
		}
	}
	return false
}

func unwrapParenthesizedExpression(expr *tokens.Expression) *tokens.Expression {
	current := expr
	for {
		if current == nil || current.Cond == nil || current.Cond.LogicalOr == nil || current.Cond.LogicalOr.Or != nil {
			return current
		}
		lo := current.Cond.LogicalOr.LogicalOr
		if lo == nil || lo.Next != nil || lo.LogicalAnd == nil || lo.LogicalAnd.Next != nil {
			return current
		}
		eq := lo.LogicalAnd.Equality
		if eq == nil || eq.Op != "" || eq.Next != nil || eq.Comparison == nil || eq.Comparison.Op != "" || eq.Comparison.Next != nil {
			return current
		}
		add := eq.Comparison.Addition
		if add == nil || add.Op != "" || add.Next != nil || add.Multiplication == nil {
			return current
		}
		mul := add.Multiplication
		if mul.Op != "" || mul.Next != nil || mul.Unary == nil {
			return current
		}
		un := mul.Unary
		if un.Op != "" || un.Unary != nil || un.Cast != nil || un.Primary == nil || un.Primary.SubExpression == nil || un.Primary.Literal != nil {
			return current
		}
		current = un.Primary.SubExpression
	}
}
