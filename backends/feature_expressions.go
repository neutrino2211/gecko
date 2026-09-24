// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"github.com/neutrino2211/gecko/tokens"
)

func (d *featureDetector) analyzeExpression(expr *tokens.Expression) {
	if expr == nil || expr.GetLogicalOr() == nil {
		return
	}
	d.analyzeLogicalOr(expr.GetLogicalOr())
}

func (d *featureDetector) analyzeLogicalOr(lo *tokens.LogicalOr) {
	if lo == nil {
		return
	}
	if lo.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeLogicalAnd(lo.LogicalAnd)
	if lo.Next != nil {
		d.analyzeLogicalOr(lo.Next)
	}
}

func (d *featureDetector) analyzeLogicalAnd(la *tokens.LogicalAnd) {
	if la == nil {
		return
	}
	if la.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeEquality(la.Equality)
	if la.Next != nil {
		d.analyzeLogicalAnd(la.Next)
	}
}

func (d *featureDetector) analyzeEquality(eq *tokens.Equality) {
	if eq == nil {
		return
	}
	if eq.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeComparison(eq.Comparison)
	if eq.Next != nil {
		d.analyzeEquality(eq.Next)
	}
}

func (d *featureDetector) analyzeComparison(cmp *tokens.Comparison) {
	if cmp == nil {
		return
	}
	if cmp.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeAddition(cmp.Addition)
	if cmp.Next != nil {
		d.analyzeComparison(cmp.Next)
	}
}

func (d *featureDetector) analyzeAddition(add *tokens.Addition) {
	if add == nil {
		return
	}
	if add.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeMultiplication(add.Multiplication)
	if add.Next != nil {
		d.analyzeAddition(add.Next)
	}
}

func (d *featureDetector) analyzeMultiplication(mul *tokens.Multiplication) {
	if mul == nil {
		return
	}
	if mul.Op != "" {
		d.mark(FeatureBasicOps)
	}
	d.analyzeUnary(mul.Unary)
	if mul.Next != nil {
		d.analyzeMultiplication(mul.Next)
	}
}

func (d *featureDetector) analyzeUnary(un *tokens.Unary) {
	if un == nil {
		return
	}
	if un.Op != "" {
		d.mark(FeatureBasicOps)
	}
	if un.Cast != nil {
		d.mark(FeatureCasts)
		d.analyzeTypeRef(un.Cast.Type)
	}
	if un.Primary != nil {
		d.analyzePrimary(un.Primary)
	}
	if un.Unary != nil {
		d.analyzeUnary(un.Unary)
	}
}

func (d *featureDetector) analyzePrimary(prim *tokens.Primary) {
	if prim == nil {
		return
	}
	if prim.SubExpression != nil {
		d.analyzeExpression(prim.SubExpression)
	}
	if prim.Literal != nil {
		d.analyzeLiteral(prim.Literal)
	}
}

func (d *featureDetector) analyzeLiteral(lit *tokens.Literal) {
	if lit == nil {
		return
	}

	// Address-of operator
	if lit.IsPointer {
		d.mark(FeatureAddressOf)
		d.mark(FeaturePointers)
	}

	// String literals
	if lit.HasStringLiteral() {
		d.mark(FeatureStrings)
	}

	// Struct literals
	if lit.StructType != "" {
		d.mark(FeatureStructs)
	}

	// Array literals
	if len(lit.Array) > 0 {
		d.mark(FeatureArrays)
	}

	// Function calls
	if lit.FuncCall != nil {
		d.analyzeFuncCall(lit.FuncCall)
	}

	// Intrinsics
	if lit.Intrinsic != nil {
		d.analyzeIntrinsic(lit.Intrinsic)
	}

	// Method chains
	for _, chain := range lit.Chain {
		if chain.IsMethodCall() {
			d.mark(FeatureFunctions)
		}
	}

	// Array indexing
	if lit.ArrayIndex != nil {
		d.mark(FeatureArrays)
		d.analyzeExpression(lit.ArrayIndex)
	}
}

func (d *featureDetector) analyzeFuncCall(fc *tokens.FuncCall) {
	if fc == nil {
		return
	}
	d.mark(FeatureFunctions)

	// Generic type args
	if len(fc.TypeArgs) > 0 || len(fc.StaticTypeArgs) > 0 {
		d.mark(FeatureGenerics)
	}
}

func (d *featureDetector) analyzeIntrinsic(intr *tokens.Intrinsic) {
	if intr == nil {
		return
	}

	d.mark(FeatureIntrinsics)

	switch intr.Name {
	case "deref":
		d.mark(FeatureDeref)
		d.mark(FeaturePointers)
	case "write_volatile":
		d.mark(FeatureVolatile)
		d.mark(FeaturePointers)
	case "is_null":
		d.mark(FeaturePointers)
	case "size_of", "align_of":
		// These are core features
	}

	for _, arg := range intr.Args {
		d.analyzeExpression(arg)
	}
}
