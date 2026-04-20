package cbackend

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// MethodSignature stores parameter and return type information for type checking
type MethodSignature struct {
	Name       string
	Parameters []*tokens.TypeRef
	ParamNames []string
	ReturnType *tokens.TypeRef
	Throws     *tokens.TypeRef        // Error type that this function can throw
	IsGeneric  bool                   // True if this is a generic function with type parameters
	TypeParams []*tokens.TypeParam    // Full type parameters with constraints (e.g., T is Area)
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
		ReturnType: m.Type,
		Throws:     m.Throws,
		IsGeneric:  len(m.TypeParams) > 0,
		TypeParams: m.TypeParams,
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

	// If actual type is empty/unknown, we can't verify - skip check
	if actual.Type == "" {
		return true
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

// CheckThrowsHandled checks if a function call to a throwing function is properly handled
func (impl *CBackendImplementation) CheckThrowsHandled(f *tokens.FuncCall, sig *MethodSignature, scope *ast.Ast) {
	if sig.Throws == nil {
		return // Function doesn't throw
	}

	// For now, just emit a compile error - proper handling context will be added later
	scope.ErrorScope.NewCompileTimeError(
		"Unhandled Exception",
		fmt.Sprintf("function '%s' throws '%s' but error is not handled",
			f.Function, FormatTypeRef(sig.Throws)),
		f.Pos,
	)
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

	// Build type substitution map for generic functions and validate trait constraints
	typeSubst := make(map[string]*tokens.TypeRef)
	if sig.IsGeneric {
		// If no type arguments provided, we can't check - skip
		if len(f.TypeArgs) == 0 {
			return
		}
		// Build substitution map and validate trait constraints
		for i, typeParam := range sig.TypeParams {
			if i < len(f.TypeArgs) {
				concreteType := f.TypeArgs[i]
				typeSubst[typeParam.Name] = concreteType

				// Validate all trait constraints
				for _, traitName := range typeParam.AllTraits() {
					if !impl.TypeImplementsTrait(concreteType, traitName, scope) {
						scope.ErrorScope.NewCompileTimeError(
							"Trait Constraint Error",
							fmt.Sprintf("Type '%s' does not implement trait '%s' required by type parameter '%s'",
								FormatTypeRef(concreteType), traitName, typeParam.Name),
							f.Pos,
						)
					}
				}
			}
		}
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

		// Apply type substitution for generic functions
		if len(typeSubst) > 0 {
			expectedType = substituteTypeRef(expectedType, typeSubst)
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
		} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
			// Warn about implicit lossy numeric conversion
			paramName := ""
			if i < len(sig.ParamNames) {
				paramName = " (" + sig.ParamNames[i] + ")"
			}
			scope.ErrorScope.NewCompileTimeWarning(
				"Lossy Conversion",
				"Argument"+paramName+": "+warning,
				f.Pos,
			)
		}
	}

	// Check if throwing function is properly handled
	impl.CheckThrowsHandled(f, sig, scope)
}

// CheckAssignmentType checks if assignment value matches variable type
func (impl *CBackendImplementation) CheckAssignmentType(a *tokens.Assignment, scope *ast.Ast) {
	if a == nil || scope.ErrorScope == nil {
		return
	}

	// Look up variable type
	varOpt := scope.ResolveSymbolAsVariable(a.Name)
	if varOpt.IsNil() {
		return
	}

	variable := varOpt.Unwrap()

	// Check if it's a const being reassigned (only for direct variable assignment)
	if a.Field == "" && a.Index == nil && variable.IsConst {
		scope.ErrorScope.NewCompileTimeError(
			"Constant Reassignment",
			"Cannot reassign constant '"+a.Name+"'",
			a.Pos,
		)
		return
	}

	// Get the variable's type from CProgramValues
	fullName := variable.GetFullName()
	valueInfo, hasInfo := (*CProgramValues)[fullName]
	if !hasInfo || valueInfo.GeckoType == nil {
		TypeCheckError(scope, a.Pos, "assignment to '"+a.Name+"'", "type information not available")
		return
	}

	var expectedType *tokens.TypeRef

	// Handle field assignment (e.g., circ.radius = 10)
	if a.Field != "" {
		// Get the class type
		typeName := valueInfo.GeckoType.Type
		rootScope := scope.GetRoot()
		classOpt := rootScope.ResolveClass(typeName)
		if classOpt.IsNil() {
			TypeCheckError(scope, a.Pos, "field assignment '"+a.Name+"."+a.Field+"'",
				"cannot resolve type '"+typeName+"'")
			return
		}
		class := classOpt.Unwrap()

		// Find the field
		fieldVar, ok := class.Variables[a.Field]
		if !ok {
			TypeCheckError(scope, a.Pos, "field assignment '"+a.Name+"."+a.Field+"'",
				"field '"+a.Field+"' not found on type '"+typeName+"'")
			return
		}

		// Get field's type from CProgramValues
		fieldFullName := fieldVar.GetFullName()
		fieldInfo, hasFieldInfo := (*CProgramValues)[fieldFullName]
		if !hasFieldInfo || fieldInfo.GeckoType == nil {
			TypeCheckError(scope, a.Pos, "field assignment '"+a.Name+"."+a.Field+"'",
				"type information not available for field")
			return
		}
		expectedType = fieldInfo.GeckoType

		// If indexing into the field (e.g., self.input_buf[i] = c), get element type
		if a.Index != nil && expectedType != nil {
			if expectedType.Size != nil && expectedType.Size.Type != nil {
				// Fixed-size array: [N]T - get element type T
				expectedType = expectedType.Size.Type
			} else if expectedType.Array != nil {
				// Dynamic array: []T - get element type T
				expectedType = expectedType.Array
			}
		}
	} else if a.Index != nil {
		// Direct array indexing (e.g., arr[i] = val)
		expectedType = valueInfo.GeckoType
		if expectedType != nil {
			if expectedType.Size != nil && expectedType.Size.Type != nil {
				expectedType = expectedType.Size.Type
			} else if expectedType.Array != nil {
				expectedType = expectedType.Array
			}
		}
	} else {
		// Direct variable assignment
		expectedType = valueInfo.GeckoType
	}

	// Get the expression type
	actualType := impl.GetTypeOfExpression(a.Value, scope)
	if actualType == nil {
		TypeCheckError(scope, a.Pos, "assignment to '"+a.Name+"'",
			"cannot infer type of right-hand side expression")
		return
	}

	if !TypesAreCompatible(expectedType, actualType, scope) {
		targetName := a.Name
		if a.Field != "" {
			targetName = a.Name + "." + a.Field
		}
		scope.ErrorScope.NewCompileTimeError(
			"Type Mismatch",
			"Cannot assign '"+FormatTypeRef(actualType)+"' to '"+
				targetName+"' of type '"+FormatTypeRef(expectedType)+"'",
			a.Pos,
		)
	} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
		targetName := a.Name
		if a.Field != "" {
			targetName = a.Name + "." + a.Field
		}
		scope.ErrorScope.NewCompileTimeWarning(
			"Lossy Conversion",
			"Assignment to '"+targetName+"': "+warning,
			a.Pos,
		)
	}
}

