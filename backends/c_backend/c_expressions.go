// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

// ExpressionToCString converts an expression to C code
func (impl *CBackendImplementation) ExpressionToCString(e *tokens.Expression, scope *ast.Ast) string {
	if e == nil || e.Cond == nil {
		return ""
	}
	cond := impl.OrExpressionToCString(e.Cond.LogicalOr, scope)
	if e.Cond.TrueExpr != nil {
		trueExpr := impl.ExpressionToCString(e.Cond.TrueExpr, scope)
		falseExpr := impl.ConditionalExprToCString(e.Cond.FalseExpr, scope)
		return fmt.Sprintf("(%s) ? (%s) : (%s)", cond, trueExpr, falseExpr)
	}
	return cond
}

// ConditionalExprToCString converts the false branch of a ternary to C code
func (impl *CBackendImplementation) ConditionalExprToCString(c *tokens.ConditionalExpr, scope *ast.Ast) string {
	if c == nil {
		return ""
	}
	cond := impl.OrExpressionToCString(c.LogicalOr, scope)
	if c.TrueExpr != nil {
		trueExpr := impl.ExpressionToCString(c.TrueExpr, scope)
		falseExpr := impl.ConditionalExprToCString(c.FalseExpr, scope)
		return fmt.Sprintf("(%s) ? (%s) : (%s)", cond, trueExpr, falseExpr)
	}
	return cond
}

// getTypeOriginModule resolves the package/module that owns a type.
// For monomorphized generic names (e.g., Option__int32_t), it falls back
// to the base generic class name.
func getTypeOriginModule(typeName string, scope *ast.Ast) string {
	if scope == nil {
		return ""
	}

	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(typeName)
	if !classOpt.IsNil() {
		return classOpt.Unwrap().GetOriginModule()
	}

	if idx := strings.Index(typeName, "__"); idx > 0 {
		baseTypeName := typeName[:idx]
		classOpt = rootScope.ResolveClass(baseTypeName)
		if !classOpt.IsNil() {
			return classOpt.Unwrap().GetOriginModule()
		}
		if originModule, ok := Generics.GenericClassOrigins[baseTypeName]; ok {
			return originModule
		}
	}

	if originModule, ok := Generics.GenericClassOrigins[typeName]; ok {
		return originModule
	}

	return ""
}

// concretizeGenericTraitName replaces generic placeholder trait args (e.g., T)
// with concrete type args from the owning generic class.
func concretizeGenericTraitName(traitName string, baseTypeName string, typeArgStrs []string) string {
	if traitName == "" || len(typeArgStrs) == 0 {
		return traitName
	}

	classToken, ok := Generics.GenericClasses[baseTypeName]
	if !ok || classToken == nil || len(classToken.TypeParams) == 0 {
		return traitName
	}

	paramToArg := make(map[string]string, len(classToken.TypeParams))
	for i, param := range classToken.TypeParams {
		if i < len(typeArgStrs) {
			paramToArg[param.Name] = MangleTypeArgForIdentifier(typeArgStrs[i])
		}
	}

	parts := strings.Split(traitName, "__")
	for i := 1; i < len(parts); i++ {
		if concrete, ok := paramToArg[parts[i]]; ok {
			parts[i] = concrete
		}
	}

	return strings.Join(parts, "__")
}

