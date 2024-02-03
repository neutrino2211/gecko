package llvmbackend

import (
	"strconv"

	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
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

func (impl *LLVMBackendImplementation) ExpressionToLLIRValue(e *tokens.Expression, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	var base value.Value

	eq := e.Equality

	base = impl.EqualityToLLIRValue(eq, scope, expressionType)

	return base
}

func (impl *LLVMBackendImplementation) EqualityToLLIRValue(eq *tokens.Equality, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	var base value.Value

	if eq.Next != nil {
		v := impl.EqualityToLLIRValue(eq.Next, scope, expressionType)
		info := LLVMGetScopeInformation(scope)

		base = info.LocalContext.MainBlock.NewICmp(equalityOps[eq.Op], impl.ComparisonToLLIRValue(eq.Comparison, scope, expressionType), v)
		// base += eq.Op
		// base += eq.Next.ToLLIRValue(scope)
	} else {
		base = impl.ComparisonToLLIRValue(eq.Comparison, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) ComparisonToLLIRValue(c *tokens.Comparison, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	var base value.Value

	if c.Next != nil {
		// base += c.Op
		// base += c.Next.ToLLIRValue(scope)
	} else {
		base = impl.AdditionToLLIRValue(c.Addition, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) AdditionToLLIRValue(a *tokens.Addition, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	var base value.Value

	if a.Next != nil {
		// base += a.Op
		// base += a.Next.ToLLIRValue(scope)
	} else {
		base = impl.MultiplicationToLLIRValue(a.Multiplication, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) MultiplicationToLLIRValue(m *tokens.Multiplication, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	var base value.Value

	if m.Next != nil {
		// base += m.Op
		// base += m.Next.ToLLIRValue(scope)
	} else {
		base = impl.UnaryToLLIRValue(m.Unary, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) UnaryToLLIRValue(u *tokens.Unary, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	var base value.Value

	if u.Unary != nil {
		if u.Op != "!" {
			scope.ErrorScope.NewCompileTimeError("Unknown unary operator", "unknown unary operator '"+u.Op+"'", u.Unary.Pos)
		}

		// base += u.Op
		// base += u.Unary.ToLLIRValue(scope)
	} else if u.Primary != nil {
		base = impl.PrimaryToLLIRValue(u.Primary, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) PrimaryToLLIRValue(p *tokens.Primary, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	var base value.Value

	info := LLVMGetScopeInformation(scope)

	if p.SubExpression != nil {
		base = impl.ExpressionToLLIRValue(p.SubExpression, scope, expressionType)
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
			def := info.ProgramContext.Module.NewGlobalDef(".str."+p.GetID(), str)
			def.Linkage = enum.LinkagePrivate

			if expressionType.Const {
				def.UnnamedAddr = enum.UnnamedAddrUnnamedAddr
				def.Immutable = true
			}

			base = info.LocalContext.MainBlock.NewGetElementPtr(str.Typ, def, constant.NewInt(types.I8, 0))
		} else {
			base = str
		}
	} else if p.Literal.Array != nil {
		memType := &types.ArrayType{
			Len:      uint64(len(p.Literal.Array)),
			ElemType: impl.TypeRefGetLLIRType(expressionType.Array, scope),
		}

		mem := info.LocalContext.MainBlock.NewAlloca(memType)

		memDirect := info.LocalContext.MainBlock.NewExtractValue(mem, 0)

		for i, e := range p.Literal.Array {
			p := &tokens.Primary{
				Literal: e,
			}
			v := impl.PrimaryToLLIRValue(p, scope, expressionType)
			repr.Println(v.Type().LLString(), memDirect.Typ.LLString())

			// info.LocalContext.MainBlock.NewStore(v, mem)

			info.LocalContext.MainBlock.NewInsertValue(memDirect, v, uint64(i))
		}

		base = mem
	} else if p.Literal.ArrayIndex != nil {
		index := *p.Literal.ArrayIndex
		p.Literal.ArrayIndex = nil
		val := impl.PrimaryToLLIRValue(p, scope, expressionType)

		arrayType := val.Type()
		arrayIndexVal := &tokens.Primary{Literal: &index}
		arrayIndex := impl.PrimaryToLLIRValue(arrayIndexVal, scope, expressionType)

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

			arrayPtr := info.LocalContext.MainBlock.NewPtrToInt(val, pointerType)
			ptrOffset := info.LocalContext.MainBlock.NewMul(arrayPtr, arrayIndex)
			offset := info.LocalContext.MainBlock.NewAdd(arrayPtr, ptrOffset)
			elPtr := info.LocalContext.MainBlock.NewIntToPtr(offset, elType)
			base = info.LocalContext.MainBlock.NewLoad(elType, elPtr)

			// base = info.LocalContext.MainBlock.NewExtractValue(val, 0)
		}

	} else if p.Literal.Symbol != "" {
		symbolVariable := scope.ResolveSymbolAsVariable(p.Literal.Symbol)

		if !symbolVariable.IsNil() {
			variable := symbolVariable.Unwrap()

			base = LLVMGetValueInformation(variable).Value
			repr.Println(variable.GetFullName(), base)
			// repr.Println(symbolVariable.Unwrap().GetLLIRType(scope))
		} else {
			scope.ErrorScope.NewCompileTimeError("Variable Resolution Error", "Unable to resolve the variable '"+p.Literal.Symbol+"'", p.Pos)
		}
	} else if p.Literal.FuncCall != nil {
		// base = p.FuncCall.(scope)
		CurrentBackend.GetImpls().FuncCall(scope, p.Literal.FuncCall)

		call := FuncCalls[scope.FullScopeName()+"#"+p.Literal.FuncCall.Function]

		if call != nil {
			base = call
		}
	}

	return base
}
