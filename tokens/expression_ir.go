package tokens

import (
	"strconv"
	"strings"

	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/go-option"
)

func (e *Expression) ToLLIRValue(scope *ast.Ast) value.Value {
	var base value.Value

	eq := e.Equality

	base = eq.ToLLIRValue(scope)

	return base
}

func (eq *Equality) ToLLIRValue(scope *ast.Ast) value.Value {
	var base value.Value

	if eq.Next != nil {
		// base += eq.Op
		// base += eq.Next.ToLLIRValue(scope)
	} else {
		base = eq.Comparison.ToLLIRValue(scope)
	}

	return base
}

func (c *Comparison) ToLLIRValue(scope *ast.Ast) value.Value {
	var base value.Value

	if c.Next != nil {
		// base += c.Op
		// base += c.Next.ToLLIRValue(scope)
	} else {
		base = c.Addition.ToLLIRValue(scope)
	}

	return base
}

func (a *Addition) ToLLIRValue(scope *ast.Ast) value.Value {
	var base value.Value

	if a.Next != nil {
		// base += a.Op
		// base += a.Next.ToLLIRValue(scope)
	} else {
		base = a.Multiplication.ToLLIRValue(scope)
	}

	return base
}

func (m *Multiplication) ToLLIRValue(scope *ast.Ast) value.Value {
	var base value.Value

	if m.Next != nil {
		// base += m.Op
		// base += m.Next.ToLLIRValue(scope)
	} else {
		base = m.Unary.ToLLIRValue(scope)
	}

	return base
}

func (u *Unary) ToLLIRValue(scope *ast.Ast) value.Value {
	var base value.Value

	if u.Unary != nil {
		// base += u.Op
		// base += u.Unary.ToLLIRValue(scope)
	} else if u.Primary != nil {
		base = u.Primary.ToLLIRValue(scope)
	}

	return base
}

func (p *Primary) ToLLIRValue(scope *ast.Ast) value.Value {
	var base value.Value

	if p.SubExpression != nil {
		base = p.SubExpression.ToLLIRValue(scope)
	} else if p.Literal.Bool != "" {
		i := map[string]int64{"true": 1, "false": 0}[p.Literal.Bool]
		base = constant.NewInt(types.I1, i)
	} else if p.Literal.Number != "" {
		conv := option.SomePair(strconv.Atoi(p.Literal.Number))
		base = constant.NewInt(types.I64, int64(conv.Unwrap()))
	} else if p.Literal.String != "" {
		actual, err := strconv.Unquote(p.Literal.String)

		if err != nil {
			scope.ErrorScope.NewCompileTimeError("String Escape", "unable to escape the string provided "+err.Error(), p.Pos)
			actual = ""
		}

		str := constant.NewCharArrayFromString(actual + string('\x00'))
		println(actual, p.Literal.IsPointer)
		if p.Literal.IsPointer {
			def := scope.ProgramContext.Module.NewGlobalDef("__"+scope.FullScopeName()+strings.ReplaceAll(actual, " ", "_"), str)
			def.Linkage = enum.LinkagePrivate
			def.UnnamedAddr = enum.UnnamedAddrUnnamedAddr
			def.Immutable = true
			base = scope.LocalContext.MainBlock.NewGetElementPtr(str.Typ, def, constant.NewInt(types.I8, 0))
		} else {
			base = str
		}
	} else if p.Literal.Symbol != "" {
		// symbolVariable := scope.ResolveSymbolAsVariable(p.Symbol)

		// if !symbolVariable.IsNil() {
		// 	base = symbolVariable.Unwrap().GetFullName()
		// }
	} else if p.Literal.FuncCall != nil {
		// base = p.FuncCall.(scope)
	}

	return base
}
