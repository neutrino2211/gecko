package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (e *Expression) ToCString(scope *ast.Ast) string {
	base := ""

	eq := e.Equality

	base += eq.ToCString(scope)

	return base
}

func (eq *Equality) ToCString(scope *ast.Ast) string {
	base := ""

	base += eq.Comparison.ToCString(scope)

	if eq.Next != nil {
		base += eq.Op
		base += eq.Next.ToCString(scope)
	}

	return base
}

func (c *Comparison) ToCString(scope *ast.Ast) string {
	base := ""

	base += c.Addition.ToCString(scope)

	if c.Next != nil {
		base += c.Op
		base += c.Next.ToCString(scope)
	}

	return base
}

func (a *Addition) ToCString(scope *ast.Ast) string {
	base := ""

	base += a.Multiplication.ToCString(scope)

	if a.Next != nil {
		base += a.Op
		base += a.Next.ToCString(scope)
	}

	return base
}

func (m *Multiplication) ToCString(scope *ast.Ast) string {
	base := ""

	base += m.Unary.ToCString(scope)

	if m.Next != nil {
		base += m.Op
		base += m.Next.ToCString(scope)
	}

	return base
}

func (u *Unary) ToCString(scope *ast.Ast) string {
	base := ""

	if u.Unary != nil {
		base += u.Op
		base += u.Unary.ToCString(scope)
	} else if u.Primary != nil {
		base += u.Primary.ToCString(scope)
	}

	return base
}

func (p *Primary) ToCString(scope *ast.Ast) string {
	base := ""

	if p.SubExpression != nil {
		base = p.SubExpression.ToCString(scope)
	} else if p.Bool != "" {
		base = p.Bool
	} else if p.Number != "" {
		base = p.Number
	} else if p.String != "" {
		base = p.String
	} else if p.Symbol != "" {
		symbolVariable := scope.ResolveSymbolAsVariable(p.Symbol)

		if !symbolVariable.IsNil() {
			base = symbolVariable.Unwrap().GetFullName()
		}
	} else if p.FuncCall != nil {
		base = p.FuncCall.ToCString(scope)
	}

	return base
}
