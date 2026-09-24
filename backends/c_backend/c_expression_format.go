// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

func geckoTypeRefToString(t *tokens.TypeRef) string {
	if t == nil {
		return ""
	}
	if t.Array != nil {
		return "[" + geckoTypeRefToString(t.Array) + "]"
	}
	if t.Size != nil {
		return "[" + t.Size.Size + "]" + geckoTypeRefToString(t.Size.Type)
	}
	if t.FuncType != nil {
		params := make([]string, len(t.FuncType.ParamTypes))
		for i, p := range t.FuncType.ParamTypes {
			params[i] = geckoTypeRefToString(p)
		}
		out := "func(" + strings.Join(params, ", ") + ")"
		if t.FuncType.ReturnType != nil {
			out += ": " + geckoTypeRefToString(t.FuncType.ReturnType)
		}
		if t.FuncType.Throws != nil {
			out += " throws " + geckoTypeRefToString(t.FuncType.Throws)
		}
		return out
	}

	base := t.Type
	if t.Module != "" {
		base = t.Module + "." + t.Type
	}
	if len(t.TypeArgs) > 0 {
		args := make([]string, len(t.TypeArgs))
		for i, arg := range t.TypeArgs {
			args[i] = geckoTypeRefToString(arg)
		}
		base += "<" + strings.Join(args, ", ") + ">"
	}
	if t.Trait != "" {
		base += " is " + t.Trait
	}
	if t.Const {
		base += " readonly"
	}
	if t.Volatile {
		base += " volatile"
	}
	if t.Pointer {
		base += "*"
	}
	if t.NonNull {
		base += "!"
	}
	return base
}

func geckoExpressionToString(e *tokens.Expression) string {
	if e == nil || e.Cond == nil || e.Cond.LogicalOr == nil {
		return ""
	}
	return geckoOrExpressionToString(e.Cond.LogicalOr)
}

func geckoOrExpressionToString(o *tokens.OrExpression) string {
	if o == nil || o.LogicalOr == nil {
		return ""
	}
	base := geckoLogicalOrToString(o.LogicalOr)
	if o.Or != nil {
		base += " or " + geckoOrExpressionToString(o.Or)
	}
	return base
}

func geckoLogicalOrToString(lo *tokens.LogicalOr) string {
	if lo == nil || lo.LogicalAnd == nil {
		return ""
	}
	base := geckoLogicalAndToString(lo.LogicalAnd)
	if lo.Next != nil {
		base += " " + lo.Op + " " + geckoLogicalOrToString(lo.Next)
	}
	return base
}

func geckoLogicalAndToString(la *tokens.LogicalAnd) string {
	if la == nil || la.Equality == nil {
		return ""
	}
	base := geckoEqualityToString(la.Equality)
	if la.Next != nil {
		base += " " + la.Op + " " + geckoLogicalAndToString(la.Next)
	}
	return base
}

func geckoEqualityToString(eq *tokens.Equality) string {
	if eq == nil || eq.Comparison == nil {
		return ""
	}
	base := geckoComparisonToString(eq.Comparison)
	if eq.Next != nil {
		base += " " + eq.Op + " " + geckoEqualityToString(eq.Next)
	}
	return base
}

func geckoComparisonToString(c *tokens.Comparison) string {
	if c == nil || c.Addition == nil {
		return ""
	}
	base := geckoAdditionToString(c.Addition)
	if c.Next != nil {
		base += " " + c.Op + " " + geckoComparisonToString(c.Next)
	}
	return base
}

func geckoAdditionToString(a *tokens.Addition) string {
	if a == nil || a.Multiplication == nil {
		return ""
	}
	base := geckoMultiplicationToString(a.Multiplication)
	if a.Next != nil {
		base += " " + a.Op + " " + geckoAdditionToString(a.Next)
	}
	return base
}

func geckoMultiplicationToString(m *tokens.Multiplication) string {
	if m == nil || m.Unary == nil {
		return ""
	}
	base := geckoUnaryToString(m.Unary)
	if m.Next != nil {
		base += " " + m.Op + " " + geckoMultiplicationToString(m.Next)
	}
	return base
}

func geckoUnaryToString(u *tokens.Unary) string {
	if u == nil {
		return ""
	}

	var base string
	if u.Unary != nil {
		if u.Op == "try" {
			base = "try " + geckoUnaryToString(u.Unary)
		} else {
			base = u.Op + geckoUnaryToString(u.Unary)
		}
	} else if u.Primary != nil {
		base = geckoPrimaryToString(u.Primary)
	}

	if u.Cast != nil && u.Cast.Type != nil {
		if u.Cast.Trusted {
			base += " as! " + geckoTypeRefToString(u.Cast.Type)
		} else {
			base += " as " + geckoTypeRefToString(u.Cast.Type)
		}
	}
	return base
}

