// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"strconv"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

type ValueAsContant struct {
	value.Value
}

func (v ValueAsContant) IsConstant() {}

var equalityOps map[string]enum.IPred = map[string]enum.IPred{
	"!=": enum.IPredNE,
	"==": enum.IPredEQ,
}

var comparisonOps map[string]enum.IPred = map[string]enum.IPred{
	">":  enum.IPredSGT, // signed greater than
	">=": enum.IPredSGE, // signed greater than or equal
	"<":  enum.IPredSLT, // signed less than
	"<=": enum.IPredSLE, // signed less than or equal
}

var unsignedComparisonOps map[string]enum.IPred = map[string]enum.IPred{
	">":  enum.IPredUGT, // unsigned greater than
	">=": enum.IPredUGE, // unsigned greater than or equal
	"<":  enum.IPredULT, // unsigned less than
	"<=": enum.IPredULE, // unsigned less than or equal
}

func (impl *LLVMBackendImplementation) requireBinaryOperands(scope *ast.Ast, op string, left value.Value, right value.Value, pos lexer.Position) bool {
	if left == nil || right == nil {
		scope.ErrorScope.NewCompileTimeError("Expression Error", "unable to evaluate operands for operator '"+op+"'", pos)
		return false
	}
	return true
}

func (impl *LLVMBackendImplementation) coerceComparisonOperands(scope *ast.Ast, left value.Value, right value.Value, pos lexer.Position) (value.Value, value.Value, bool) {
	if left == nil || right == nil {
		return left, right, false
	}
	if left.Type().Equal(right.Type()) {
		return left, right, true
	}

	if coercedRight := impl.coerceValueToType(right, left.Type(), scope, pos); coercedRight != nil {
		return left, coercedRight, true
	}
	if coercedLeft := impl.coerceValueToType(left, right.Type(), scope, pos); coercedLeft != nil {
		return coercedLeft, right, true
	}

	return left, right, false
}

func supportsICmpEqualityOperandType(t types.Type) bool {
	switch t.(type) {
	case *types.IntType, *types.PointerType, *types.VectorType:
		return true
	default:
		return false
	}
}

func supportsOrderedComparisonOperandType(t types.Type) bool {
	_, ok := t.(*types.IntType)
	return ok
}

