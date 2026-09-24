// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// MethodSignature stores parameter and return type information for type checking
type MethodSignature struct {
	Name       string
	Parameters []*tokens.TypeRef
	ParamNames []string
	ParamOut   []bool
	Variadic   bool
	ReturnType *tokens.TypeRef
	Throws     *tokens.TypeRef     // Error type that this function can throw
	IsGeneric  bool                // True if this is a generic function with type parameters
	TypeParams []*tokens.TypeParam // Full type parameters with constraints (e.g., T is Area)
}

// MethodSignatures stores all method signatures for type checking
var MethodSignatures = make(map[string]*MethodSignature)

// TypeCheckError emits an error when type checking cannot be performed
// Type safety gaps are errors, not warnings - code that can't be verified won't compile
func TypeCheckError(scope *ast.Ast, pos lexer.Position, context string, detail string) {
	if scope.ErrorScope == nil {
		return
	}
	scope.ErrorScope.NewCompileTimeError(
		"Type Check Error",
		fmt.Sprintf("Unable to verify type safety for %s: %s", context, detail),
		pos,
	)
}

// RegisterMethodSignature stores a method's signature for later type checking
func RegisterMethodSignature(fullName string, m *tokens.Method) {
	sig := &MethodSignature{
		Name:       m.Name,
		Parameters: make([]*tokens.TypeRef, 0),
		ParamNames: make([]string, 0),
		ParamOut:   make([]bool, 0),
		ReturnType: m.Type,
		Throws:     m.Throws,
		IsGeneric:  len(m.TypeParams) > 0,
		TypeParams: m.TypeParams,
		Variadic:   m.IsVariadic(),
	}

	for _, arg := range m.Arguments {
		sig.Parameters = append(sig.Parameters, arg.Type)
		sig.ParamNames = append(sig.ParamNames, arg.Name)
		sig.ParamOut = append(sig.ParamOut, arg.Out)
	}

	MethodSignatures[fullName] = sig
}

// TypesAreCompatible checks if two types are compatible for assignment/argument passing
func TypesAreCompatible(expected, actual *tokens.TypeRef, scope *ast.Ast) bool {
	if expected == nil || actual == nil {
		return true // Can't check if types are unknown
	}

	// If actual type is empty/unknown, we can't verify - skip check
	if actual.Type == "" {
		return true
	}

	// Resolve Self to the current class type
	if CurrentSelfType != "" {
		if expected.Type == "Self" {
			resolved := *expected
			resolved.Type = CurrentSelfType
			return TypesAreCompatible(&resolved, actual, scope)
		}
		if actual.Type == "Self" {
			resolved := *actual
			resolved.Type = CurrentSelfType
			return TypesAreCompatible(expected, &resolved, scope)
		}
	}

	// If expected is a type parameter (single uppercase letter or known param), skip for now
	// This will be handled properly when we implement generic function type checking
	if isTypeParameter(expected.Type) {
		return true
	}

	// Normalize types to handle Gecko vs C type names
	expectedNorm := normalizeTypeName(expected.Type)
	actualNorm := normalizeTypeName(actual.Type)

	// Check for exact match after normalization
	if expectedNorm == actualNorm && expected.Pointer == actual.Pointer {
		// Nullability check for pointers: nullable pointer cannot flow into non-null pointer.
		if expected.Pointer && expected.NonNull && !actual.NonNull {
			return false
		}
		// Word qualifiers: may add readonly/volatile, must not drop them implicitly.
		if actual.Const && !expected.Const {
			return false
		}
		if actual.Volatile && !expected.Volatile {
			return false
		}
		return true
	}

	// Check for numeric compatibility (int/uint sizes)
	if isNumericType(expected) && isNumericType(actual) {
		// Allow numeric conversions for now (could be stricter)
		return true
	}

	// Check pointer compatibility
	if expected.Pointer && actual.Pointer {
		// Nullability check for pointers: nullable pointer cannot flow into non-null pointer.
		if expected.NonNull && !actual.NonNull {
			return false
		}
		if actual.Const && !expected.Const {
			return false
		}
		if actual.Volatile && !expected.Volatile {
			return false
		}
		// void* is compatible with any pointer
		if expectedNorm == "void" || actualNorm == "void" {
			return true
		}
	}

	// String literals are compatible with string type
	if expectedNorm == "string" && actualNorm == "string" {
		return true
	}

	return false
}

// substituteTypeRef applies type parameter substitution to a TypeRef
// Returns a new TypeRef with type parameters replaced by concrete types
func substituteTypeRef(t *tokens.TypeRef, subst map[string]*tokens.TypeRef) *tokens.TypeRef {
	if t == nil || len(subst) == 0 {
		return t
	}

	// Check if this type itself is a type parameter
	if concrete, ok := subst[t.Type]; ok {
		// Return the concrete type, preserving pointer/const modifiers from original
		result := &tokens.TypeRef{
			Type:     concrete.Type,
			Pointer:  t.Pointer || concrete.Pointer,
			Const:    t.Const || concrete.Const,
			Volatile: t.Volatile || concrete.Volatile,
			NonNull:  t.NonNull || concrete.NonNull,
		}
		return result
	}

	// Recursively substitute in array types
	if t.Array != nil {
		return &tokens.TypeRef{
			Array:    substituteTypeRef(t.Array, subst),
			Pointer:  t.Pointer,
			Const:    t.Const,
			Volatile: t.Volatile,
			NonNull:  t.NonNull,
		}
	}

	// Substitute in type arguments (for generic types like Vec<T>)
	if len(t.TypeArgs) > 0 {
		newArgs := make([]*tokens.TypeRef, len(t.TypeArgs))
		for i, arg := range t.TypeArgs {
			newArgs[i] = substituteTypeRef(arg, subst)
		}
		return &tokens.TypeRef{
			Type:     t.Type,
			TypeArgs: newArgs,
			Pointer:  t.Pointer,
			Const:    t.Const,
			Volatile: t.Volatile,
			NonNull:  t.NonNull,
		}
	}

	return t
}