func geckoPrimaryToString(p *tokens.Primary) string {
	if p == nil {
		return ""
	}
	if p.SubExpression != nil {
		return "(" + geckoExpressionToString(p.SubExpression) + ")"
	}
	return geckoLiteralToString(p.Literal)
}

func geckoTypeArgsToString(typeArgs []*tokens.TypeRef) string {
	if len(typeArgs) == 0 {
		return ""
	}
	args := make([]string, len(typeArgs))
	for i, arg := range typeArgs {
		args[i] = geckoTypeRefToString(arg)
	}
	return "<" + strings.Join(args, ", ") + ">"
}

func geckoArgumentToString(arg *tokens.Argument) string {
	if arg == nil {
		return ""
	}
	base := ""
	if arg.Name != "" {
		base += arg.Name + ": "
	}
	if arg.Out {
		base += "out "
	}
	if arg.Value != nil {
		base += geckoExpressionToString(arg.Value)
	} else if arg.SubCall != nil {
		base += geckoFuncCallToString(arg.SubCall)
	}
	return base
}

func geckoFuncCallToString(f *tokens.FuncCall) string {
	if f == nil {
		return ""
	}

	base := ""
	if f.StaticType != "" {
		if f.StaticModule != "" {
			base += f.StaticModule + "."
		}
		base += f.StaticType + geckoTypeArgsToString(f.StaticTypeArgs) + "::" + f.Function
	} else if f.Module != "" {
		base += f.Module + "." + f.Function
	} else {
		base += f.Function
	}
	base += geckoTypeArgsToString(f.TypeArgs)

	args := make([]string, len(f.Arguments))
	for i, arg := range f.Arguments {
		args[i] = geckoArgumentToString(arg)
	}
	return base + "(" + strings.Join(args, ", ") + ")"
}

func geckoIntrinsicToString(i *tokens.Intrinsic) string {
	if i == nil {
		return ""
	}
	args := make([]string, len(i.Args))
	for idx, arg := range i.Args {
		args[idx] = geckoExpressionToString(arg)
	}
	return "@" + i.Name + geckoTypeArgsToString(i.TypeArgs) + "(" + strings.Join(args, ", ") + ")"
}

func geckoLiteralToString(l *tokens.Literal) string {
	if l == nil {
		return ""
	}

	base := ""
	switch {
	case l.Bool != "":
		base = l.Bool
	case l.HasStringLiteral():
		base = l.StringLiteralToken()
	case l.Symbol != "":
		if l.SymbolModule != "" {
			base = l.SymbolModule + "." + l.Symbol
		} else {
			base = l.Symbol
		}
	case l.Number != "":
		base = l.Number
	case l.Intrinsic != nil:
		base = geckoIntrinsicToString(l.Intrinsic)
	case l.FuncCall != nil:
		base = geckoFuncCallToString(l.FuncCall)
	case len(l.Array) > 0:
		items := make([]string, len(l.Array))
		for i, item := range l.Array {
			items[i] = geckoLiteralToString(item)
		}
		base = "[" + strings.Join(items, ", ") + "]"
	case l.IsStructLiteral():
		fields := make([]string, len(l.StructFields))
		for i, kv := range l.StructFields {
			fields[i] = kv.Key + ": " + geckoExpressionToString(kv.Value)
		}
		base = l.StructType + geckoTypeArgsToString(l.StructTypeArgs) + " { " + strings.Join(fields, ", ") + " }"
	case len(l.Object) > 0:
		fields := make([]string, len(l.Object))
		for i, kv := range l.Object {
			fields[i] = kv.Key + ": " + geckoExpressionToString(kv.Value)
		}
		base = "{ " + strings.Join(fields, ", ") + " }"
	}

	for _, chain := range l.Chain {
		base += "." + chain.Name + geckoTypeArgsToString(chain.TypeArgs)
		if chain.IsMethodCall() {
			args := make([]string, len(chain.GetArgs()))
			for i, arg := range chain.GetArgs() {
				args[i] = geckoArgumentToString(arg)
			}
			base += "(" + strings.Join(args, ", ") + ")"
		}
	}

	if l.ArrayIndex != nil {
		base += "[" + geckoExpressionToString(l.ArrayIndex) + "]"
	}
	if l.IsPointer {
		base = "&" + base
	}
	return base
}
