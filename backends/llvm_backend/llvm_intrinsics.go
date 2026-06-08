// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

func (impl *LLVMBackendImplementation) IntrinsicToLLIRValue(scope *ast.Ast, i *tokens.Intrinsic, expressionType *tokens.TypeRef) value.Value {
	if i == nil || scope == nil {
		return nil
	}

	info := LLVMGetScopeInformation(scope)
	if info == nil || info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "intrinsic call must be inside a function body", i.Pos)
		return nil
	}

	name := strings.TrimSpace(i.Name)
	switch name {
	case "is_null", "is_not_null":
		if len(i.Args) != 1 {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@"+name+" expects exactly one argument", i.Pos)
			return nil
		}
		ptrValue := impl.ExpressionToLLIRValue(i.Args[0], scope, &tokens.TypeRef{Type: "int8", Pointer: true})
		if ptrValue == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@"+name+" could not evaluate pointer operand", i.Pos)
			return nil
		}

		ptrType, ok := ptrValue.Type().(*types.PointerType)
		if !ok {
			if _, isInt := ptrValue.Type().(*types.IntType); isInt {
				ptrType = types.I8Ptr
				ptrValue = info.LocalContext.MainBlock.NewIntToPtr(ptrValue, ptrType)
			} else {
				scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@"+name+" requires a pointer-like operand", i.Pos)
				return nil
			}
		}

		nullPtr := constant.NewNull(ptrType)
		pred := enum.IPredEQ
		if name == "is_not_null" {
			pred = enum.IPredNE
		}
		return info.LocalContext.MainBlock.NewICmp(pred, ptrValue, nullPtr)

	case "deref":
		if len(i.Args) != 1 {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@deref expects exactly one argument", i.Pos)
			return nil
		}
		ptrValue := impl.ExpressionToLLIRValue(i.Args[0], scope, &tokens.TypeRef{Type: "int8", Pointer: true})
		if ptrValue == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@deref could not evaluate pointer operand", i.Pos)
			return nil
		}
		ptrType, ok := ptrValue.Type().(*types.PointerType)
		if !ok || ptrType.ElemType == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@deref requires a pointer operand", i.Pos)
			return nil
		}
		return impl.NewVolatileLoad(info.LocalContext.MainBlock, ptrType.ElemType, ptrValue, false)

	case "write_volatile":
		if len(i.Args) != 2 {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@write_volatile expects exactly two arguments", i.Pos)
			return nil
		}
		ptrValue := impl.ExpressionToLLIRValue(i.Args[0], scope, &tokens.TypeRef{Type: "int8", Pointer: true})
		if ptrValue == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@write_volatile could not evaluate destination pointer", i.Pos)
			return nil
		}
		ptrType, ok := ptrValue.Type().(*types.PointerType)
		if !ok || ptrType.ElemType == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@write_volatile destination must be a pointer", i.Pos)
			return nil
		}

		valueToStore := impl.ExpressionToLLIRValue(i.Args[1], scope, expressionType)
		if valueToStore == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@write_volatile could not evaluate value argument", i.Pos)
			return nil
		}
		valueToStore = impl.coerceValueToType(valueToStore, ptrType.ElemType, scope, i.Pos)
		if valueToStore == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@write_volatile value type does not match destination pointer element type", i.Pos)
			return nil
		}

		if impl.NewVolatileStore(info.LocalContext.MainBlock, valueToStore, ptrValue, true) == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@write_volatile failed due to incompatible pointer/value types", i.Pos)
			return nil
		}
		return valueToStore

	case "size_of", "align_of":
		var llvmType types.Type
		if len(i.TypeArgs) > 0 {
			llvmType = impl.TypeRefGetLLIRType(i.TypeArgs[0], scope)
		} else if len(i.Args) > 0 {
			argValue := impl.ExpressionToLLIRValue(i.Args[0], scope, &tokens.TypeRef{})
			if argValue != nil {
				llvmType = argValue.Type()
			}
		}
		if llvmType == nil {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@"+name+" requires a resolvable type", i.Pos)
			return constant.NewInt(types.I64, 0)
		}

		var n uint64
		if name == "size_of" {
			n = llvmApproxSizeOfType(llvmType)
		} else {
			n = llvmApproxAlignOfType(llvmType)
		}
		if n == 0 {
			n = 1
		}
		return constant.NewInt(types.I64, int64(n))

	case "set_try_error_handler":
		// Runtime diagnostic hook used by stdlib/errors.
		// LLVM backend currently treats this as a no-op initializer hook.
		return constant.NewInt(types.I1, 0)
	}

	scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "unsupported LLVM intrinsic '@"+name+"'", i.Pos)
	return nil
}

func llvmApproxAlignOfType(t types.Type) uint64 {
	switch tt := t.(type) {
	case *types.IntType:
		bytes := tt.BitSize / 8
		if bytes == 0 {
			return 1
		}
		return uint64(bytes)
	case *types.PointerType:
		return 8
	case *types.ArrayType:
		return llvmApproxAlignOfType(tt.ElemType)
	case *types.StructType:
		if tt.Packed {
			return 1
		}
		maxAlign := uint64(1)
		for _, field := range tt.Fields {
			align := llvmApproxAlignOfType(field)
			if align > maxAlign {
				maxAlign = align
			}
		}
		return maxAlign
	default:
		return 8
	}
}