// isTypeParameter checks if a type name looks like a generic type parameter
// Type parameters are typically single uppercase letters (T, U, V) or short names like T1, T2
func isTypeParameter(typeName string) bool {
	if typeName == "" {
		return false
	}
	// Single uppercase letter
	if len(typeName) == 1 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
		return true
	}
	// Single uppercase letter followed by digits (T1, T2, etc.)
	if len(typeName) >= 2 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
		for i := 1; i < len(typeName); i++ {
			if typeName[i] < '0' || typeName[i] > '9' {
				return false
			}
		}
		return true
	}
	return false
}

// normalizeTypeName converts C type names back to Gecko type names for comparison
func normalizeTypeName(t string) string {
	// Map C types to Gecko types
	cToGecko := map[string]string{
		"int64_t":     "int",
		"int32_t":     "int32",
		"int16_t":     "int16",
		"int8_t":      "int8",
		"uint64_t":    "uint",
		"uint32_t":    "uint32",
		"uint16_t":    "uint16",
		"uint8_t":     "uint8",
		"double":      "float64",
		"float":       "float32",
		"const char*": "string",
		"int":         "int", // C's int maps to Gecko's int32, but keep as-is
	}

	if normalized, ok := cToGecko[t]; ok {
		return normalized
	}
	return t
}

// TypeRefsEqual checks if two TypeRefs represent the same type
func TypeRefsEqual(a, b *tokens.TypeRef) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Check base type
	if a.Type != b.Type {
		return false
	}

	// Check pointer
	if a.Pointer != b.Pointer {
		return false
	}

	// Check non-null pointer modifier
	if a.NonNull != b.NonNull {
		return false
	}

	// Check volatile
	if a.Volatile != b.Volatile {
		return false
	}

	// Check array
	if (a.Array == nil) != (b.Array == nil) {
		return false
	}
	if a.Array != nil && !TypeRefsEqual(a.Array, b.Array) {
		return false
	}

	// Check fixed-size array
	if (a.Size == nil) != (b.Size == nil) {
		return false
	}
	if a.Size != nil {
		if a.Size.Size != b.Size.Size {
			return false
		}
		if !TypeRefsEqual(a.Size.Type, b.Size.Type) {
			return false
		}
	}

	return true
}

// CheckVariableInitType validates that a variable initializer matches the declared type.
func (impl *CBackendImplementation) CheckVariableInitType(f *tokens.Field, scope *ast.Ast) {
	if f == nil || f.Value == nil || f.Type == nil || scope.ErrorScope == nil {
		return
	}

	// For now, enforce initialization checks only for explicit non-null pointer targets.
	// This preserves existing behavior for complex generic/intrinsic inference paths while
	// still preventing nullable-to-nonnull flows at declaration sites.
	expectedType := f.Type
	if !expectedType.Pointer || !expectedType.NonNull {
		return
	}

	actualType := impl.GetTypeOfExpression(f.Value, scope)
	if actualType == nil {
		return
	}

	if !TypesAreCompatible(expectedType, actualType, scope) {
		scope.ErrorScope.NewCompileTimeError(
			"Type Mismatch",
			fmt.Sprintf("Cannot initialize '%s' of type '%s' with '%s'",
				f.Name, FormatTypeRef(expectedType), FormatTypeRef(actualType)),
			f.Pos,
		)
	}
}

func isNumericType(t *tokens.TypeRef) bool {
	if t == nil {
		return false
	}
	normalized := normalizeTypeName(t.Type)
	switch normalized {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return true
	}
	return false
}

// getNumericBitSize returns the bit size of a numeric type, or 0 if not numeric
func getNumericBitSize(typeName string) int {
	normalized := normalizeTypeName(typeName)
	switch normalized {
	case "int8", "uint8":
		return 8
	case "int16", "uint16":
		return 16
	case "int32", "uint32", "float32":
		return 32
	case "int", "uint", "int64", "uint64", "float64":
		return 64
	}
	return 0
}

// isSignedType returns true if the type is a signed numeric type
func isSignedType(typeName string) bool {
	normalized := normalizeTypeName(typeName)
	switch normalized {
	case "int", "int8", "int16", "int32", "int64":
		return true
	}
	return false
}

// IsLossyConversion checks if converting from 'from' to 'to' may lose data.
// Returns a warning message if lossy, empty string if safe.
func IsLossyConversion(from, to *tokens.TypeRef) string {
	if from == nil || to == nil {
		return ""
	}
	if !isNumericType(from) || !isNumericType(to) {
		return ""
	}

	fromBits := getNumericBitSize(from.Type)
	toBits := getNumericBitSize(to.Type)

	// Larger to smaller is always lossy
	if fromBits > toBits {
		return fmt.Sprintf("implicit conversion from '%s' to '%s' may lose data", from.Type, to.Type)
	}

	// Signed to unsigned of same size can lose negative values
	if fromBits == toBits && isSignedType(from.Type) && !isSignedType(to.Type) {
		return fmt.Sprintf("implicit conversion from '%s' to '%s' may lose sign", from.Type, to.Type)
	}

	return ""
}

// FormatTypeRef formats a TypeRef for error messages
func FormatTypeRef(t *tokens.TypeRef) string {
	if t == nil {
		return "unknown"
	}

	result := t.Type

	if t.Const {
		result += " readonly"
	}
	if t.Volatile {
		result += " volatile"
	}
	if t.Pointer {
		result += "*"
	}
	if t.NonNull {
		result += "!"
	}

	return result
}
