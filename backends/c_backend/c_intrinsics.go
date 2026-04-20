package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// IntrinsicStatement handles intrinsic calls as statements (e.g., @write_volatile)
func (impl *CBackendImplementation) IntrinsicStatement(scope *ast.Ast, i *tokens.Intrinsic) {
	info := CGetScopeInformation(scope)
	code := impl.IntrinsicToCString(i, scope)
	info.Code += "    " + code + ";\n"
}

// IntrinsicToCString converts an intrinsic call to C code
func (impl *CBackendImplementation) IntrinsicToCString(i *tokens.Intrinsic, scope *ast.Ast) string {
	switch i.Name {
	case "deref":
		return impl.intrinsicDeref(i, scope)
	case "is_null":
		return impl.intrinsicIsNull(i, scope)
	case "is_not_null":
		return impl.intrinsicIsNotNull(i, scope)
	case "size_of":
		return impl.intrinsicSizeOf(i, scope)
	case "align_of":
		return impl.intrinsicAlignOf(i, scope)
	case "ptr_add":
		return impl.intrinsicPtrAdd(i, scope)
	case "ptr_sub":
		return impl.intrinsicPtrSub(i, scope)
	case "copy":
		return impl.intrinsicCopy(i, scope)
	case "zero":
		return impl.intrinsicZero(i, scope)
	case "read_volatile":
		return impl.intrinsicReadVolatile(i, scope)
	case "write_volatile":
		return impl.intrinsicWriteVolatile(i, scope)
	case "unreachable":
		return impl.intrinsicUnreachable(i, scope)
	case "trap":
		return impl.intrinsicTrap(i, scope)
	// Builtin operators for primitive types
	case "builtin_add":
		return impl.intrinsicBuiltinOp(i, scope, "+")
	case "builtin_sub":
		return impl.intrinsicBuiltinOp(i, scope, "-")
	case "builtin_mul":
		return impl.intrinsicBuiltinOp(i, scope, "*")
	case "builtin_div":
		return impl.intrinsicBuiltinOp(i, scope, "/")
	case "builtin_eq":
		return impl.intrinsicBuiltinOp(i, scope, "==")
	case "builtin_ne":
		return impl.intrinsicBuiltinOp(i, scope, "!=")
	case "builtin_lt":
		return impl.intrinsicBuiltinOp(i, scope, "<")
	case "builtin_gt":
		return impl.intrinsicBuiltinOp(i, scope, ">")
	case "builtin_le":
		return impl.intrinsicBuiltinOp(i, scope, "<=")
	case "builtin_ge":
		return impl.intrinsicBuiltinOp(i, scope, ">=")
	case "builtin_bitand":
		return impl.intrinsicBuiltinOp(i, scope, "&")
	case "builtin_bitor":
		return impl.intrinsicBuiltinOp(i, scope, "|")
	case "builtin_bitxor":
		return impl.intrinsicBuiltinOp(i, scope, "^")
	case "builtin_shl":
		return impl.intrinsicBuiltinOp(i, scope, "<<")
	case "builtin_shr":
		return impl.intrinsicBuiltinOp(i, scope, ">>")
	case "builtin_neg":
		return impl.intrinsicBuiltinUnary(i, scope, "-")
	case "builtin_not":
		return impl.intrinsicBuiltinUnary(i, scope, "!")
	default:
		scope.ErrorScope.NewCompileTimeError(
			"Unknown Intrinsic",
			fmt.Sprintf("Unknown intrinsic '@%s'", i.Name),
			i.Pos,
		)
		return "/* unknown intrinsic */"
	}
}

// @deref(ptr) - dereference a pointer
func (impl *CBackendImplementation) intrinsicDeref(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@deref requires exactly 1 argument", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	return fmt.Sprintf("(*(%s))", ptr)
}

// @is_null(ptr) - check if pointer is null
func (impl *CBackendImplementation) intrinsicIsNull(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@is_null requires exactly 1 argument", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	return fmt.Sprintf("(%s == NULL)", ptr)
}

// @is_not_null(ptr) - check if pointer is not null (enables type narrowing)
func (impl *CBackendImplementation) intrinsicIsNotNull(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@is_not_null requires exactly 1 argument", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	return fmt.Sprintf("(%s != NULL)", ptr)
}