func geckoTypeIsUnsignedInteger(t *tokens.TypeRef) bool {
	if t == nil {
		return false
	}

	switch t.Type {
	case "uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func (impl *LLVMBackendImplementation) inferTypeForAddition(scope *ast.Ast, add *tokens.Addition) *tokens.TypeRef {
	if add == nil {
		return nil
	}

	resolveSymbol := func(name string) *tokens.TypeRef {
		opt := scope.ResolveSymbolAsVariable(name)
		if opt.IsNil() {
			return nil
		}
		v := opt.Unwrap()
		info := LLVMGetValueInformation(v)
		return info.GeckoType
	}

	expr := &tokens.Expression{
		OrExpr: &tokens.OrExpression{
			LogicalOr: &tokens.LogicalOr{
				LogicalAnd: &tokens.LogicalAnd{
					Equality: &tokens.Equality{
						Comparison: &tokens.Comparison{
							Addition: add,
						},
					},
				},
			},
		},
	}

	return tokens.InferType(expr, resolveSymbol)
}

func (impl *LLVMBackendImplementation) ExpressionToLLIRValue(e *tokens.Expression, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if e == nil || e.GetLogicalOr() == nil {
		return nil
	}

	var base value.Value

	lo := e.GetLogicalOr()

	base = impl.LogicalOrToLLIRValue(lo, scope, expressionType)

	return base
}

func (impl *LLVMBackendImplementation) LogicalOrToLLIRValue(lo *tokens.LogicalOr, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if lo == nil {
		return nil
	}

	var base value.Value

	base = impl.LogicalAndToLLIRValue(lo.LogicalAnd, scope, expressionType)

	if lo.Next != nil {
		v := impl.LogicalOrToLLIRValue(lo.Next, scope, expressionType)
		if !impl.requireBinaryOperands(scope, "||", base, v, lo.Pos) {
			return nil
		}
		info := LLVMGetScopeInformation(scope)
		base = info.LocalContext.MainBlock.NewOr(base, v)
	}

	return base
}

func (impl *LLVMBackendImplementation) LogicalAndToLLIRValue(la *tokens.LogicalAnd, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if la == nil {
		return nil
	}

	var base value.Value

	base = impl.EqualityToLLIRValue(la.Equality, scope, expressionType)

	if la.Next != nil {
		v := impl.LogicalAndToLLIRValue(la.Next, scope, expressionType)
		if !impl.requireBinaryOperands(scope, "&&", base, v, la.Pos) {
			return nil
		}
		info := LLVMGetScopeInformation(scope)
		base = info.LocalContext.MainBlock.NewAnd(base, v)
	}

	return base
}

func (impl *LLVMBackendImplementation) EqualityToLLIRValue(eq *tokens.Equality, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if eq == nil {
		return nil
	}

	var base value.Value

	if eq.Next != nil {
		v := impl.EqualityToLLIRValue(eq.Next, scope, expressionType)
		left := impl.ComparisonToLLIRValue(eq.Comparison, scope, expressionType)
		if !impl.requireBinaryOperands(scope, eq.Op, left, v, eq.Pos) {
			return nil
		}
		left, v, ok := impl.coerceComparisonOperands(scope, left, v, eq.Pos)
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Expression Error",
				"unable to compare values of incompatible types in equality expression",
				eq.Pos,
			)
			return nil
		}
		if !supportsICmpEqualityOperandType(left.Type()) || !supportsICmpEqualityOperandType(v.Type()) {
			scope.ErrorScope.NewCompileTimeError(
				"Expression Error",
				"equality comparison requires integer/pointer-compatible operand types",
				eq.Pos,
			)
			return nil
		}
		info := LLVMGetScopeInformation(scope)

		base = info.LocalContext.MainBlock.NewICmp(equalityOps[eq.Op], left, v)
		// base += eq.Op
		// base += eq.Next.ToLLIRValue(scope)
	} else {
		base = impl.ComparisonToLLIRValue(eq.Comparison, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) ComparisonToLLIRValue(c *tokens.Comparison, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if c == nil {
		return nil
	}

	var base value.Value

	if c.Next != nil {
		v := impl.ComparisonToLLIRValue(c.Next, scope, expressionType)
		left := impl.AdditionToLLIRValue(c.Addition, scope, expressionType)
		if !impl.requireBinaryOperands(scope, c.Op, left, v, c.Pos) {
			return nil
		}
		left, v, ok := impl.coerceComparisonOperands(scope, left, v, c.Pos)
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Expression Error",
				"unable to compare values of incompatible types",
				c.Pos,
			)
			return nil
		}
		if !supportsOrderedComparisonOperandType(left.Type()) || !supportsOrderedComparisonOperandType(v.Type()) {
			scope.ErrorScope.NewCompileTimeError(
				"Expression Error",
				"ordered comparison requires integer operand types",
				c.Pos,
			)
			return nil
		}
		info := LLVMGetScopeInformation(scope)
		predicate := comparisonOps[c.Op]
		leftGeckoType := impl.inferTypeForAddition(scope, c.Addition)
		rightGeckoType := impl.inferTypeForAddition(scope, c.Next.Addition)
		if geckoTypeIsUnsignedInteger(leftGeckoType) || geckoTypeIsUnsignedInteger(rightGeckoType) {
			if unsignedPredicate, ok := unsignedComparisonOps[c.Op]; ok {
				predicate = unsignedPredicate
			}
		}

		base = info.LocalContext.MainBlock.NewICmp(predicate, left, v)
	} else {
		base = impl.AdditionToLLIRValue(c.Addition, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) AdditionToLLIRValue(a *tokens.Addition, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if a == nil {
		return nil
	}

	var base value.Value

	if a.Next != nil {
		left := impl.MultiplicationToLLIRValue(a.Multiplication, scope, expressionType)
		right := impl.AdditionToLLIRValue(a.Next, scope, expressionType)
		if !impl.requireBinaryOperands(scope, a.Op, left, right, a.Pos) {
			return nil
		}
		info := LLVMGetScopeInformation(scope)

		switch a.Op {
		case "+":
			base = info.LocalContext.MainBlock.NewAdd(left, right)
		case "-":
			base = info.LocalContext.MainBlock.NewSub(left, right)
		case "|":
			base = info.LocalContext.MainBlock.NewOr(left, right)
		case "&":
			base = info.LocalContext.MainBlock.NewAnd(left, right)
		case "^":
			base = info.LocalContext.MainBlock.NewXor(left, right)
		case ">>>", ">>":
			// Arithmetic right shift (preserves sign)
			base = info.LocalContext.MainBlock.NewAShr(left, right)
		case "<<<", "<<":
			// Left shift
			base = info.LocalContext.MainBlock.NewShl(left, right)
		default:
			scope.ErrorScope.NewCompileTimeError("Unknown operator", "unknown addition-level operator '"+a.Op+"'", a.Pos)
		}
	} else {
		base = impl.MultiplicationToLLIRValue(a.Multiplication, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) MultiplicationToLLIRValue(m *tokens.Multiplication, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if m == nil {
		return nil
	}

	var base value.Value

	if m.Next != nil {
		left := impl.UnaryToLLIRValue(m.Unary, scope, expressionType)
		right := impl.MultiplicationToLLIRValue(m.Next, scope, expressionType)
		if !impl.requireBinaryOperands(scope, m.Op, left, right, m.Pos) {
			return nil
		}
		info := LLVMGetScopeInformation(scope)

		switch m.Op {
		case "*":
			base = info.LocalContext.MainBlock.NewMul(left, right)
		case "/":
			// Signed division for integers
			base = info.LocalContext.MainBlock.NewSDiv(left, right)
		default:
			scope.ErrorScope.NewCompileTimeError("Unknown operator", "unknown multiplication-level operator '"+m.Op+"'", m.Pos)
		}
	} else {
		base = impl.UnaryToLLIRValue(m.Unary, scope, expressionType)
	}

	return base
}

func (impl *LLVMBackendImplementation) UnaryToLLIRValue(u *tokens.Unary, scope *ast.Ast, expressionType *tokens.TypeRef) value.Value {
	if u == nil {
		return nil
	}

	var base value.Value

	if u.Unary != nil {
		operand := impl.UnaryToLLIRValue(u.Unary, scope, expressionType)
		if operand == nil {
			scope.ErrorScope.NewCompileTimeError("Expression Error", "unable to evaluate operand for unary operator '"+u.Op+"'", u.Pos)
			return nil
		}
		info := LLVMGetScopeInformation(scope)

		switch u.Op {
		case "!":
			// Logical NOT: XOR with 1 (for booleans) or compare with 0
			base = info.LocalContext.MainBlock.NewXor(operand, constant.NewInt(types.I1, 1))
		case "-":
			// Negation: subtract from 0
			zero := constant.NewInt(types.I64, 0)
			base = info.LocalContext.MainBlock.NewSub(zero, operand)
		case "+":
			// Unary plus is a no-op
			base = operand
		case "try":
			base = impl.lowerTryUnary(scope, operand, expressionType, u.Pos)
		default:
			scope.ErrorScope.NewCompileTimeError("Unknown unary operator", "unknown unary operator '"+u.Op+"'", u.Unary.Pos)
		}
	} else if u.Primary != nil {
		base = impl.PrimaryToLLIRValue(u.Primary, scope, expressionType)
	}

	// Handle cast expression (e.g., "value as *uint16" or "ptr as uint64")
	if u.Cast != nil && base != nil {
		base = impl.ApplyCast(base, u.Cast.Type, scope)
	}

	return base
}

// ApplyCast applies a type cast to a value
// For int-to-pointer: uses inttoptr
// For pointer-to-int: uses ptrtoint
// For same-size int types: uses bitcast or trunc/zext as needed
func (impl *LLVMBackendImplementation) ApplyCast(val value.Value, targetType *tokens.TypeRef, scope *ast.Ast) value.Value {
	info := LLVMGetScopeInformation(scope)
	srcType := val.Type()
	dstType := impl.TypeRefGetLLIRType(targetType, scope)

	if dstType == nil {
		scope.ErrorScope.NewCompileTimeError("Cast Error", "unable to resolve target type for cast", targetType.Pos)
		return val
	}

	// Check if source is a pointer type
	_, srcIsPtr := srcType.(*types.PointerType)
	// Check if destination is a pointer type
	_, dstIsPtr := dstType.(*types.PointerType)

	// Pointer to integer cast
	if srcIsPtr && !dstIsPtr {
		if intType, ok := dstType.(*types.IntType); ok {
			return info.LocalContext.MainBlock.NewPtrToInt(val, intType)
		}
		scope.ErrorScope.NewCompileTimeError("Cast Error", "cannot cast pointer to non-integer type", targetType.Pos)
		return val
	}

	// Integer to pointer cast
	if !srcIsPtr && dstIsPtr {
		if _, ok := srcType.(*types.IntType); ok {
			return info.LocalContext.MainBlock.NewIntToPtr(val, dstType)
		}
		scope.ErrorScope.NewCompileTimeError("Cast Error", "cannot cast non-integer to pointer type", targetType.Pos)
		return val
	}

	// Integer to integer cast (truncation or extension)
	if srcIntType, srcOk := srcType.(*types.IntType); srcOk {
		if dstIntType, dstOk := dstType.(*types.IntType); dstOk {
			if srcIntType.BitSize > dstIntType.BitSize {
				// Truncate
				return info.LocalContext.MainBlock.NewTrunc(val, dstIntType)
			} else if srcIntType.BitSize < dstIntType.BitSize {
				// Zero extend (for unsigned) - TODO: handle signed extension
				return info.LocalContext.MainBlock.NewZExt(val, dstIntType)
			}
			// Same size, no cast needed
			return val
		}
	}

	// Pointer to pointer cast
	if srcIsPtr && dstIsPtr {
		return info.LocalContext.MainBlock.NewBitCast(val, dstType)
	}

	// Fallback: try bitcast for same-size types
	return info.LocalContext.MainBlock.NewBitCast(val, dstType)
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
		// Parse the number, handling hex (0x...) and decimal
		numStr := p.Literal.Number
		var conv int64
		var err error

		if len(numStr) > 2 && (numStr[0:2] == "0x" || numStr[0:2] == "0X") {
			conv, err = strconv.ParseInt(numStr[2:], 16, 64)
		} else {
			conv, err = strconv.ParseInt(numStr, 10, 64)
		}

		if err != nil {
			scope.ErrorScope.NewCompileTimeError("Number Parse Error", "unable to parse number '"+numStr+"': "+err.Error(), p.Literal.Pos)
			base = constant.NewInt(types.I64, 0)
		} else if p.Literal.IsPointer {
			base = constant.NewInt(types.I1, 0)
			scope.ErrorScope.NewCompileTimeError("Invalid Pointer", "cannot take a pointer of the raw value '"+numStr+"'", p.Literal.Pos)
		} else {
			// Determine the correct integer type based on expressionType
			intType := impl.TypeRefGetLLIRType(expressionType, scope)
			if intType == nil {
				intType = types.I64 // default to i64
			}
			if iType, ok := intType.(*types.IntType); ok {
				if iType.BitSize == 1 {
					// Don't use i1 for integer literals (1 != true in bitwise contexts)
					base = constant.NewInt(types.I64, conv)
				} else {
					base = constant.NewInt(iType, conv)
				}
			} else {
				base = constant.NewInt(types.I64, conv)
			}
		}
	} else if p.Literal.HasStringLiteral() {
		actual, err := p.Literal.StringLiteralValue()

		if err != nil {
			scope.ErrorScope.NewCompileTimeError("String Escape", "unable to escape the string provided "+err.Error(), p.Literal.Pos)
			actual = ""
		}

		str := constant.NewCharArrayFromString(actual + string('\x00'))
		// Gecko string literals are pointer-oriented (C-string style) in call/assignment contexts.
		// Keep the previous array form only when an explicit array type is expected.
		lowerAsPointer := true
		if expressionType != nil && expressionType.Array != nil {
			lowerAsPointer = false
		}
		if p.Literal.IsPointer {
			lowerAsPointer = true
		}

		if lowerAsPointer {
			def := info.ProgramContext.Module.NewGlobalDef(".str."+p.GetID(), str)
			def.Linkage = enum.LinkagePrivate

			if expressionType != nil && expressionType.Const {
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

			// info.LocalContext.MainBlock.NewStore(v, mem)

			info.LocalContext.MainBlock.NewInsertValue(memDirect, v, uint64(i))
		}

		base = mem
	} else if p.Literal.ArrayIndex != nil {
		indexExpr := p.Literal.ArrayIndex

		var val value.Value
		if p.Literal.Symbol != "" {
			// Resolve symbol chain as a pointer (don't load final value)
			val = impl.ResolveSymbolChainValue(scope, p.Literal.Symbol, p.Literal.Chain, p.Pos, true)
		} else {
			p.Literal.ArrayIndex = nil
			val = impl.PrimaryToLLIRValue(p, scope, expressionType)
		}

		arrayIndex := impl.ExpressionToLLIRValue(indexExpr, scope, &tokens.TypeRef{Type: "uint64"})

		if val == nil {
			scope.ErrorScope.NewCompileTimeError("Parse Error", "unable to parse the expression", p.Literal.Pos)
		} else {
			ptrType, isPtr := val.Type().(*types.PointerType)
			if !isPtr {
				scope.ErrorScope.NewCompileTimeError("Index Error", "cannot index a non-pointer type", p.Literal.Pos)
				return nil
			}

			elemType := ptrType.ElemType

			var elemPtr value.Value
			var loadedType types.Type
			// If the element type is an array (fixed-size array field),
			// GEP needs two indices: 0 + arrayIndex
			if innerPtrType, isPtrType := elemType.(*types.PointerType); isPtrType {
				// The base stores a pointer (e.g., T**, or a field containing T*).
				// Load the actual pointer value first, then GEP on the loaded pointer.
				isVolatile := expressionType != nil && expressionType.IsVolatile()
				loadedPtr := impl.NewVolatileLoad(info.LocalContext.MainBlock, elemType, val, isVolatile)
				if loadedPtr != nil {
					elemPtr = info.LocalContext.MainBlock.NewGetElementPtr(innerPtrType.ElemType, loadedPtr, arrayIndex)
					loadedType = innerPtrType.ElemType
				}
			} else if arrType, isArray := elemType.(*types.ArrayType); isArray {
				zero := constant.NewInt(types.I64, 0)
				elemPtr = info.LocalContext.MainBlock.NewGetElementPtr(elemType, val, zero, arrayIndex)
				loadedType = arrType.ElemType
			} else {
				elemPtr = info.LocalContext.MainBlock.NewGetElementPtr(elemType, val, arrayIndex)
				loadedType = elemType
			}

			// Use volatile load if the expression type is volatile (for MMIO)
			isVolatile := expressionType != nil && expressionType.IsVolatile()
			base = impl.NewVolatileLoad(info.LocalContext.MainBlock, loadedType, elemPtr, isVolatile)
		}

	} else if p.Literal.Symbol != "" {
		base = impl.ResolveSymbolChainValue(scope, p.Literal.Symbol, p.Literal.Chain, p.Pos, p.Literal.IsPointer)
	} else if p.Literal.Intrinsic != nil {
		base = impl.IntrinsicToLLIRValue(scope, p.Literal.Intrinsic, expressionType)
	} else if p.Literal.FuncCall != nil {
		// base = p.FuncCall.(scope)
		CurrentBackend.GetImpls().FuncCall(scope, p.Literal.FuncCall)

		call := FuncCalls[llvmFuncCallCacheKey(scope, p.Literal.FuncCall)]

		if call != nil {
			base = call
		}
	} else if p.Literal.IsStructLiteral() {
		// Handle struct literal: TypeName { field: value, ... }
		base = impl.StructLiteralToLLIRValue(p.Literal, scope, expressionType)
	}

	return base
}

// StructLiteralToLLIRValue converts a struct literal to an LLVM value.
func (impl *LLVMBackendImplementation) StructLiteralToLLIRValue(l *tokens.Literal, scope *ast.Ast, expectedType *tokens.TypeRef) value.Value {
	info := LLVMGetScopeInformation(scope)

	// Look up struct info from the global map
	structInfo, ok := LLVMStructMap[l.StructType]
	if !ok {
		scope.ErrorScope.NewCompileTimeError("Type Error", "Unable to resolve struct type '"+l.StructType+"'", l.Pos)
		return nil
	}

	structType := structInfo.Type

	// Create field index map
	fieldIndexMap := make(map[string]int)
	for i, name := range structInfo.FieldNames {
		fieldIndexMap[name] = i
	}

	// Allocate space for the struct
	structPtr := info.LocalContext.MainBlock.NewAlloca(structType)

	// Initialize each field from the struct literal
	for _, kv := range l.StructFields {
		fieldIdx, ok := fieldIndexMap[kv.Key]
		if !ok {
			scope.ErrorScope.NewCompileTimeError("Field Error", "Unknown field '"+kv.Key+"' in struct '"+l.StructType+"'", kv.Pos)
			continue
		}

		// Get the field type for the expression evaluation
		var fieldType *tokens.TypeRef
		if fieldIdx < len(structInfo.FieldTypes) {
			fieldType = structInfo.FieldTypes[fieldIdx]
		} else {
			fieldType = &tokens.TypeRef{Type: "int"}
		}

		// Evaluate the field value expression
		fieldValue := impl.ExpressionToLLIRValue(kv.Value, scope, fieldType)
		if fieldValue == nil {
			scope.ErrorScope.NewCompileTimeError("Expression Error", "Unable to evaluate value for field '"+kv.Key+"'", kv.Pos)
			continue
		}

		// Get pointer to the field using GEP
		zero := constant.NewInt(types.I32, 0)
		fieldIdxConst := constant.NewInt(types.I32, int64(fieldIdx))
		fieldPtr := info.LocalContext.MainBlock.NewGetElementPtr(structType, structPtr, zero, fieldIdxConst)

		// Store the value
		store := impl.NewVolatileStore(info.LocalContext.MainBlock, fieldValue, fieldPtr, false)
		if store == nil {
			scope.ErrorScope.NewCompileTimeError("Type Error", "Unable to assign field '"+kv.Key+"' in struct literal due to incompatible type", kv.Pos)
		}
	}

	// If a by-value struct is expected (e.g., return/assignment to struct type),
	// return the loaded value so downstream coercion/parity matches C backend behavior.
	if expectedType != nil && !expectedType.Pointer {
		return impl.NewVolatileLoad(info.LocalContext.MainBlock, structType, structPtr, false)
	}

	// Default to pointer form for callers that expect address semantics.
	return structPtr
}
