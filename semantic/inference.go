// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) inferMethodCallStatement(call *tokens.MethodCall, env *flowEnv) {
	if call == nil {
		return
	}
	lit := &tokens.Literal{Symbol: call.Base, Chain: call.Chain}
	expr := &tokens.Expression{Cond: &tokens.ConditionalExpr{LogicalOr: &tokens.OrExpression{LogicalOr: &tokens.LogicalOr{LogicalAnd: &tokens.LogicalAnd{Equality: &tokens.Equality{Comparison: &tokens.Comparison{Addition: &tokens.Addition{Multiplication: &tokens.Multiplication{Unary: &tokens.Unary{Primary: &tokens.Primary{Literal: lit}}}}}}}}}}}
	a.inferExpression(expr, env, nil)
}

func (a *analyzer) inferExpression(expr *tokens.Expression, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if expr == nil || expr.Cond == nil || expr.Cond.LogicalOr == nil {
		return nil
	}
	// Handle ternary: condition must be bool, result type is from true/false branches
	if expr.Cond.TrueExpr != nil {
		trueType := a.inferExpression(expr.Cond.TrueExpr, env, expected)
		if expr.Cond.FalseExpr != nil {
			a.inferConditionalExpr(expr.Cond.FalseExpr, env, expected)
		}
		if trueType != nil {
			a.program.expressionTypes[expr] = CloneTypeRef(trueType)
		}
		return trueType
	}
	resolved := a.inferOrExpression(expr.Cond.LogicalOr, env, expected)
	if resolved != nil {
		a.program.expressionTypes[expr] = CloneTypeRef(resolved)
	}
	return resolved
}

func (a *analyzer) inferConditionalExpr(c *tokens.ConditionalExpr, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if c == nil || c.LogicalOr == nil {
		return nil
	}
	if c.TrueExpr != nil {
		a.inferExpression(c.TrueExpr, env, expected)
		if c.FalseExpr != nil {
			a.inferConditionalExpr(c.FalseExpr, env, expected)
		}
		return a.inferExpression(c.TrueExpr, env, expected)
	}
	return a.inferOrExpression(c.LogicalOr, env, expected)
}

func (a *analyzer) inferOrExpression(orExpr *tokens.OrExpression, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if orExpr == nil {
		return nil
	}
	left := a.inferLogicalOr(orExpr.LogicalOr, env, expected)
	if orExpr.Or == nil {
		return left
	}
	right := a.inferOrExpression(orExpr.Or, env, expected)
	if right != nil {
		return right
	}
	return left
}