// OrExpressionToCString handles the 'or' keyword for error handling
func (impl *CBackendImplementation) OrExpressionToCString(o *tokens.OrExpression, scope *ast.Ast) string {
	if o == nil {
		return ""
	}

	base := impl.LogicalOrToCString(o.LogicalOr, scope)

	// Handle 'or' keyword for fallback values
	if o.Or != nil {
		leftType := impl.GetTypeOfLogicalOr(o.LogicalOr, scope)
		defaultValue := impl.OrExpressionToCString(o.Or, scope)

		// Check if this type implements @or_hook (Orable trait)
		if leftType != nil && leftType.Type != "" {
			baseTypeName := leftType.Type
			typeName := baseTypeName

			// Collect C type args for mangling
			var typeArgStrs []string
			if len(leftType.TypeArgs) > 0 {
				typeArgStrs = make([]string, len(leftType.TypeArgs))
				for i, arg := range leftType.TypeArgs {
					if cType, ok := GeckoToCType[arg.Type]; ok {
						typeArgStrs[i] = cType
					} else {
						typeArgStrs[i] = arg.Type
					}
				}
				typeName = mangleName(baseTypeName, typeArgStrs)
			}

			orHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookOr)
			if orHook != nil {
				mangledTraitName, found := impl.GetOperatorTraitName(typeName, orHook.TraitName, scope)
				if found && len(orHook.Methods) > 0 {
					base = impl.moveLogicalOrValue(o.LogicalOr, base, scope)
					if o.Or.Or == nil {
						defaultValue = impl.moveLogicalOrValue(o.Or.LogicalOr, defaultValue, scope)
					}
					methodName := orHook.Methods[0]
					// Generic trait impl placeholders are stored with type params (e.g., Orable__T).
					// Replace placeholders with concrete class type args.
					mangledTraitName = concretizeGenericTraitName(mangledTraitName, baseTypeName, typeArgStrs)

					// Only add module prefix for generic types (detected by having type args)
					var fullMethodName string
					if len(typeArgStrs) > 0 {
						modulePrefix := getTypeOriginModule(typeName, scope)
						if modulePrefix != "" {
							modulePrefix += "__"
						}
						fullMethodName = modulePrefix + typeName + "__" + mangledTraitName + "__" + methodName
					} else {
						fullMethodName = typeName + "__" + mangledTraitName + "__" + methodName
					}

					// Prefer lazy lowering when Try hook exists for this same type:
					// evaluate RHS only if lhs has no value.
					tryHook := hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookTry)
					if tryHook != nil && len(tryHook.Methods) >= 2 {
						tryTraitName, hasTry := impl.GetOperatorTraitName(typeName, tryHook.TraitName, scope)
						if hasTry {
							tryTraitName = concretizeGenericTraitName(tryTraitName, baseTypeName, typeArgStrs)
							hasValueMethod := tryHook.Methods[0]
							tryUnwrapMethod := tryHook.Methods[1]

							var tryMethodPrefix string
							if len(typeArgStrs) > 0 {
								modulePrefix := getTypeOriginModule(typeName, scope)
								if modulePrefix != "" {
									modulePrefix += "__"
								}
								tryMethodPrefix = modulePrefix + typeName + "__" + tryTraitName + "__"
							} else {
								tryMethodPrefix = typeName + "__" + tryTraitName + "__"
							}

							hasValueCall := tryMethodPrefix + hasValueMethod
							tryUnwrapCall := tryMethodPrefix + tryUnwrapMethod

							orTempCounter++
							tempVar := "__or_tmp_" + strconv.Itoa(orTempCounter)
							fallback := "(" + defaultValue + ")"
							if dropMethod := dropMethodForType(leftType, scope); dropMethod != "" {
								fallback = "({ " + dropMethod + "(&" + tempVar + "); " + defaultValue + "; })"
							}
							return "({ __auto_type " + tempVar + " = " + base + "; (" +
								hasValueCall + "(&" + tempVar + ") ? " +
								tryUnwrapCall + "(&" + tempVar + ") : " + fallback + "); })"
						}
					}

					// Fallback when only Or hook exists: still use a temp so lhs may be an rvalue.
					orTempCounter++
					tempVar := "__or_tmp_" + strconv.Itoa(orTempCounter)
					// Note: this path keeps previous eager semantics for RHS evaluation.
					return "({ __auto_type " + tempVar + " = " + base + "; " + fullMethodName + "(&" + tempVar + ", " + defaultValue + "); })"
				}
			}
		}

		// Fallback: just use C's ternary (for simple cases like pointers)
		return "(" + base + " ? " + base + " : " + defaultValue + ")"
	}

	return base
}

