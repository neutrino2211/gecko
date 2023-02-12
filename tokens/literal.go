package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (l *Literal) ToCString(scope *ast.Ast) string {
	base := ""

	if l.Bool != "" {
		base = l.Bool
	} else if l.String != "" {
		base = l.String
	} else if l.Symbol != "" {
		symbolVariable := scope.ResolveSymbolAsVariable(l.Symbol)

		if !symbolVariable.IsNil() {
			base = symbolVariable.Unwrap().GetFullName()
		}
	} else if l.Number != "" {
		base = l.Number
	} else if len(l.Array) != 0 {
		base += "{"
		for _, arrayLit := range l.Array[:len(l.Array)-1] {
			base += arrayLit.ToCString(scope) + ", "
		}

		base += l.Array[len(l.Array)-1].ToCString(scope)

		base += "}"
	} else if l.Expression != nil {
		base = l.Expression.ToCString(scope)
	}

	if base != "" && l.ArrayIndex != nil {
		base += "[" + l.ArrayIndex.ToCString(scope) + "]"
	}

	return base
}
