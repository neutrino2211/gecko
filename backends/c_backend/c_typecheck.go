package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// MethodSignature stores parameter and return type information for type checking
type MethodSignature struct {
	Name       string
	Parameters []*tokens.TypeRef
	ParamNames []string
	ReturnType *tokens.TypeRef
	IsGeneric  bool // True if this is a generic function with type parameters
}

// MethodSignatures stores all method signatures for type checking
var MethodSignatures = make(map[string]*MethodSignature)

// RegisterMethodSignature stores a method's signature for later type checking
func RegisterMethodSignature(fullName string, m *tokens.Method) {
	sig := &MethodSignature{
		Name:       m.Name,
		Parameters: make([]*tokens.TypeRef, 0),
		ParamNames: make([]string, 0),
		ReturnType: m.Type,
		IsGeneric:  len(m.TypeParams) > 0,
	}

	for _, arg := range m.Arguments {
		sig.Parameters = append(sig.Parameters, arg.Type)
		sig.ParamNames = append(sig.ParamNames, arg.Name)
	}

	MethodSignatures[fullName] = sig
}

// TypesAreCompatible checks if two types are compatible for assignment/argument passing
func TypesAreCompatible(expected, actual *tokens.TypeRef, scope *ast.Ast) bool {
	if expected == nil || actual == nil {
		return true // Can't check if types are unknown
	}

	// Normalize types to handle Gecko vs C type names
	expectedNorm := normalizeTypeName(expected.Type)
	actualNorm := normalizeTypeName(actual.Type)

	// Check for exact match after normalization
	if expectedNorm == actualNorm && expected.Pointer == actual.Pointer {
		return true
	}

	// Check for numeric compatibility (int/uint sizes)
	if isNumericType(expected) && isNumericType(actual) {
		// Allow numeric conversions for now (could be stricter)
		return true
	}

	// Check pointer compatibility
	if expected.Pointer && actual.Pointer {
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

// normalizeTypeName converts C type names back to Gecko type names for comparison
func normalizeTypeName(t string) string {
	// Map C types to Gecko types
	cToGecko := map[string]string{
		"int64_t":      "int",
		"int32_t":      "int32",
		"int16_t":      "int16",
		"int8_t":       "int8",
		"uint64_t":     "uint",
		"uint32_t":     "uint32",
		"uint16_t":     "uint16",
		"uint8_t":      "uint8",
		"double":       "float64",
		"float":        "float32",
		"const char*":  "string",
		"int":          "int", // C's int maps to Gecko's int32, but keep as-is
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

// FormatTypeRef formats a TypeRef for error messages
func FormatTypeRef(t *tokens.TypeRef) string {
	if t == nil {
		return "unknown"
	}

	result := t.Type

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

// CheckFunctionCallTypes checks if function call arguments match parameter types
func (impl *CBackendImplementation) CheckFunctionCallTypes(f *tokens.FuncCall, scope *ast.Ast) {
	if f == nil || scope.ErrorScope == nil {
		return
	}

	// Determine the function name to look up
	var funcKey string
	var sig *MethodSignature
	var ok bool

	if f.StaticType != "" {
		// Static method call: Type::method()
		funcKey = scope.GetRoot().GetFullName() + "__" + f.StaticType + "__" + f.Function
		sig, ok = MethodSignatures[funcKey]
	} else if f.Module != "" {
		// Module.function() call
		funcKey = f.Module + "__" + f.Function
		sig, ok = MethodSignatures[funcKey]
	} else {
		// Regular function call - try multiple lookup strategies
		// 1. Try resolving through scope hierarchy
		methodOpt := scope.ResolveMethod(f.Function)
		if !methodOpt.IsNil() {
			method := methodOpt.Unwrap()
			funcKey = method.GetFullName()
			sig, ok = MethodSignatures[funcKey]
		}

		// 2. Try root scope prefix
		if !ok {
			funcKey = scope.GetRoot().GetFullName() + "__" + f.Function
			sig, ok = MethodSignatures[funcKey]
		}

		// 3. Try just the function name (for external functions)
		if !ok {
			sig, ok = MethodSignatures[f.Function]
		}

		// 4. Try parent scope prefix (for functions in same file)
		if !ok {
			parent := scope
			for parent != nil {
				funcKey = parent.GetFullName() + "__" + f.Function
				sig, ok = MethodSignatures[funcKey]
				if ok {
					break
				}
				parent = parent.Parent
			}
		}
	}

	if !ok {
		return // Can't check - method signature not found
	}

	// Skip type checking for generic functions - proper generic type checking
	// requires a full type declaration system (TODO: implement in future)
	if sig.IsGeneric {
		return
	}

	// Check argument count
	if len(f.Arguments) != len(sig.Parameters) {
		scope.ErrorScope.NewCompileTimeError(
			"Argument Count Mismatch",
			fmt.Sprintf("Function '%s' expects %d arguments, got %d",
				f.Function, len(sig.Parameters), len(f.Arguments)),
			f.Pos,
		)
		return
	}

	// Check each argument type
	for i, arg := range f.Arguments {
		if i >= len(sig.Parameters) {
			break
		}

		expectedType := sig.Parameters[i]
		if expectedType == nil {
			continue
		}

		actualType := impl.GetTypeOfExpression(arg.Value, scope)
		if actualType == nil {
			continue
		}

		if !TypesAreCompatible(expectedType, actualType, scope) {
			paramName := ""
			if i < len(sig.ParamNames) {
				paramName = sig.ParamNames[i]
			}
			scope.ErrorScope.NewCompileTimeError(
				"Type Mismatch",
				"Argument "+(paramName)+" expects type '"+FormatTypeRef(expectedType)+
					"', got '"+FormatTypeRef(actualType)+"'",
				f.Pos,
			)
		}
	}
}

// CheckAssignmentType checks if assignment value matches variable type
func (impl *CBackendImplementation) CheckAssignmentType(a *tokens.Assignment, scope *ast.Ast) {
	if a == nil || scope.ErrorScope == nil {
		return
	}

	// Skip field/index assignments for now - they require more complex type checking
	// e.g., rect.width = 10 needs to check field type, not variable type
	if a.Field != "" || a.Index != nil {
		// TODO: Implement field/index assignment type checking
		return
	}

	// Look up variable type
	varOpt := scope.ResolveSymbolAsVariable(a.Name)
	if varOpt.IsNil() {
		return
	}

	variable := varOpt.Unwrap()

	// Check if it's a const being reassigned
	if variable.IsConst {
		scope.ErrorScope.NewCompileTimeError(
			"Cannot Reassign Constant",
			"Cannot reassign constant '"+a.Name+"'",
			a.Pos,
		)
		return
	}

	// Get the variable's type from CProgramValues
	fullName := variable.GetFullName()
	valueInfo, hasInfo := (*CProgramValues)[fullName]
	if !hasInfo || valueInfo.GeckoType == nil {
		return // Can't check type
	}

	expectedType := valueInfo.GeckoType

	// Get the expression type
	actualType := impl.GetTypeOfExpression(a.Value, scope)
	if actualType == nil {
		return
	}

	if !TypesAreCompatible(expectedType, actualType, scope) {
		scope.ErrorScope.NewCompileTimeError(
			"Type Mismatch",
			"Cannot assign '"+FormatTypeRef(actualType)+"' to variable '"+
				a.Name+"' of type '"+FormatTypeRef(expectedType)+"'",
			a.Pos,
		)
	}
}