// LogicalOrToCString converts logical OR expressions
func (impl *CBackendImplementation) LogicalOrToCString(lo *tokens.LogicalOr, scope *ast.Ast) string {
	if lo == nil {
		return ""
	}

	base := impl.LogicalAndToCString(lo.LogicalAnd, scope)

	if lo.Next != nil {
		base += " || " + impl.LogicalOrToCString(lo.Next, scope)
	}

	return base
}

// LogicalAndToCString converts logical AND expressions
func (impl *CBackendImplementation) LogicalAndToCString(la *tokens.LogicalAnd, scope *ast.Ast) string {
	if la == nil {
		return ""
	}

	base := impl.EqualityToCString(la.Equality, scope)

	if la.Next != nil {
		base += " && " + impl.LogicalAndToCString(la.Next, scope)
	}

	return base
}

// EqualityToCString converts equality expressions
func (impl *CBackendImplementation) EqualityToCString(eq *tokens.Equality, scope *ast.Ast) string {
	if eq == nil {
		return ""
	}

	base := impl.ComparisonToCString(eq.Comparison, scope)

	if eq.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfComparison(eq.Comparison, scope)
		rightCode := impl.EqualityToCString(eq.Next, scope)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, eq.Op, scope, eq.Pos); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + eq.Op + " " + rightCode
	}

	return base
}

// ComparisonToCString converts comparison expressions
func (impl *CBackendImplementation) ComparisonToCString(c *tokens.Comparison, scope *ast.Ast) string {
	if c == nil {
		return ""
	}

	base := impl.AdditionToCString(c.Addition, scope)

	if c.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfAddition(c.Addition, scope)
		rightCode := impl.ComparisonToCString(c.Next, scope)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, c.Op, scope, c.Pos); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + c.Op + " " + rightCode
	}

	return base
}

// AdditionToCString converts addition expressions
func (impl *CBackendImplementation) AdditionToCString(a *tokens.Addition, scope *ast.Ast) string {
	if a == nil {
		return ""
	}

	base := impl.MultiplicationToCString(a.Multiplication, scope)

	if a.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfMultiplication(a.Multiplication, scope)
		rightType := impl.GetTypeOfAddition(a.Next, scope)
		rightCode := impl.AdditionToCString(a.Next, scope)

		impl.reportPointerArithmeticIfNeeded(a.Op, leftType, rightType, scope, a.Pos)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, a.Op, scope, a.Pos); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + a.Op + " " + rightCode
	}

	return base
}

func (impl *CBackendImplementation) reportPointerArithmeticIfNeeded(
	op string,
	leftType *tokens.TypeRef,
	rightType *tokens.TypeRef,
	scope *ast.Ast,
	pos lexer.Position,
) {
	if op != "+" && op != "-" {
		return
	}
	if scope == nil || scope.ErrorScope == nil {
		return
	}

	leftIsPtr := leftType != nil && leftType.Pointer
	rightIsPtr := rightType != nil && rightType.Pointer
	if !leftIsPtr && !rightIsPtr {
		return
	}

	scope.ErrorScope.NewCompileTimeError(
		"Pointer Arithmetic Error",
		"Raw pointer arithmetic is not allowed in expression operators.\n"+
			"help: use @ptr_add/@ptr_sub intrinsics or std.memory.buffer.Buffer<T> for typed indexed access.",
		pos,
	)
}

// MultiplicationToCString converts multiplication expressions
func (impl *CBackendImplementation) MultiplicationToCString(m *tokens.Multiplication, scope *ast.Ast) string {
	if m == nil {
		return ""
	}

	base := impl.UnaryToCString(m.Unary, scope)

	if m.Next != nil {
		// Try operator overloading first
		leftType := impl.GetTypeOfUnary(m.Unary, scope)
		rightCode := impl.MultiplicationToCString(m.Next, scope)

		if traitCall, ok := impl.GetOperatorTraitMethodCall(base, leftType, rightCode, m.Op, scope, m.Pos); ok {
			return traitCall
		}

		// Fall back to raw C operator
		base += " " + m.Op + " " + rightCode
	}

	return base
}