// @size_of<T>() - size of type in bytes
func (impl *CBackendImplementation) intrinsicSizeOf(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.TypeArgs) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@size_of requires exactly 1 type argument", i.Pos)
		return "0"
	}
	cType := TypeRefToCType(i.TypeArgs[0], scope)
	return fmt.Sprintf("sizeof(%s)", cType)
}

// @align_of<T>() - alignment of type
func (impl *CBackendImplementation) intrinsicAlignOf(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.TypeArgs) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@align_of requires exactly 1 type argument", i.Pos)
		return "0"
	}
	cType := TypeRefToCType(i.TypeArgs[0], scope)
	return fmt.Sprintf("_Alignof(%s)", cType)
}

// @ptr_add(ptr, offset) - pointer arithmetic (add)
func (impl *CBackendImplementation) intrinsicPtrAdd(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 2 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@ptr_add requires exactly 2 arguments", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	offset := impl.ExpressionToCString(i.Args[1], scope)
	return fmt.Sprintf("((%s) + (%s))", ptr, offset)
}

// @ptr_sub(ptr, offset) - pointer arithmetic (subtract)
func (impl *CBackendImplementation) intrinsicPtrSub(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 2 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@ptr_sub requires exactly 2 arguments", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	offset := impl.ExpressionToCString(i.Args[1], scope)
	return fmt.Sprintf("((%s) - (%s))", ptr, offset)
}

// @copy(dest, src, size) - memory copy
func (impl *CBackendImplementation) intrinsicCopy(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 3 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@copy requires exactly 3 arguments (dest, src, size)", i.Pos)
		return "0"
	}
	dest := impl.ExpressionToCString(i.Args[0], scope)
	src := impl.ExpressionToCString(i.Args[1], scope)
	size := impl.ExpressionToCString(i.Args[2], scope)
	return fmt.Sprintf("__builtin_memcpy(%s, %s, %s)", dest, src, size)
}

// @zero(ptr, size) - zero memory
func (impl *CBackendImplementation) intrinsicZero(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 2 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@zero requires exactly 2 arguments (ptr, size)", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	size := impl.ExpressionToCString(i.Args[1], scope)
	return fmt.Sprintf("__builtin_memset(%s, 0, %s)", ptr, size)
}

// @read_volatile(ptr) - volatile read
func (impl *CBackendImplementation) intrinsicReadVolatile(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@read_volatile requires exactly 1 argument", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	return fmt.Sprintf("(*(volatile typeof(*%s)*)(%s))", ptr, ptr)
}

// @write_volatile(ptr, value) - volatile write (returns void, use as statement)
func (impl *CBackendImplementation) intrinsicWriteVolatile(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 2 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@write_volatile requires exactly 2 arguments (ptr, value)", i.Pos)
		return "0"
	}
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	value := impl.ExpressionToCString(i.Args[1], scope)
	return fmt.Sprintf("(*(volatile typeof(*%s)*)(%s) = (%s))", ptr, ptr, value)
}

// @unreachable() - mark code as unreachable
func (impl *CBackendImplementation) intrinsicUnreachable(i *tokens.Intrinsic, scope *ast.Ast) string {
	return "__builtin_unreachable()"
}

// @trap() - trigger a trap/abort
func (impl *CBackendImplementation) intrinsicTrap(i *tokens.Intrinsic, scope *ast.Ast) string {
	return "__builtin_trap()"
}

// @builtin_add, @builtin_sub, etc. - primitive binary operators
func (impl *CBackendImplementation) intrinsicBuiltinOp(i *tokens.Intrinsic, scope *ast.Ast, op string) string {
	if len(i.Args) != 2 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", fmt.Sprintf("@builtin_%s requires exactly 2 arguments", i.Name), i.Pos)
		return "0"
	}
	left := impl.ExpressionToCString(i.Args[0], scope)
	right := impl.ExpressionToCString(i.Args[1], scope)
	return fmt.Sprintf("((%s) %s (%s))", left, op, right)
}

// @builtin_neg, @builtin_not - primitive unary operators
func (impl *CBackendImplementation) intrinsicBuiltinUnary(i *tokens.Intrinsic, scope *ast.Ast, op string) string {
	if len(i.Args) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", fmt.Sprintf("@builtin_%s requires exactly 1 argument", i.Name), i.Pos)
		return "0"
	}
	operand := impl.ExpressionToCString(i.Args[0], scope)
	return fmt.Sprintf("(%s(%s))", op, operand)
}
