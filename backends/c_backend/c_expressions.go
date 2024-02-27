package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

func (impl *CBackendImplementation) ExpressionToCString(e *tokens.Expression, scope *ast.Ast, expressionType *tokens.TypeRef) string {
	var base string

	eq := e.Equality

	base = impl.EqualityToCString(eq, scope, expressionType)

	return base
}

func (impl *CBackendImplementation) EqualityToCString(eq *tokens.Equality, scope *ast.Ast, expressionType *tokens.TypeRef) string {
	var base string

	if eq.Next != nil {

		base = impl.ComparisonToCString(eq.Comparison, scope, expressionType)
		base += eq.Op
		base += impl.EqualityToCString(eq.Next, scope, expressionType)
	} else {
		base = impl.ComparisonToCString(eq.Comparison, scope, expressionType)
	}

	return base
}

func (impl *CBackendImplementation) ComparisonToCString(c *tokens.Comparison, scope *ast.Ast, expressionType *tokens.TypeRef) string {
	var base string

	if c.Next != nil {
		base = impl.AdditionToCString(c.Addition, scope, expressionType)
		base += c.Op
		base += impl.ComparisonToCString(c.Next, scope, expressionType)
	} else {
		base = impl.AdditionToCString(c.Addition, scope, expressionType)
	}

	return base
}

func (impl *CBackendImplementation) AdditionToCString(a *tokens.Addition, scope *ast.Ast, expressionType *tokens.TypeRef) string {
	var base string

	if a.Next != nil {
		base = impl.MultiplicationToCString(a.Multiplication, scope, expressionType)
		base += a.Op
		base += impl.AdditionToCString(a.Next, scope, expressionType)
	} else {
		base = impl.MultiplicationToCString(a.Multiplication, scope, expressionType)
	}

	return base
}

func (impl *CBackendImplementation) MultiplicationToCString(m *tokens.Multiplication, scope *ast.Ast, expressionType *tokens.TypeRef) string {
	var base string

	if m.Next != nil {
		base = impl.UnaryToCString(m.Unary, scope, expressionType)
		base += m.Op
		base += impl.MultiplicationToCString(m.Next, scope, expressionType)
	} else {
		base = impl.UnaryToCString(m.Unary, scope, expressionType)
	}

	return base
}

func (impl *CBackendImplementation) UnaryToCString(u *tokens.Unary, scope *ast.Ast, expressionType *tokens.TypeRef) string {
	var base string

	if u.Unary != nil {
		if u.Op != "!" {
			scope.ErrorScope.NewCompileTimeError("Unknown unary operator", "unknown unary operator '"+u.Op+"'", u.Unary.Pos)
		}

		base = u.Op
		base += impl.UnaryToCString(u.Unary, scope, expressionType)
	} else if u.Primary != nil {
		base = impl.PrimaryToCString(u.Primary, scope, expressionType)
	}

	return base
}

func (impl *CBackendImplementation) PrimaryToCString(p *tokens.Primary, scope *ast.Ast, expressionType *tokens.TypeRef) string {
	var base string = ""

	if p.Literal != nil && p.Literal.IsPointer {
		base += "&"
	}

	if p.SubExpression != nil {
		base += impl.ExpressionToCString(p.SubExpression, scope, expressionType)
	} else if p.Literal.Bool != "" {
		base += p.Literal.Bool
	} else if p.Literal.Number != "" {
		// conv := option.SomePair(strconv.Atoi(p.Literal.Number)).Unwrap()

		base += p.Literal.Number
	} else if p.Literal.String != "" {
		base += p.Literal.String
	} else if p.Literal.Array != nil {
		base += "{"

		for _, e := range p.Literal.Array {
			prim := &tokens.Primary{
				Literal: e,
			}
			v := impl.PrimaryToCString(prim, scope, expressionType)

			base += v + ","
		}

		base = base[:len(base)-1]

		base += "}"
	} else if p.Literal.ArrayIndex != nil {
		index := &tokens.Primary{
			Literal: p.Literal.ArrayIndex,
		}

		val := impl.PrimaryToCString(index, scope, expressionType)

		base = "[" + val + "]"
	} else if p.Literal.Symbol != "" {
		symbolVariable := scope.ResolveSymbolAsVariable(p.Literal.Symbol)

		if !symbolVariable.IsNil() {
			variable := symbolVariable.Unwrap()

			base = variable.GetFullName()
		} else {
			scope.ErrorScope.NewCompileTimeError("Variable Resolution Error", "Unable to resolve the variable '"+p.Literal.Symbol+"'", p.Pos)
		}
	} else if p.Literal.FuncCall != nil {
		// base = p.FuncCall.(scope)
		base += p.Literal.FuncCall.Function
		base += "("

		for _, a := range p.Literal.FuncCall.Arguments {
			if a.Value != nil {
				base += impl.ExpressionToCString(a.Value, scope, expressionType)
			} else if a.SubCall != nil {
				base += impl.PrimaryToCString(&tokens.Primary{
					Literal: &tokens.Literal{
						FuncCall: a.SubCall,
					},
				}, scope, expressionType)
			}
			base += ","
		}

		base = base[:len(base)-1]
		base += ")"
	}

	return base
}