// UnaryToCString converts unary expressions
func (impl *CBackendImplementation) UnaryToCString(u *tokens.Unary, scope *ast.Ast) string {
	if u == nil {
		return ""
	}

	var base string

	if u.Unary != nil {
		innerCode := impl.UnaryToCString(u.Unary, scope)
		innerType := impl.GetTypeOfUnary(u.Unary, scope)

		// Handle 'try' operator for error handling
		if u.Op == "try" {
			if traitCall, ok := impl.GetTryOperatorCall(innerCode, innerType, u.Unary, scope, u.Pos); ok {
				base = traitCall
			} else {
				operandType := "<unknown>"
				if innerType != nil && innerType.Type != "" {
					operandType = innerType.Type
				}
				if scope != nil && scope.ErrorScope != nil {
					scope.ErrorScope.NewCompileTimeError(
						"Try Expression Error",
						"'try' requires a Tryable operand with resolvable @try_hook methods; got '"+operandType+"'",
						u.Pos,
					)
				}
				// Keep expression emission stable after reporting the compile-time error.
				base = innerCode
			}
		} else if traitCall, ok := impl.GetUnaryOperatorTraitMethodCall(innerCode, innerType, u.Op, scope, u.Pos); ok {
			// Try operator overloading for other unary operators
			base = traitCall
		} else {
			base = u.Op + innerCode
		}
	} else if u.Primary != nil {
		base = impl.PrimaryToCString(u.Primary, scope)
	}

	// Handle cast expression (e.g., "value as *uint16" or "ptr as uint64")
	if u.Cast != nil {
		// Validate the cast target type
		if u.Cast.Type != nil {
			u.Cast.Type.Check(scope)
		}
		sourceType := impl.GetTypeOfUnaryWithoutCast(u, scope)
		impl.CheckCast(sourceType, u.Cast.Type, u.Cast.Trusted, scope, u.Cast.Pos)
		cType := TypeRefToCType(u.Cast.Type, scope)
		// C does not allow scalar-style casts into aggregates (e.g., (Pair)(0)).
		// Lower `0 as T` for aggregate T as a zero-initialized compound literal.
		if isZeroLiteralUnary(u) && isNonScalarCastTarget(u.Cast.Type) {
			base = "((" + cType + "){0})"
		} else {
			base = "((" + cType + ")(" + base + "))"
		}
	}

	return base
}

// GetTypeOfUnaryWithoutCast returns the operand type before an applied cast.
func (impl *CBackendImplementation) GetTypeOfUnaryWithoutCast(u *tokens.Unary, scope *ast.Ast) *tokens.TypeRef {
	if u == nil {
		return nil
	}
	if u.Op != "" {
		return &tokens.TypeRef{Type: "bool"}
	}
	if u.Unary != nil {
		return impl.GetTypeOfUnary(u.Unary, scope)
	}
	if u.Primary != nil {
		return impl.GetTypeOfPrimary(u.Primary, scope)
	}
	return nil
}

func isZeroLiteralUnary(u *tokens.Unary) bool {
	if u == nil || u.Primary == nil || u.Primary.Literal == nil {
		return false
	}
	lit := u.Primary.Literal
	return lit.Number == "0"
}

func isNonScalarCastTarget(t *tokens.TypeRef) bool {
	if t == nil {
		return false
	}
	if t.Pointer || t.Array != nil || t.Size != nil || t.FuncType != nil {
		return false
	}

	normalized := normalizeTypeName(t.Type)
	switch normalized {
	case "void", "bool", "string",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64":
		return false
	}
	if _, isEnum := EnumToCType[t.Type]; isEnum {
		return false
	}
	return true
}

// orTempCounter generates unique names for or-expression temporaries.
var orTempCounter int = 0