func (a *analyzer) inferLogicalOr(lo *tokens.LogicalOr, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if lo == nil {
		return nil
	}
	left := a.inferLogicalAnd(lo.LogicalAnd, env, expected)
	if lo.Next != nil {
		a.inferLogicalOr(lo.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferLogicalAnd(la *tokens.LogicalAnd, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if la == nil {
		return nil
	}
	left := a.inferEquality(la.Equality, env, expected)
	if la.Next != nil {
		a.inferLogicalAnd(la.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferEquality(eq *tokens.Equality, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if eq == nil {
		return nil
	}
	left := a.inferComparison(eq.Comparison, env, expected)
	if eq.Next != nil {
		a.inferEquality(eq.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferComparison(c *tokens.Comparison, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if c == nil {
		return nil
	}
	left := a.inferAddition(c.Addition, env, expected)
	if c.Next != nil {
		a.inferComparison(c.Next, env, expected)
		return &tokens.TypeRef{Type: "bool"}
	}
	return left
}

func (a *analyzer) inferAddition(add *tokens.Addition, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if add == nil {
		return nil
	}
	left := a.inferMultiplication(add.Multiplication, env, expected)
	if add.Next == nil {
		return left
	}
	right := a.inferAddition(add.Next, env, expected)
	if IsNumericType(left) && IsNumericType(right) {
		if strings.HasPrefix(left.Type, "float") || strings.HasPrefix(right.Type, "float") {
			return &tokens.TypeRef{Type: "float64"}
		}
		if left.Type == "uint" || right.Type == "uint" {
			return &tokens.TypeRef{Type: "uint"}
		}
		return &tokens.TypeRef{Type: "int32"}
	}
	if left != nil {
		return left
	}
	return right
}

func (a *analyzer) inferMultiplication(mul *tokens.Multiplication, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if mul == nil {
		return nil
	}
	left := a.inferUnary(mul.Unary, env, expected)
	if mul.Next == nil {
		return left
	}
	right := a.inferMultiplication(mul.Next, env, expected)
	if IsNumericType(left) && IsNumericType(right) {
		if strings.HasPrefix(left.Type, "float") || strings.HasPrefix(right.Type, "float") {
			return &tokens.TypeRef{Type: "float64"}
		}
		if left.Type == "uint" || right.Type == "uint" {
			return &tokens.TypeRef{Type: "uint"}
		}
		return &tokens.TypeRef{Type: "int32"}
	}
	if left != nil {
		return left
	}
	return right
}

func (a *analyzer) inferUnary(un *tokens.Unary, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if un == nil {
		return nil
	}
	if un.Unary != nil {
		inner := a.inferUnary(un.Unary, env, expected)
		if un.Op == "!" {
			if inner != nil && inner.Type != "bool" {
				return inner
			}
			return &tokens.TypeRef{Type: "bool"}
		}
		if un.Op == "try" {
			if inner != nil && len(inner.TypeArgs) > 0 {
				return CloneTypeRef(inner.TypeArgs[0])
			}
		}
		return inner
	}

	if un.Primary != nil {
		result := a.inferPrimary(un.Primary, env, expected)
		if un.Cast != nil && un.Cast.Type != nil {
			return CloneTypeRef(un.Cast.Type)
		}
		return result
	}
	return nil
}

func (a *analyzer) inferPrimary(p *tokens.Primary, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if p == nil {
		return nil
	}
	if p.SubExpression != nil {
		return a.inferExpression(p.SubExpression, env, expected)
	}
	return a.inferLiteral(p.Literal, env, expected)
}

// analyzeUnsafeBlock synthesizes an `@unsafe with H { ... }` block's Result type.
// It records a unique error-enum type name on the token (so codegen can emit the
// enum), infers the block's trailing expression as the Result's Ok type, and
// binds the block's name (set programmatically by the `let r =` / `r =` forms) to
// `Result<T, E>` so later statements can read its Ok/Err via Result's
// is_error/unwrap/err. It returns the synthesized Result type (or nil when the
// block has no binder, i.e. a bare `@unsafe with H { }` statement with no Result).
func (a *analyzer) analyzeUnsafeBlock(block *tokens.UnsafeBlock, env *flowEnv) *tokens.TypeRef {
	bodyEnv := env
	if block.Setup != nil {
		bodyEnv = env.clone()
		for _, binding := range block.Setup.Bindings {
			a.setupNames[binding.Name] = true
		}
		for _, binding := range block.Setup.Bindings {
			bodyEnv = a.analyzeFieldDecl(binding.Field(), bodyEnv)
		}
		for _, handler := range block.Handlers {
			a.inferExpression(handler.ToExpression(), bodyEnv, nil)
		}
	}
	bodyEnv = a.analyzeEntries(block.Body, bodyEnv)
	bindName, hasBind := tokens.UnsafeBlockBindNames[block]
	if !hasBind {
		return nil
	}
	enumName := tokens.UnsafeBlockErrorNames[block]
	if enumName == "" {
		a.unsafeBlockCounter++
		enumName = fmt.Sprintf("__gecko_unsafe_err%d", a.unsafeBlockCounter)
		tokens.UnsafeBlockErrorNames[block] = enumName
	}
	if _, ok := a.program.classes[enumName]; !ok {
		a.program.classes[enumName] = &ClassInfo{
			Name:   enumName,
			Fields: make(map[string]*tokens.TypeRef),
		}
	}

	var tType *tokens.TypeRef
	if len(block.Body) > 0 {
		last := block.Body[len(block.Body)-1]
		if block.Setup != nil {
			tType = a.inferExpression(last.UnsafeValue(), bodyEnv, nil)
		} else if last.UnsafeValue() != nil {
			tType = a.inferExpression(last.UnsafeValue(), bodyEnv, nil)
		}
	}
	if tType == nil {
		tType = &tokens.TypeRef{Type: "void"}
	}

	resType := &tokens.TypeRef{
		Type:     "Result",
		TypeArgs: []*tokens.TypeRef{tType, &tokens.TypeRef{Type: enumName}},
	}
	env.vars[bindName] = &boundVar{
		Name: bindName,
		Type: CloneTypeRef(resType),
	}
	return resType
}

func (a *analyzer) inferLiteral(l *tokens.Literal, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if l == nil {
		return nil
	}

	var inferred *tokens.TypeRef
	switch {
	case l.Number != "":
		if strings.Contains(l.Number, ".") {
			inferred = &tokens.TypeRef{Type: "float64"}
		} else {
			inferred = &tokens.TypeRef{Type: "int32"}
		}
	case l.Bool != "":
		inferred = &tokens.TypeRef{Type: "bool"}
	case l.HasStringLiteral():
		inferred = &tokens.TypeRef{Type: "string"}
	case l.Intrinsic != nil:
		inferred = a.inferIntrinsic(l.Intrinsic, env)
	case l.FuncCall != nil:
		inferred = a.inferFuncCall(l.FuncCall, env, expected)
	case (l.Symbol == "nil" || l.Symbol == "null") && len(l.Chain) == 0:
		inferred = &tokens.TypeRef{Type: "void", Pointer: true}
	case l.Symbol != "":
		inferred = a.inferSymbolLiteral(l, env)
	case l.StructType != "":
		inferred = &tokens.TypeRef{Type: l.StructType, TypeArgs: cloneTypeRefSlice(l.StructTypeArgs)}
	case len(l.Array) > 0:
		first := a.inferLiteral(l.Array[0], env, nil)
		if first != nil {
			inferred = &tokens.TypeRef{Array: CloneTypeRef(first)}
		}
	}

	if inferred != nil && l.IsPointer {
		inferred = &tokens.TypeRef{Type: inferred.Type, TypeArgs: cloneTypeRefSlice(inferred.TypeArgs), Pointer: true, NonNull: true}
	}
	if inferred != nil {
		a.program.literalTypes[l] = CloneTypeRef(inferred)
	}
	return inferred
}

func cloneTypeRefSlice(in []*tokens.TypeRef) []*tokens.TypeRef {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tokens.TypeRef, len(in))
	for i := range in {
		out[i] = CloneTypeRef(in[i])
	}
	return out
}

func (a *analyzer) inferSymbolLiteral(l *tokens.Literal, env *flowEnv) *tokens.TypeRef {
	var current *tokens.TypeRef
	if v := env.lookup(l.Symbol); v != nil {
		current = CloneTypeRef(v.Type)
		if current != nil && current.Pointer && env.nonNull[v.SymbolID] {
			current.NonNull = true
		}
	} else if gt, ok := a.program.globalsByName[l.Symbol]; ok {
		current = CloneTypeRef(gt)
		if current != nil && current.Pointer {
			if sid, exists := a.program.globalSymbolIDs[l.Symbol]; exists && env.nonNull[sid] {
				current.NonNull = true
			}
		}
	}

	if current == nil {
		if a.setupNames[l.Symbol] {
			a.program.addDiagnostic(Diagnostic{
				Severity: SeverityError,
				Kind:     DiagnosticTypeMismatch,
				Title:    "Scope Error",
				Message:  "Setup binding '" + l.Symbol + "' is not available in this scope",
				Pos:      l.Pos,
			})
		}
		return nil
	}

	if l.ArrayIndex != nil {
		a.inferExpression(l.ArrayIndex, env, &tokens.TypeRef{Type: "int32"})
		if current.Array != nil {
			current = CloneTypeRef(current.Array)
		} else if current.Size != nil {
			current = CloneTypeRef(current.Size.Type)
		} else if indexed := a.inferIndexAccessType(current, l.ArrayIndex, env); indexed != nil {
			current = indexed
		}
	}

	for _, chain := range l.Chain {
		if chain == nil {
			continue
		}
		if chain.IsMethodCall() {
			ret := a.inferMethodOnType(current, chain, env)
			if ret == nil {
				return nil
			}
			current = ret
			continue
		}
		classInfo := a.program.classes[current.Type]
		if classInfo == nil {
			return nil
		}
		fieldType := classInfo.Fields[chain.Name]
		if fieldType == nil {
			return nil
		}
		subst := classTypeSubst(current, classInfo.TypeParams)
		current = SubstituteTypeParams(fieldType, subst)
	}

	return current
}

func (a *analyzer) inferIndexAccessType(receiver *tokens.TypeRef, indexExpr *tokens.Expression, env *flowEnv) *tokens.TypeRef {
	if receiver == nil || receiver.Type == "" {
		return nil
	}
	arg := &tokens.Argument{Value: indexExpr}

	// Prefer conventional index hook method names.
	for _, methodName := range []string{"index", "get"} {
		candidates := a.program.staticMethods[receiver.Type][methodName]
		if len(candidates) == 0 {
			continue
		}
		attempt := a.resolveCallCandidates(candidates, nil, []*tokens.Argument{arg}, receiver, nil, env, lexer.Position{})
		if attempt == nil {
			continue
		}
		return CloneTypeRef(attempt.returnTyp)
	}

	return nil
}

func classTypeSubst(instanceType *tokens.TypeRef, params []*tokens.TypeParam) map[string]*tokens.TypeRef {
	if instanceType == nil || len(params) == 0 || len(instanceType.TypeArgs) == 0 {
		return nil
	}
	subst := make(map[string]*tokens.TypeRef)
	for i, p := range params {
		if i >= len(instanceType.TypeArgs) || p == nil {
			continue
		}
		subst[p.Name] = CloneTypeRef(instanceType.TypeArgs[i])
	}
	return subst
}

func (a *analyzer) ownerTypeParams(ownerType string) []*tokens.TypeParam {
	if ownerType == "" {
		return nil
	}
	classInfo := a.program.classes[ownerType]
	if classInfo == nil {
		return nil
	}
	return classInfo.TypeParams
}

func (a *analyzer) inferMethodOnType(receiver *tokens.TypeRef, chain *tokens.ChainAccess, env *flowEnv) *tokens.TypeRef {
	if receiver == nil || chain == nil {
		return nil
	}
	candidates := a.program.staticMethods[receiver.Type][chain.Name]
	if len(candidates) == 0 {
		return nil
	}
	attempt := a.resolveCallCandidates(candidates, chain.TypeArgs, chain.Args, receiver, nil, env, chain.Pos)
	if attempt == nil {
		return nil
	}
	return CloneTypeRef(attempt.returnTyp)
}

func (a *analyzer) inferIntrinsic(intr *tokens.Intrinsic, env *flowEnv) *tokens.TypeRef {
	if intr == nil {
		return nil
	}
	for _, arg := range intr.Args {
		a.inferExpression(arg, env, nil)
	}
	switch intr.Name {
	case "move":
		if len(intr.Args) == 1 {
			return a.inferExpression(intr.Args[0], env, nil)
		}
		return nil
	case "borrow", "borrow_mut":
		return a.inferBorrow(intr, env)
	case "size_of", "align_of":
		return &tokens.TypeRef{Type: "uint64"}
	case "alloc":
		return &tokens.TypeRef{Type: "void", Pointer: true}
	case "deref", "read_volatile":
		if len(intr.Args) == 1 {
			t := a.inferExpression(intr.Args[0], env, nil)
			if t != nil && t.Pointer {
				t = CloneTypeRef(t)
				t.Pointer = false
				t.NonNull = false
				t.Const = false
				t.Volatile = false
				return t
			}
		}
		return nil
	case "is_null", "is_not_null":
		return &tokens.TypeRef{Type: "bool"}
	default:
		return nil
	}
}