// CheckReturnType validates that a return expression matches the function's declared return type
func (impl *CBackendImplementation) CheckReturnType(expr *tokens.Expression, scope *ast.Ast) {
	if expr == nil || scope.ErrorScope == nil {
		return
	}

	// Walk up scope chain to find the function's return type
	// Return statements can be nested inside if/while/for blocks
	var expectedType *tokens.TypeRef
	currentScope := scope
	for currentScope != nil {
		info, ok := (*CScopeDataMap)[currentScope.GetFullName()]
		if ok && info.CurrentFuncReturnType != nil {
			expectedType = info.CurrentFuncReturnType
			break
		}
		// Also check if this scope has CurrentFunc set (indicates we're in a method)
		if ok && info.CurrentFunc != "" {
			// Found function scope but no return type = void function
			break
		}
		currentScope = currentScope.Parent
	}

	// If no function context found, skip check (shouldn't happen in valid code)
	if currentScope == nil {
		return
	}

	// Get the actual return expression type
	actualType := impl.GetTypeOfExpression(expr, scope)
	if actualType == nil {
		return // Can't determine type, skip check
	}

	// If no return type declared (void function), we shouldn't be returning a value
	if expectedType == nil {
		if actualType.Type != "void" {
			scope.ErrorScope.NewCompileTimeError(
				"Return Type Mismatch",
				"Cannot return value from void function",
				expr.Pos,
			)
		}
		return
	}

	// Check compatibility
	if !TypesAreCompatible(expectedType, actualType, scope) {
		scope.ErrorScope.NewCompileTimeError(
			"Return Type Mismatch",
			"Cannot return '"+FormatTypeRef(actualType)+"' from function expecting '"+FormatTypeRef(expectedType)+"'",
			expr.Pos,
		)
	} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
		scope.ErrorScope.NewCompileTimeWarning(
			"Lossy Conversion",
			"Return statement: "+warning,
			expr.Pos,
		)
	}
}

// CheckVoidReturn validates that void return is used in void function
func (impl *CBackendImplementation) CheckVoidReturn(scope *ast.Ast, pos lexer.Position) {
	if scope.ErrorScope == nil {
		return
	}

	// Get the current function's return type from scope info
	info := CGetScopeInformation(scope)
	expectedType := info.CurrentFuncReturnType

	// If function has a return type, void return is an error
	if expectedType != nil && expectedType.Type != "void" {
		scope.ErrorScope.NewCompileTimeError(
			"Missing Return Value",
			"Function expects return type '"+FormatTypeRef(expectedType)+"' but returns void",
			pos,
		)
	}
}

