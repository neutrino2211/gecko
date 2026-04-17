package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// TypePattern represents a pattern for matching types
type TypePattern int

const (
	PatternPointer    TypePattern = iota // T* - any pointer
	PatternNonNull                       // T*! - non-null pointer
	PatternArray                         // [N]T - fixed array
	PatternPrimitive                     // int, uint, etc.
)

// GetTypePattern determines the pattern for a given type
func GetTypePattern(t *tokens.TypeRef) TypePattern {
	if t == nil {
		return -1
	}
	if t.Pointer {
		if t.NonNull {
			return PatternNonNull
		}
		return PatternPointer
	}
	if t.Array != nil {
		return PatternArray
	}
	if tokens.IsPrimitive(t.Type) {
		return PatternPrimitive
	}
	return -1
}

// TryBuiltinMethod attempts to resolve a method call as a built-in trait method
// Returns the generated C code and true if successful, empty string and false otherwise
func (impl *CBackendImplementation) TryBuiltinMethod(
	receiver string,
	receiverType *tokens.TypeRef,
	methodName string,
	args []*tokens.Argument,
	scope *ast.Ast,
) (string, bool) {
	pattern := GetTypePattern(receiverType)
	if pattern < 0 {
		return "", false
	}

	switch pattern {
	case PatternPointer:
		return impl.tryPointerMethod(receiver, receiverType, methodName, args, scope)
	case PatternNonNull:
		return impl.tryNonNullMethod(receiver, receiverType, methodName, args, scope)
	}

	return "", false
}

// tryPointerMethod handles builtin methods for T* types
func (impl *CBackendImplementation) tryPointerMethod(
	receiver string,
	receiverType *tokens.TypeRef,
	methodName string,
	args []*tokens.Argument,
	scope *ast.Ast,
) (string, bool) {
	switch methodName {
	case "deref":
		return fmt.Sprintf("(*(%s))", receiver), true
	case "is_null":
		return fmt.Sprintf("(%s == (void*)0)", receiver), true
	case "offset":
		if len(args) != 1 {
			return receiver, true
		}
		offset := impl.ExpressionToCString(args[0].Value, scope)
		return fmt.Sprintf("((%s) + (%s))", receiver, offset), true
	}
	return "", false
}

// tryNonNullMethod handles builtin methods for T*! types
func (impl *CBackendImplementation) tryNonNullMethod(
	receiver string,
	receiverType *tokens.TypeRef,
	methodName string,
	args []*tokens.Argument,
	scope *ast.Ast,
) (string, bool) {
	switch methodName {
	case "value":
		return fmt.Sprintf("(*(%s))", receiver), true
	case "deref":
		return fmt.Sprintf("(*(%s))", receiver), true
	case "is_null":
		return "0", true // Non-null pointers are never null
	case "offset":
		if len(args) != 1 {
			return receiver, true
		}
		offset := impl.ExpressionToCString(args[0].Value, scope)
		return fmt.Sprintf("((%s) + (%s))", receiver, offset), true
	}
	return "", false
}

// HasBuiltinTrait checks if a type pattern implements a given trait
func HasBuiltinTrait(t *tokens.TypeRef, traitName string) bool {
	pattern := GetTypePattern(t)
	if pattern < 0 {
		return false
	}

	traitLower := strings.ToLower(traitName)

	switch pattern {
	case PatternPointer:
		return traitLower == "pointer"
	case PatternNonNull:
		return traitLower == "pointer" || traitLower == "nonnullable"
	}

	return false
}
