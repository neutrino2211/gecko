package tokens

import (
	"fmt"
	"strconv"

	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/go-option"
)

type ValueAsContant struct {
	value.Value
}

func (v ValueAsContant) IsConstant() {}

var equalityOps map[string]enum.IPred = map[string]enum.IPred{
	"!=": enum.IPredNE,
	"==": enum.IPredEQ,
}

func (e *Expression) ToLLIRValue(scope *ast.Ast, expressionType *TypeRef) value.Value {
	var base value.Value

	eq := e.Equality

	base = eq.ToLLIRValue(scope, expressionType)

	return base
}

func (eq *Equality) ToLLIRValue(scope *ast.Ast, expressionType *TypeRef) value.Value {
	var base value.Value

	if eq.Next != nil {
		v := eq.Next.ToLLIRValue(scope, expressionType)
		base = scope.LocalContext.MainBlock.NewICmp(equalityOps[eq.Op], eq.Comparison.ToLLIRValue(scope, expressionType), v)
		// base += eq.Op
		// base += eq.Next.ToLLIRValue(scope)
	} else {
		base = eq.Comparison.ToLLIRValue(scope, expressionType)
	}

	return base
}

func (c *Comparison) ToLLIRValue(scope *ast.Ast, expressionType *TypeRef) value.Value {
	var base value.Value

	if c.Next != nil {
		// base += c.Op
		// base += c.Next.ToLLIRValue(scope)
	} else {
		base = c.Addition.ToLLIRValue(scope, expressionType)
	}

	return base
}

func (a *Addition) ToLLIRValue(scope *ast.Ast, expressionType *TypeRef) value.Value {
	var base value.Value

	if a.Next != nil {
		// base += a.Op
		// base += a.Next.ToLLIRValue(scope)
	} else {
		base = a.Multiplication.ToLLIRValue(scope, expressionType)
	}

	return base
}

func (m *Multiplication) ToLLIRValue(scope *ast.Ast, expressionType *TypeRef) value.Value {
	var base value.Value

	if m.Next != nil {
		// base += m.Op
		// base += m.Next.ToLLIRValue(scope)
	} else {
		base = m.Unary.ToLLIRValue(scope, expressionType)
	}

	return base
}

func (u *Unary) ToLLIRValue(scope *ast.Ast, expressionType *TypeRef) value.Value {
	var base value.Value

	if u.Unary != nil {
		if u.Op != "!" {
			scope.ErrorScope.NewCompileTimeError("Unknown unary operator", "unknown unary operator '"+u.Op+"'", u.Unary.Pos)
		}

		// base += u.Op
		// base += u.Unary.ToLLIRValue(scope)
	} else if u.Primary != nil {
		base = u.Primary.ToLLIRValue(scope, expressionType)
	}

	return base
}

func (p *Primary) ToLLIRValue(scope *ast.Ast, expressionType *TypeRef) value.Value {
	var base value.Value

	if p.SubExpression != nil {
		base = p.SubExpression.ToLLIRValue(scope, expressionType)
	} else if p.Literal.Bool != "" {
		i := map[string]int64{"true": 1, "false": 0}[p.Literal.Bool]
		base = constant.NewInt(types.I1, i)
	} else if p.Literal.Number != "" {
		conv := option.SomePair(strconv.Atoi(p.Literal.Number)).Unwrap()

		if p.Literal.IsPointer {
			base = constant.NewInt(types.I1, 0)
			scope.ErrorScope.NewCompileTimeError("Invalid Pointer", "cannot take a pointer of the raw value '"+strconv.Itoa(conv)+"'", p.Literal.Pos)
		} else {
			base = constant.NewInt(types.I64, int64(conv))
		}
	} else if p.Literal.String != "" {
		actual, err := strconv.Unquote(p.Literal.String)

		if err != nil {
			scope.ErrorScope.NewCompileTimeError("String Escape", "unable to escape the string provided "+err.Error(), p.Literal.Pos)
			actual = ""
		}

		str := constant.NewCharArrayFromString(actual + string('\x00'))
		println(actual, p.Literal.IsPointer)
		if p.Literal.IsPointer {
			def := scope.ProgramContext.Module.NewGlobalDef(".str."+p.GetID(), str)
			def.Linkage = enum.LinkagePrivate

			if expressionType.Const {
				def.UnnamedAddr = enum.UnnamedAddrUnnamedAddr
				def.Immutable = true
			}
			base = scope.LocalContext.MainBlock.NewGetElementPtr(str.Typ, def, constant.NewInt(types.I8, 0))
		} else {
			base = str
		}
	} else if p.Literal.Array != nil {
		memType := &types.ArrayType{
			Len:      uint64(len(p.Literal.Array)),
			ElemType: expressionType.Array.GetLLIRType(scope),
		}
		mem := scope.LocalContext.MainBlock.NewAlloca(memType)
		memDirect := scope.LocalContext.MainBlock.NewExtractValue(mem, 0)

		for i, e := range p.Literal.Array {
			p := &Primary{
				Literal: e,
			}
			v := p.ToLLIRValue(scope, expressionType)
			repr.Println(v.Type().LLString(), memDirect.Typ.LLString())
			// scope.LocalContext.MainBlock.NewStore(v, mem)
			scope.LocalContext.MainBlock.NewInsertValue(memDirect, v, uint64(i))
		}

		base = mem
	} else if p.Literal.ArrayIndex != nil {
		index := *p.Literal.ArrayIndex
		p.Literal.ArrayIndex = nil
		val := p.ToLLIRValue(scope, expressionType)

		arrayType := val.Type()
		arrayIndexVal := &Primary{Literal: &index}
		arrayIndex := arrayIndexVal.ToLLIRValue(scope, expressionType)

		if arrayType == nil {
			scope.ErrorScope.NewCompileTimeWarning("Invalid Expression Type", "the type for the following expression ended up being invalid, weird", p.Pos)
		}

		repr.Println(arrayType, arrayIndex, val)

		if val == nil {
			scope.ErrorScope.NewCompileTimeError("Parse Error", "unable to parse the expression", p.Literal.Pos)
		} else {
			elType := val.Type()
			pointerType := &types.PointerType{
				ElemType: &types.IntType{
					BitSize: 8,
				},
			}
			arrayPtr := scope.LocalContext.MainBlock.NewPtrToInt(val, pointerType)
			ptrOffset := scope.LocalContext.MainBlock.NewMul(arrayPtr, arrayIndex)
			offset := scope.LocalContext.MainBlock.NewAdd(arrayPtr, ptrOffset)
			elPtr := scope.LocalContext.MainBlock.NewIntToPtr(offset, elType)
			base = scope.LocalContext.MainBlock.NewLoad(elType, elPtr)

			// base = scope.LocalContext.MainBlock.NewExtractValue(val, 0)
		}

	} else if p.Literal.Symbol != "" {
		symbolVariable := scope.ResolveSymbolAsVariable(p.Literal.Symbol)

		fmt.Println(scope.ToFMTString())

		if !symbolVariable.IsNil() {
			variable := symbolVariable.Unwrap()

			base = variable.Value
			// repr.Println(symbolVariable.Unwrap().GetLLIRType(scope))
		} else {
			scope.ErrorScope.NewCompileTimeError("Variable Resolution Error", "Unable to resolve the variable '"+p.Literal.Symbol+"'", p.Pos)
		}
	} else if p.Literal.FuncCall != nil {
		// base = p.FuncCall.(scope)
		call := p.Literal.FuncCall.AddToLLIR(scope)

		if !call.IsNil() {
			base = call.Unwrap()
		}
	}

	return base
}