// ValidateClassTypeArgs checks that type arguments satisfy class-level type parameter constraints
func (impl *CBackendImplementation) ValidateClassTypeArgs(className string, typeArgs []*tokens.TypeRef, scope *ast.Ast, pos lexer.Position) {
	if className == "" || len(typeArgs) == 0 || scope.ErrorScope == nil {
		return
	}

	// Get the generic class token
	classToken, ok := Generics.GenericClasses[className]
	if !ok || classToken == nil {
		return // Not a registered generic class
	}

	// Check each type parameter constraint
	for i, typeParam := range classToken.TypeParams {
		if i >= len(typeArgs) {
			break
		}

		concreteType := typeArgs[i]
		for _, traitName := range typeParam.AllTraits() {
			if !impl.TypeImplementsTrait(concreteType, traitName, scope) {
				scope.ErrorScope.NewCompileTimeError(
					"Trait Constraint Error",
					fmt.Sprintf("Type '%s' does not implement trait '%s' required by type parameter '%s' of class '%s'",
						FormatTypeRef(concreteType), traitName, typeParam.Name, className),
					pos,
				)
			}
		}
	}
}

// CheckStructLiteralTypes validates that struct literal field values match expected types
func (impl *CBackendImplementation) CheckStructLiteralTypes(structType string, fields []*tokens.ObjectKeyValue, scope *ast.Ast, pos lexer.Position) {
	if structType == "" || len(fields) == 0 || scope.ErrorScope == nil {
		return
	}

	// Look up the struct/class in scope
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(structType)

	// Also check imported modules
	if classOpt.IsNil() {
		for _, child := range rootScope.Children {
			classOpt = child.ResolveClass(structType)
			if !classOpt.IsNil() {
				break
			}
		}
	}

	if classOpt.IsNil() {
		// Class not found - might be generic or external, can't check
		return
	}

	class := classOpt.Unwrap()

	// Check each field
	for _, kv := range fields {
		fieldName := kv.Key
		fieldVar, ok := class.Variables[fieldName]
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Unknown Field",
				fmt.Sprintf("Struct '%s' has no field named '%s'", structType, fieldName),
				pos,
			)
			continue
		}

		// Get expected type from CProgramValues
		fieldFullName := fieldVar.GetFullName()
		fieldInfo, hasInfo := (*CProgramValues)[fieldFullName]
		if !hasInfo || fieldInfo.GeckoType == nil {
			continue // Can't check without type info
		}

		// Get actual type of the value expression
		actualType := impl.GetTypeOfExpression(kv.Value, scope)
		if actualType == nil {
			continue
		}

		expectedType := fieldInfo.GeckoType
		if !TypesAreCompatible(expectedType, actualType, scope) {
			scope.ErrorScope.NewCompileTimeError(
				"Type Mismatch",
				fmt.Sprintf("Field '%s' expects type '%s', got '%s'",
					fieldName, FormatTypeRef(expectedType), FormatTypeRef(actualType)),
				pos,
			)
		} else if warning := IsLossyConversion(actualType, expectedType); warning != "" {
			scope.ErrorScope.NewCompileTimeWarning(
				"Lossy Conversion",
				fmt.Sprintf("Field '%s': %s", fieldName, warning),
				pos,
			)
		}
	}
}

// TypeImplementsTrait checks if a type implements a given trait
func (impl *CBackendImplementation) TypeImplementsTrait(typeRef *tokens.TypeRef, traitName string, scope *ast.Ast) bool {
	if typeRef == nil || traitName == "" {
		return true // Can't check, assume OK
	}

	typeName := typeRef.Type
	if typeName == "" {
		return true
	}

	// Remove pointer suffix for class lookup
	baseType := typeName
	for len(baseType) > 0 && baseType[len(baseType)-1] == '*' {
		baseType = baseType[:len(baseType)-1]
	}

	// Look up the class in scope
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(baseType)

	// Also check imported modules
	if classOpt.IsNil() {
		for _, child := range rootScope.Children {
			classOpt = child.ResolveClass(baseType)
			if !classOpt.IsNil() {
				break
			}
		}
	}

	if classOpt.IsNil() {
		// Class not found - might be a primitive or external type
		// For now, return true (can't verify)
		return true
	}

	class := classOpt.Unwrap()

	// Check if the class has the trait implemented
	// Traits are stored with mangled names like "TraitName" or "TraitName__TypeArg"
	for registeredTrait := range class.Traits {
		// Check for exact match or prefix match (for generic traits)
		if registeredTrait == traitName || strings.HasPrefix(registeredTrait, traitName+"__") {
			return true
		}
	}

	return false
}
