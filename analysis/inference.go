package analysis

import "github.com/neutrino2211/gecko/tokens"

func InferExpressionType(expr *tokens.Expression, ctx *AnalysisContext) string {
	if expr == nil {
		return "unknown"
	}
	if ctx != nil && ctx.SemanticGraph != nil {
		return FormatTypeRef(ctx.SemanticGraph.TypeOfExpression(expr))
	}
	return FormatTypeRef(tokens.InferType(expr, func(string) *tokens.TypeRef { return nil }))
}

func getPrimaryFromExpr(expr *tokens.Expression) *tokens.Primary {
	lo := expr.GetLogicalOr()
	if lo == nil {
		return nil
	}
	if lo.LogicalAnd == nil {
		return nil
	}
	if lo.LogicalAnd.Equality == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison.Addition == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison.Addition.Multiplication == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary == nil {
		return nil
	}
	return lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary
}
