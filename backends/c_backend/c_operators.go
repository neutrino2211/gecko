package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// OperatorTraitInfo maps an operator to its trait name and method name
type OperatorTraitInfo struct {
	TraitName  string
	MethodName string
}

// operatorTraitMap maps operators to their trait info
var operatorTraitMap = map[string]OperatorTraitInfo{
	"+":  {"Add", "add"},
	"-":  {"Sub", "sub"},
	"*":  {"Mul", "mul"},
	"/":  {"Div", "div"},
	"==": {"Eq", "eq"},
	"!=": {"Ne", "ne"},
	"<":  {"Lt", "lt"},
	">":  {"Gt", "gt"},
	"<=": {"Le", "le"},
	">=": {"Ge", "ge"},
	"&":  {"BitAnd", "bitand"},
	"|":  {"BitOr", "bitor"},
	"^":  {"BitXor", "bitxor"},
	"<<": {"Shl", "shl"},
	">>": {"Shr", "shr"},
}

// unaryOperatorTraitMap maps unary operators to their trait info
var unaryOperatorTraitMap = map[string]OperatorTraitInfo{
	"-": {"Neg", "neg"},
	"!": {"Not", "not"},
}

// isPrimitiveType checks if a type name is a primitive type
func isPrimitiveType(typeName string) bool {
	primitives := map[string]bool{
		"int":    true,
		"int8":   true,
		"int16":  true,
		"int32":  true,
		"int64":  true,
		"uint":   true,
		"uint8":  true,
		"uint16": true,
		"uint32": true,
		"uint64": true,
		"bool":   true,
		"float":  true,
		"float32": true,
		"float64": true,
		"string": true,
		"void":   true,
	}
	return primitives[typeName]
}

// GetTypeOfLiteral attempts to determine the type of a literal expression
func (impl *CBackendImplementation) GetTypeOfLiteral(l *tokens.Literal, scope *ast.Ast) *tokens.TypeRef {
	if l == nil {
		return nil
	}

	// Number literals - default to int32
	if l.Number != "" {
		return &tokens.TypeRef{Type: "int32"}
	}

	// Boolean literals
	if l.Bool != "" {
		return &tokens.TypeRef{Type: "bool"}
	}

	// String literals
	if l.String != "" {
		return &tokens.TypeRef{Type: "string"}
	}

	// Symbol - look up in scope
	if l.Symbol != "" {
		if l.SymbolModule != "" {
			// module.field or var.field - get field type
			varOpt := scope.ResolveSymbolAsVariable(l.SymbolModule)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				fullName := variable.GetFullName()
				if info, ok := (*CProgramValues)[fullName]; ok && info.GeckoType != nil {
					typeName := info.GeckoType.Type
					rootScope := scope.GetRoot()
					classOpt := rootScope.ResolveClass(typeName)
					if !classOpt.IsNil() {
						class := classOpt.Unwrap()
						if fieldVar, ok := class.Variables[l.Symbol]; ok {
							fieldFullName := fieldVar.GetFullName()
							if fieldInfo, ok := (*CProgramValues)[fieldFullName]; ok {
								return fieldInfo.GeckoType
							}
						}
					}
				}
			}
		} else {
			// Simple symbol
			varOpt := scope.ResolveSymbolAsVariable(l.Symbol)
			if !varOpt.IsNil() {
				variable := varOpt.Unwrap()
				fullName := variable.GetFullName()
				if info, ok := (*CProgramValues)[fullName]; ok {
					return info.GeckoType
				}
			}
		}
	}

	// Struct literal
	if l.IsStructLiteral() {
		return &tokens.TypeRef{Type: l.StructType}
	}

	// Function call - would need return type analysis
	if l.FuncCall != nil {
		return impl.GetTypeOfFuncCall(l.FuncCall, scope)
	}

	return nil
}

// GetTypeOfFuncCall attempts to determine the return type of a function call
func (impl *CBackendImplementation) GetTypeOfFuncCall(f *tokens.FuncCall, scope *ast.Ast) *tokens.TypeRef {
	if f == nil {
		return nil
	}

	// Static method call: Type::method()
	if f.StaticType != "" {
		rootScope := scope.GetRoot()
		classOpt := rootScope.ResolveClass(f.StaticType)
		if !classOpt.IsNil() {
			class := classOpt.Unwrap()
			if method, ok := class.Methods[f.Function]; ok {
				return &tokens.TypeRef{Type: method.Type}
			}
		}
	}

	// Regular function call
	if f.Module == "" {
		mth := scope.ResolveMethod(f.Function)
		if !mth.IsNil() {
			return &tokens.TypeRef{Type: mth.Unwrap().Type}
		}
	}

	return nil
}

// GetTypeOfUnary gets the type of a unary expression
func (impl *CBackendImplementation) GetTypeOfUnary(u *tokens.Unary, scope *ast.Ast) *tokens.TypeRef {
	if u == nil {
		return nil
	}

	if u.Primary != nil {
		return impl.GetTypeOfPrimary(u.Primary, scope)
	}

	if u.Unary != nil {
		return impl.GetTypeOfUnary(u.Unary, scope)
	}

	return nil
}

// GetTypeOfPrimary gets the type of a primary expression
func (impl *CBackendImplementation) GetTypeOfPrimary(p *tokens.Primary, scope *ast.Ast) *tokens.TypeRef {
	if p == nil {
		return nil
	}

	if p.Literal != nil {
		return impl.GetTypeOfLiteral(p.Literal, scope)
	}

	if p.SubExpression != nil {
		return impl.GetTypeOfExpression(p.SubExpression, scope)
	}

	return nil
}