func llvmApproxSizeOfType(t types.Type) uint64 {
	switch tt := t.(type) {
	case *types.IntType:
		bytes := tt.BitSize / 8
		if bytes == 0 {
			return 1
		}
		return uint64(bytes)
	case *types.PointerType:
		return 8
	case *types.ArrayType:
		return tt.Len * llvmApproxSizeOfType(tt.ElemType)
	case *types.StructType:
		if tt.Opaque {
			return 8
		}
		if tt.Packed {
			var total uint64
			for _, field := range tt.Fields {
				total += llvmApproxSizeOfType(field)
			}
			return total
		}

		var total uint64
		maxAlign := uint64(1)
		for _, field := range tt.Fields {
			align := llvmApproxAlignOfType(field)
			size := llvmApproxSizeOfType(field)
			if align > maxAlign {
				maxAlign = align
			}
			if align > 0 && total%align != 0 {
				total += align - (total % align)
			}
			total += size
		}
		if maxAlign > 0 && total%maxAlign != 0 {
			total += maxAlign - (total % maxAlign)
		}
		return total
	default:
		return 8
	}
}

func llvmResolveStructInfoByType(t types.Type) *LLVMStructInfo {
	structType, ok := t.(*types.StructType)
	if !ok || structType == nil {
		return nil
	}

	if structType.Name() != "" {
		if info, ok := LLVMStructMap[structType.Name()]; ok && info != nil {
			return info
		}
	}

	for _, info := range LLVMStructMap {
		if info != nil && info.Type != nil && info.Type.Equal(structType) {
			return info
		}
	}
	return nil
}

func llvmFieldIndex(info *LLVMStructInfo, names ...string) int {
	if info == nil {
		return -1
	}
	for _, candidate := range names {
		for idx, fieldName := range info.FieldNames {
			if fieldName == candidate {
				return idx
			}
		}
	}
	return -1
}

func (impl *LLVMBackendImplementation) lowerTryUnary(scope *ast.Ast, operand value.Value, expectedType *tokens.TypeRef, pos lexer.Position) value.Value {
	if scope == nil || operand == nil {
		return nil
	}

	info := LLVMGetScopeInformation(scope)
	if info == nil || info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Try Expression Error", "'try' can only be used inside a function", pos)
		return nil
	}
	block := info.LocalContext.MainBlock

	var structPtr value.Value
	var structType types.Type

	switch t := operand.Type().(type) {
	case *types.StructType:
		tmp := block.NewAlloca(t)
		if impl.NewVolatileStore(block, operand, tmp, false) == nil {
			scope.ErrorScope.NewCompileTimeError("Try Expression Error", "unable to materialize try operand", pos)
			return nil
		}
		structPtr = tmp
		structType = t
	case *types.PointerType:
		if _, ok := t.ElemType.(*types.StructType); ok {
			structPtr = operand
			structType = t.ElemType
		} else if nestedPtr, ok := t.ElemType.(*types.PointerType); ok {
			if _, ok := nestedPtr.ElemType.(*types.StructType); ok {
				structPtr = impl.NewVolatileLoad(block, nestedPtr, operand, false)
				structType = nestedPtr.ElemType
			}
		}
	}

	if structPtr == nil || structType == nil {
		scope.ErrorScope.NewCompileTimeError("Try Expression Error", "'try' requires a struct-like Tryable operand", pos)
		return nil
	}

	structInfo := llvmResolveStructInfoByType(structType)
	if structInfo == nil {
		scope.ErrorScope.NewCompileTimeError("Try Expression Error", "unable to resolve Tryable payload layout for try operand", pos)
		return nil
	}

	flagIdx := llvmFieldIndex(structInfo, "has_value", "is_ok")
	valueIdx := llvmFieldIndex(structInfo, "value")
	if flagIdx < 0 || valueIdx < 0 {
		scope.ErrorScope.NewCompileTimeError("Try Expression Error", "try operand does not expose expected Tryable fields", pos)
		return nil
	}

	_ = flagIdx // compile-time validation only for now; runtime check wiring will follow.

	zero := constant.NewInt(types.I32, 0)
	valueIdxConst := constant.NewInt(types.I32, int64(valueIdx))
	valuePtr := block.NewGetElementPtr(structInfo.Type, structPtr, zero, valueIdxConst)

	valueType := types.Type(nil)
	if ptrType, ok := valuePtr.Type().(*types.PointerType); ok {
		valueType = ptrType.ElemType
	}
	if valueType == nil && valueIdx < len(structInfo.FieldTypes) && structInfo.FieldTypes[valueIdx] != nil {
		valueType = impl.TypeRefGetLLIRType(structInfo.FieldTypes[valueIdx], scope)
	}
	if valueType == nil {
		scope.ErrorScope.NewCompileTimeError("Try Expression Error", "unable to resolve payload type for try operand", pos)
		return nil
	}

	var loaded value.Value = impl.NewVolatileLoad(block, valueType, valuePtr, false)
	if expectedType != nil {
		if expectedLL := impl.TypeRefGetLLIRType(expectedType, scope); expectedLL != nil {
			loaded = impl.coerceValueToType(loaded, expectedLL, scope, pos)
		}
	}
	return loaded
}
