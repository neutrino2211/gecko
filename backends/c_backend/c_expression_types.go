// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

// GetTypeOfUnary gets the type of a unary expression
func (impl *CBackendImplementation) GetTypeOfUnary(u *tokens.Unary, scope *ast.Ast) *tokens.TypeRef {
	if u == nil {
		return nil
	}

	// If there's a cast, the result type is the cast target type
	if u.Cast != nil && u.Cast.Type != nil {
		return u.Cast.Type
	}

	if u.Unary != nil {
		operandType := impl.GetTypeOfUnary(u.Unary, scope)
		if u.Op == "try" {
			if tryType := impl.resolveHookValueType(operandType, scope, hooks.HookTry, "try_unwrap"); tryType != nil {
				return tryType
			}
		}
		return operandType
	}

	if u.Primary != nil {
		return impl.GetTypeOfPrimary(u.Primary, scope)
	}

	return nil
}

// GetTypeOfPrimary gets the type of a primary expression
func (impl *CBackendImplementation) GetTypeOfPrimary(p *tokens.Primary, scope *ast.Ast) *tokens.TypeRef {
	if p == nil {
		return nil
	}

	if p.Literal != nil {
		return impl.GetTypeOfLiteral(p.Literal, scope)
	}

	if p.SubExpression != nil {
		return impl.GetTypeOfExpression(p.SubExpression, scope)
	}

	return nil
}

// GetTypeOfMultiplication gets the type of a multiplication expression
func (impl *CBackendImplementation) GetTypeOfMultiplication(m *tokens.Multiplication, scope *ast.Ast) *tokens.TypeRef {
	if m == nil {
		return nil
	}
	return impl.GetTypeOfUnary(m.Unary, scope)
}

// GetTypeOfAddition gets the type of an addition expression
func (impl *CBackendImplementation) GetTypeOfAddition(a *tokens.Addition, scope *ast.Ast) *tokens.TypeRef {
	if a == nil {
		return nil
	}
	return impl.GetTypeOfMultiplication(a.Multiplication, scope)
}

// GetTypeOfComparison gets the type of a comparison expression
func (impl *CBackendImplementation) GetTypeOfComparison(c *tokens.Comparison, scope *ast.Ast) *tokens.TypeRef {
	if c == nil {
		return nil
	}
	// Comparisons return bool
	if c.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfAddition(c.Addition, scope)
}

// GetTypeOfEquality gets the type of an equality expression
func (impl *CBackendImplementation) GetTypeOfEquality(e *tokens.Equality, scope *ast.Ast) *tokens.TypeRef {
	if e == nil {
		return nil
	}
	// Equality checks return bool
	if e.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfComparison(e.Comparison, scope)
}

// GetTypeOfLogicalAnd gets the type of a logical AND expression
func (impl *CBackendImplementation) GetTypeOfLogicalAnd(la *tokens.LogicalAnd, scope *ast.Ast) *tokens.TypeRef {
	if la == nil {
		return nil
	}
	// Logical AND returns bool
	if la.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfEquality(la.Equality, scope)
}

// GetTypeOfLogicalOr gets the type of a logical OR expression
func (impl *CBackendImplementation) GetTypeOfLogicalOr(lo *tokens.LogicalOr, scope *ast.Ast) *tokens.TypeRef {
	if lo == nil {
		return nil
	}
	// Logical OR returns bool
	if lo.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfLogicalAnd(lo.LogicalAnd, scope)
}

// GetTypeOfExpression gets the type of an expression
func (impl *CBackendImplementation) GetTypeOfExpression(e *tokens.Expression, scope *ast.Ast) *tokens.TypeRef {
	if e == nil {
		return nil
	}
	if CurrentSemanticProgram != nil {
		if inferred := CurrentSemanticProgram.TypeOfExpression(e); inferred != nil {
			return inferred
		}
	}
	return impl.GetTypeOfOrExpression(e.Cond.LogicalOr, scope)
}

// GetTypeOfOrExpression gets the type of an `or` expression chain.
func (impl *CBackendImplementation) GetTypeOfOrExpression(o *tokens.OrExpression, scope *ast.Ast) *tokens.TypeRef {
	if o == nil {
		return nil
	}

	leftType := impl.GetTypeOfLogicalOr(o.LogicalOr, scope)
	if o.Or == nil {
		return leftType
	}

	// `expr or fallback` resolves to the wrapped value type.
	if valueType := impl.resolveHookValueType(leftType, scope, hooks.HookOr, "unwrap_or"); valueType != nil {
		return valueType
	}

	// Fallback for non-hook paths (e.g., pointer/null idioms): use RHS type if known.
	rightType := impl.GetTypeOfOrExpression(o.Or, scope)
	if rightType != nil {
		return rightType
	}

	return leftType
}
