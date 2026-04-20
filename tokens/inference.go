package tokens

import (
	"strings"
)

// InferType attempts to infer the type of an expression.
// Returns nil if type cannot be inferred.
func InferType(expr *Expression, resolveSymbol func(string) *TypeRef) *TypeRef {
	if expr == nil || expr.LogicalOr == nil {
		return nil
	}
	return inferFromLogicalOr(expr.LogicalOr, resolveSymbol)
}

func inferFromLogicalOr(lo *LogicalOr, resolveSymbol func(string) *TypeRef) *TypeRef {
	if lo == nil {
		return nil
	}

	// If there's an || operator, result is bool
	if lo.Op != "" {
		return &TypeRef{Type: "bool"}
	}

	return inferFromLogicalAnd(lo.LogicalAnd, resolveSymbol)
}

func inferFromLogicalAnd(la *LogicalAnd, resolveSymbol func(string) *TypeRef) *TypeRef {
	if la == nil {
		return nil
	}

	// If there's an && operator, result is bool
	if la.Op != "" {
		return &TypeRef{Type: "bool"}
	}

	return inferFromEquality(la.Equality, resolveSymbol)
}

func inferFromEquality(eq *Equality, resolveSymbol func(string) *TypeRef) *TypeRef {
	if eq == nil {
		return nil
	}

	// If there's an equality operator, result is bool
	if eq.Op != "" {
		return &TypeRef{Type: "bool"}
	}

	return inferFromComparison(eq.Comparison, resolveSymbol)
}

func inferFromComparison(cmp *Comparison, resolveSymbol func(string) *TypeRef) *TypeRef {
	if cmp == nil {
		return nil
	}

	// If there's a comparison operator, result is bool
	if cmp.Op != "" {
		return &TypeRef{Type: "bool"}
	}

	return inferFromAddition(cmp.Addition, resolveSymbol)
}

func inferFromAddition(add *Addition, resolveSymbol func(string) *TypeRef) *TypeRef {
	if add == nil {
		return nil
	}

	leftType := inferFromMultiplication(add.Multiplication, resolveSymbol)

	// If there's an operator, we need to consider type promotion
	if add.Op != "" && add.Next != nil {
		rightType := inferFromAddition(add.Next, resolveSymbol)
		return promoteTypes(leftType, rightType)
	}

	return leftType
}

func inferFromMultiplication(mul *Multiplication, resolveSymbol func(string) *TypeRef) *TypeRef {
	if mul == nil {
		return nil
	}

	leftType := inferFromUnary(mul.Unary, resolveSymbol)

	if mul.Op != "" && mul.Next != nil {
		rightType := inferFromMultiplication(mul.Next, resolveSymbol)
		return promoteTypes(leftType, rightType)
	}

	return leftType
}

func inferFromUnary(un *Unary, resolveSymbol func(string) *TypeRef) *TypeRef {
	if un == nil {
		return nil
	}

	// Unary operator with nested unary
	if un.Op != "" && un.Unary != nil {
		innerType := inferFromUnary(un.Unary, resolveSymbol)
		// Logical NOT returns bool
		if un.Op == "!" {
			return &TypeRef{Type: "bool"}
		}
		// Negation and plus preserve type
		return innerType
	}

	// Primary expression
	primaryType := inferFromPrimary(un.Primary, resolveSymbol)

	// Check for cast
	if un.Cast != nil && un.Cast.Type != nil {
		return un.Cast.Type
	}

	return primaryType
}

func inferFromPrimary(prim *Primary, resolveSymbol func(string) *TypeRef) *TypeRef {
	if prim == nil {
		return nil
	}

	// Parenthesized sub-expression
	if prim.SubExpression != nil {
		return InferType(prim.SubExpression, resolveSymbol)
	}

	return inferFromLiteral(prim.Literal, resolveSymbol)
}

func inferFromLiteral(lit *Literal, resolveSymbol func(string) *TypeRef) *TypeRef {
	if lit == nil {
		return nil
	}

	// Number literal
	if lit.Number != "" {
		if strings.Contains(lit.Number, ".") {
			return &TypeRef{Type: "float64"}
		}
		return &TypeRef{Type: "int32"}
	}

	// Boolean literal
	if lit.Bool != "" {
		return &TypeRef{Type: "bool"}
	}

	// String literal
	if lit.String != "" {
		return &TypeRef{Type: "string"}
	}

	// Function call - need to look up return type
	if lit.FuncCall != nil {
		// Backend will handle this via method lookup
		return nil
	}

	// Symbol reference - look it up (handle address-of operator too)
	// If there's a chain (like rect.width), let the backend handle it
	// since it has full type information for field access
	if lit.Symbol != "" && len(lit.Chain) == 0 {
		if resolveSymbol != nil {
			baseType := resolveSymbol(lit.Symbol)
			if baseType != nil {
				// Address-of operator: &symbol produces a pointer
				if lit.IsPointer {
					return &TypeRef{
						Type:     baseType.Type,
						TypeArgs: baseType.TypeArgs,
						Pointer:  true,
					}
				}
				return baseType
			}
		}
	}

	// Struct literal
	if lit.StructType != "" {
		return &TypeRef{Type: lit.StructType}
	}

	// Array literal - infer from first element
	if len(lit.Array) > 0 {
		elemType := inferFromLiteral(lit.Array[0], resolveSymbol)
		if elemType != nil {
			return &TypeRef{Array: elemType}
		}
	}

	// Intrinsic calls - some have known return types
	if lit.Intrinsic != nil {
		return inferFromIntrinsic(lit.Intrinsic)
	}

	return nil
}

func inferFromIntrinsic(intr *Intrinsic) *TypeRef {
	if intr == nil {
		return nil
	}

	switch intr.Name {
	case "size_of":
		return &TypeRef{Type: "uint64"}
	}

	return nil
}

func promoteTypes(left, right *TypeRef) *TypeRef {
	if left == nil {
		return right
	}
	if right == nil {
		return left
	}

	leftRank := typeRank(left.Type)
	rightRank := typeRank(right.Type)

	if leftRank >= rightRank {
		return left
	}
	return right
}

func typeRank(typeName string) int {
	ranks := map[string]int{
		"int8":    1,
		"uint8":   2,
		"int16":   3,
		"uint16":  4,
		"int32":   5,
		"uint32":  6,
		"int64":   7,
		"uint64":  8,
		"float32": 9,
		"float64": 10,
	}
	if rank, ok := ranks[typeName]; ok {
		return rank
	}
	return 0
}
