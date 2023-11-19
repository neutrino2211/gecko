package tokens

import (
	"strconv"

	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/go-option"
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
	}

	if base != "" && l.ArrayIndex != nil {
		base += "[" + l.ArrayIndex.ToCString(scope) + "]"
	}

	return base
}

func (l *Literal) ToLLIRValue(scope *ast.Ast) value.Value {
	var val value.Value

	if l.Bool != "" {
		i := map[string]int64{"true": 1, "false": 0}[l.Bool]
		val = constant.NewInt(types.I1, i)
	} else if l.String != "" {
		val = constant.NewCharArrayFromString(l.String)
	} else if l.Number != "" {
		conv := option.SomePair(strconv.Atoi(l.Number)).Unwrap()
		val = constant.NewInt(types.I64, int64(conv))
	}

	return val
}