// GetTypeOfMultiplication gets the type of a multiplication expression
func (impl *CBackendImplementation) GetTypeOfMultiplication(m *tokens.Multiplication, scope *ast.Ast) *tokens.TypeRef {
	if m == nil {
		return nil
	}
	return impl.GetTypeOfUnary(m.Unary, scope)
}

// GetTypeOfAddition gets the type of an addition expression
func (impl *CBackendImplementation) GetTypeOfAddition(a *tokens.Addition, scope *ast.Ast) *tokens.TypeRef {
	if a == nil {
		return nil
	}
	return impl.GetTypeOfMultiplication(a.Multiplication, scope)
}

// GetTypeOfComparison gets the type of a comparison expression
func (impl *CBackendImplementation) GetTypeOfComparison(c *tokens.Comparison, scope *ast.Ast) *tokens.TypeRef {
	if c == nil {
		return nil
	}
	// Comparisons return bool
	if c.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfAddition(c.Addition, scope)
}

// GetTypeOfEquality gets the type of an equality expression
func (impl *CBackendImplementation) GetTypeOfEquality(e *tokens.Equality, scope *ast.Ast) *tokens.TypeRef {
	if e == nil {
		return nil
	}
	// Equality checks return bool
	if e.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfComparison(e.Comparison, scope)
}

// GetTypeOfLogicalAnd gets the type of a logical AND expression
func (impl *CBackendImplementation) GetTypeOfLogicalAnd(la *tokens.LogicalAnd, scope *ast.Ast) *tokens.TypeRef {
	if la == nil {
		return nil
	}
	// Logical AND returns bool
	if la.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfEquality(la.Equality, scope)
}

// GetTypeOfLogicalOr gets the type of a logical OR expression
func (impl *CBackendImplementation) GetTypeOfLogicalOr(lo *tokens.LogicalOr, scope *ast.Ast) *tokens.TypeRef {
	if lo == nil {
		return nil
	}
	// Logical OR returns bool
	if lo.Next != nil {
		return &tokens.TypeRef{Type: "bool"}
	}
	return impl.GetTypeOfLogicalAnd(lo.LogicalAnd, scope)
}

// GetTypeOfExpression gets the type of an expression
func (impl *CBackendImplementation) GetTypeOfExpression(e *tokens.Expression, scope *ast.Ast) *tokens.TypeRef {
	if e == nil {
		return nil
	}
	return impl.GetTypeOfLogicalOr(e.LogicalOr, scope)
}

// HasOperatorTrait checks if a type has an operator trait implemented
func (impl *CBackendImplementation) HasOperatorTrait(typeName string, traitName string, scope *ast.Ast) bool {
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(typeName)
	if classOpt.IsNil() {
		return false
	}

	class := classOpt.Unwrap()

	// Check if the class has this trait implemented
	// Trait names are mangled as "TraitName__TypeArg" (e.g., "Add__Point")
	for tName := range class.Traits {
		// Check exact match or prefix match for generic traits
		if tName == traitName {
			return true
		}
		// Check if trait name starts with the base trait name followed by "__"
		prefix := traitName + "__"
		if len(tName) >= len(prefix) && tName[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// GetOperatorTraitName returns the full mangled trait name for an operator if the type implements it
func (impl *CBackendImplementation) GetOperatorTraitName(typeName string, traitName string, scope *ast.Ast) (string, bool) {
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(typeName)
	if classOpt.IsNil() {
		return "", false
	}

	class := classOpt.Unwrap()

	// Check if the class has this trait implemented
	for tName := range class.Traits {
		if tName == traitName {
			return tName, true
		}
		prefix := traitName + "__"
		if len(tName) >= len(prefix) && tName[:len(prefix)] == prefix {
			return tName, true
		}
	}

	return "", false
}

// GetOperatorTraitMethodCall generates a trait method call for an operator
func (impl *CBackendImplementation) GetOperatorTraitMethodCall(
	leftCode string,
	leftType *tokens.TypeRef,
	rightCode string,
	op string,
	scope *ast.Ast,
) (string, bool) {
	if leftType == nil {
		return "", false
	}

	typeName := leftType.Type
	if isPrimitiveType(typeName) {
		return "", false
	}

	traitInfo, ok := operatorTraitMap[op]
	if !ok {
		return "", false
	}

	mangledTraitName, found := impl.GetOperatorTraitName(typeName, traitInfo.TraitName, scope)
	if !found {
		return "", false
	}

	// Generate: TypeName__MangledTraitName__methodName(&left, right)
	methodName := typeName + "__" + mangledTraitName + "__" + traitInfo.MethodName
	return methodName + "(&(" + leftCode + "), " + rightCode + ")", true
}

// GetUnaryOperatorTraitMethodCall generates a trait method call for a unary operator
func (impl *CBackendImplementation) GetUnaryOperatorTraitMethodCall(
	operandCode string,
	operandType *tokens.TypeRef,
	op string,
	scope *ast.Ast,
) (string, bool) {
	if operandType == nil {
		return "", false
	}

	typeName := operandType.Type
	if isPrimitiveType(typeName) {
		return "", false
	}

	traitInfo, ok := unaryOperatorTraitMap[op]
	if !ok {
		return "", false
	}

	mangledTraitName, found := impl.GetOperatorTraitName(typeName, traitInfo.TraitName, scope)
	if !found {
		return "", false
	}

	// Generate: TypeName__MangledTraitName__methodName(&operand)
	methodName := typeName + "__" + mangledTraitName + "__" + traitInfo.MethodName
	return methodName + "(&(" + operandCode + "))", true
}
