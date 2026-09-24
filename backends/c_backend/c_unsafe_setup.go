package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

type unsafeSetupOperation struct {
	Pointer string
	Size    string
}

type unsafeSetupContext struct {
	Preparing    bool
	Producer     *tokens.Intrinsic
	Handlers     []*UnsafeHandlerInstance
	Allocations  []unsafeSetupOperation
	FailIndex    string
	FailureLabel string
}

func findUnsafeSetup(scope *ast.Ast) *unsafeSetupContext {
	for current := scope; current != nil; current = current.Parent {
		if info, ok := (*CScopeDataMap)[current.GetFullName()]; ok {
			if info.UnsafeSetup != nil {
				return info.UnsafeSetup
			}
			if info.FunctionBoundary {
				return nil
			}
		}
	}
	return nil
}

func (impl *CBackendImplementation) newUnsafeSetupBlock(parent *ast.Ast, block *tokens.UnsafeBlock) {
	unsafeBlockCounter++
	id := unsafeBlockCounter
	scope := &ast.Ast{Scope: fmt.Sprintf("unsafe_setup%d", id), Parent: parent}
	scope.Init(parent.ErrorScope)
	scope.Config = parent.Config
	info := CGetScopeInformation(scope)
	outer := CGetScopeInformation(parent)
	info.CurrentFunc = outer.CurrentFunc
	info.CurrentFuncReturnType = outer.CurrentFuncReturnType
	info.InUnsafe = true
	ctx := &unsafeSetupContext{
		Preparing:    true,
		FailIndex:    fmt.Sprintf("__gecko_failed%d", id),
		FailureLabel: fmt.Sprintf("__gecko_failure%d", id),
	}
	info.UnsafeSetup = ctx
	info.Code = "{\nint " + ctx.FailIndex + " = -1;\n"
	var bindings []*tokens.UnsafeBinding
	if block.Setup != nil {
		bindings = block.Setup.Bindings
	}
	for idx, binding := range bindings {
		if _, exists := scope.Variables[binding.Name]; exists {
			scope.ErrorScope.NewCompileTimeError("Unsafe Setup Error", "Duplicate setup binding '"+binding.Name+"'", binding.Pos)
			return
		}
		ctx.Producer = setupProducer(binding.Value)
		initializer := binding.Field()
		initializer.Name = fmt.Sprintf("__gecko_binding%d_%d", id, idx)
		impl.NewVariable(scope, initializer)
		field := binding.Field()
		field.Type = initializer.Type
		field.Value = (&tokens.Primary{Literal: &tokens.Literal{Symbol: initializer.Name}}).ToExpression()
		impl.NewVariable(scope, field)
	}
	ctx.Preparing = false
	for idx, h := range block.Handlers {
		expr := h.ToExpression()
		typ := impl.GetTypeOfExpression(expr, scope)
		valid := typ != nil && !typ.Pointer
		if valid {
			_, valid = impl.GetOperatorTraitName(typ.Type, "UnsafeHandler", scope)
		}
		if !valid {
			scope.ErrorScope.NewCompileTimeError("Unsafe Handler Error", "Handler must implement UnsafeHandler", h.Pos)
			return
		}
		typeName := strings.TrimSpace(impl.inferExpressionType(expr, scope))
		code := impl.ExpressionToCString(expr, scope)
		name := code
		if !isSimpleIdent(code) {
			name = fmt.Sprintf("__gecko_handler%d_%d", id, idx)
			info.Code += typeName + " " + name + " = " + code + ";\n"
		}
		ctx.Handlers = append(ctx.Handlers, &UnsafeHandlerInstance{VarName: name, TypeName: typeName, Index: idx})
	}
	for _, allocation := range ctx.Allocations {
		info.Code += impl.setupGuardChecks(scope, ctx, "alloc", allocation.Pointer, allocation.Size)
	}
	resultName := tokens.UnsafeBlockBindNames[block]
	emitBody := block.Body
	var trailing *tokens.Expression
	if resultName != "" {
		if len(block.Handlers) == 0 || len(block.Body) == 0 {
			scope.ErrorScope.NewCompileTimeError("Unsafe Setup Error", "An unsafe setup result requires handlers and a trailing value", block.Pos)
			return
		}
		last := block.Body[len(block.Body)-1]
		trailing = last.UnsafeValue()
		if trailing == nil {
			scope.ErrorScope.NewCompileTimeError("Unsafe Setup Error", "An unsafe setup result requires a trailing value", block.Pos)
			return
		}
		emitBody = block.Body[:len(block.Body)-1]
	}
	for _, entry := range emitBody {
		impl.processEntry(scope, entry)
	}
	declaration := ""
	if trailing != nil {
		declaration = impl.finishUnsafeSetupResult(parent, scope, block, ctx, trailing, resultName)
	} else {
		impl.emitDefers(scope, info)
		impl.generateDropCalls(scope, info)
		info.Code += ctx.FailureLabel + ":;\n"
	}
	info.Code += "}\n"
	outer.Code += declaration + info.Code
}

func (impl *CBackendImplementation) finishUnsafeSetupResult(parent, scope *ast.Ast, block *tokens.UnsafeBlock, ctx *unsafeSetupContext, trailing *tokens.Expression, resultName string) string {
	info := CGetScopeInformation(scope)
	valueType := impl.GetTypeOfExpression(trailing, scope)
	if valueType == nil || valueType.Type == "void" && !valueType.Pointer {
		scope.ErrorScope.NewCompileTimeError("Unsafe Setup Error", "Cannot infer the unsafe setup result value", block.Pos)
		return ""
	}
	enumName := tokens.UnsafeBlockErrorNames[block]
	if enumName == "" {
		enumName = fmt.Sprintf("__gecko_setup_error%d", unsafeBlockCounter)
	}
	registerUnsafeErrorEnum(scope, info, enumName, ctx.Handlers)
	resultType := &tokens.TypeRef{Type: "Result", TypeArgs: []*tokens.TypeRef{valueType, {Type: enumName}}}
	cType := TypeRefToCType(resultType, scope)
	value := impl.ExpressionToCString(trailing, scope)
	info.Code += resultName + ".value = " + value + ";\n" + resultName + ".is_ok = 1;\n" + resultName + ".active = 1;\n"
	impl.ApplyMoveFromExpression(trailing, scope)
	impl.emitDefers(scope, info)
	impl.generateDropCalls(scope, info)
	info.Code += ctx.FailureLabel + ":;\n"
	info.Code += "if (" + ctx.FailIndex + " >= 0) {\n" + resultName + ".is_ok = 0;\n" + resultName + ".active = 1;\n"
	info.Code += resultName + ".error = (" + enumName + ")" + ctx.FailIndex + ";\n}\n"
	v := ast.Variable{Name: resultName, Parent: parent}
	if dropMethodForType(resultType, parent) != "" {
		v.DropFlag = "__owned_" + v.GetFullName()
		CGetScopeInformation(parent).LocalVarOrder = append(CGetScopeInformation(parent).LocalVarOrder, resultName)
	}
	parent.Variables[resultName] = v
	(*CProgramValues)[v.GetFullName()] = &CValueInformation{CType: cType, GeckoType: resultType}
	declaration := cType + " " + resultName + " = {0};\n"
	if v.DropFlag != "" {
		declaration += "int " + v.DropFlag + " = 1;\n"
	}
	return declaration
}

func (impl *CBackendImplementation) setupGuardChecks(scope *ast.Ast, ctx *unsafeSetupContext, name, ptr, size string) string {
	var checks strings.Builder
	for _, h := range ctx.Handlers {
		if !handlerCovers(h.TypeName, name) {
			continue
		}
		guard := h.TypeName + "__UnsafeHandler__guard"
		catch := h.TypeName + "__UnsafeHandler__catch"
		fmt.Fprintf(&checks, "if (!%s(&%s, __op)) { %s(&%s, __op); __gecko_rejected = 1; if (%s < 0) %s = %d; }\n", guard, h.VarName, catch, h.VarName, ctx.FailIndex, ctx.FailIndex, h.Index)
	}
	if checks.Len() == 0 {
		return ""
	}
	cleanup := ""
	for current := scope; current != nil; current = current.Parent {
		info := CGetScopeInformation(current)
		for idx := len(info.DeferStack) - 1; idx >= 0; idx-- {
			cleanup += info.DeferStack[idx]
		}
		for idx := len(impl.getDroppableVariables(current)) - 1; idx >= 0; idx-- {
			d := impl.getDroppableVariables(current)[idx]
			cleanup += dropVariableCode(d)
		}
		if info.UnsafeSetup == ctx {
			break
		}
	}
	return "{ int __gecko_rejected = 0; " + unsafeOpCTypeName(scope) + " __op = { .ptr = (void*)(" + ptr + "), .size = (" + size + ") };\n" + checks.String() + "if (__gecko_rejected) {\n" + cleanup + "goto " + ctx.FailureLabel + ";\n}\n}\n"
}

func (impl *CBackendImplementation) intrinsicAlloc(i *tokens.Intrinsic, scope *ast.Ast) string {
	if ctx := findUnsafeSetup(scope); ctx != nil && ctx.Preparing && ctx.Producer != i {
		scope.ErrorScope.NewCompileTimeError("Unsafe Setup Error", "@alloc must be the entire setup initializer, optionally cast or parenthesized", i.Pos)
		return "0"
	}
	if len(i.Args) != 2 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@alloc requires size and alignment", i.Pos)
		return "0"
	}
	rootInfo := CGetScopeInformation(scope.GetRoot())
	rootInfo.Declarations = append(rootInfo.Declarations, "extern void* aligned_alloc(size_t, size_t);")
	unsafeHandlerCounter++
	prefix := fmt.Sprintf("__gecko_alloc%d", unsafeHandlerCounter)
	size, align, ptr := prefix+"_size", prefix+"_align", prefix+"_ptr"
	sizeExpr := impl.ExpressionToCString(i.Args[0], scope)
	alignExpr := impl.ExpressionToCString(i.Args[1], scope)
	code := fmt.Sprintf("size_t %s = (%s); size_t %s = (%s); void* %s = NULL;\n", size, sizeExpr, align, alignExpr, ptr)
	code += fmt.Sprintf("if (%s != 0 && (%s & (%s - 1)) == 0) {\n", align, align, align)
	code += fmt.Sprintf("if (%s < _Alignof(max_align_t)) %s = _Alignof(max_align_t);\n", align, align)
	code += fmt.Sprintf("if (%s <= SIZE_MAX - (%s - 1)) { size_t n = (%s + %s - 1) & ~(%s - 1); %s = aligned_alloc(%s, n ? n : %s); }\n}\n", size, align, size, align, align, ptr, align, align)
	if ctx := findUnsafeSetup(scope); ctx != nil && ctx.Preparing {
		CGetScopeInformation(scope).Code += code
		ctx.Allocations = append(ctx.Allocations, unsafeSetupOperation{Pointer: ptr, Size: size})
		return ptr
	}
	if ctx := findUnsafeSetup(scope); ctx != nil {
		code += impl.setupGuardChecks(scope, ctx, "alloc", ptr, size)
	}
	return "({ " + code + ptr + "; })"
}

func setupProducer(expr *tokens.Expression) *tokens.Intrinsic {
	if expr == nil || expr.Cond == nil || expr.Cond.TrueExpr != nil || expr.Cond.LogicalOr == nil || expr.Cond.LogicalOr.Or != nil {
		return nil
	}
	lo := expr.GetLogicalOr()
	if lo == nil || lo.Next != nil || lo.LogicalAnd == nil || lo.LogicalAnd.Next != nil {
		return nil
	}
	eq := lo.LogicalAnd.Equality
	if eq == nil || eq.Next != nil || eq.Comparison == nil || eq.Comparison.Next != nil {
		return nil
	}
	add := eq.Comparison.Addition
	if add == nil || add.Next != nil || add.Multiplication == nil || add.Multiplication.Next != nil {
		return nil
	}
	un := add.Multiplication.Unary
	if un == nil || un.Unary != nil || un.Primary == nil {
		return nil
	}
	if un.Primary.SubExpression != nil {
		return setupProducer(un.Primary.SubExpression)
	}
	if un.Primary.Literal != nil {
		return un.Primary.Literal.Intrinsic
	}
	return nil
}

func (impl *CBackendImplementation) guardedSetupIntrinsic(i *tokens.Intrinsic, scope *ast.Ast, ctx *unsafeSetupContext) string {
	if i.Name == "alloc" {
		return impl.intrinsicAlloc(i, scope)
	}
	counts := map[string]int{"drop_in_place": 1, "deref": 1, "read_volatile": 1, "write_volatile": 2, "free": 1, "copy": 3, "zero": 2, "ptr_add": 2, "ptr_sub": 2}
	count, known := counts[i.Name]
	if !known {
		checks := impl.setupGuardChecks(scope, ctx, i.Name, "0", "0")
		if i.Name == "trap" {
			return "({ " + checks + impl.intrinsicTrap(i, scope) + "; })"
		}
		return "({ " + checks + impl.intrinsicUnreachable(i, scope) + "; })"
	}
	if len(i.Args) != count {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", fmt.Sprintf("@%s requires %d arguments", i.Name, count), i.Pos)
		return "0"
	}
	if i.Store != nil || i.Name == "write_volatile" || i.Name == "zero" || i.Name == "copy" {
		CheckReadonlyStore(impl.GetTypeOfExpression(i.Args[0], scope), scope, i.Pos)
	}
	var code strings.Builder
	args := make([]string, len(i.Args))
	for idx, arg := range i.Args {
		args[idx] = fmt.Sprintf("__gecko_arg%d", idx)
		fmt.Fprintf(&code, "__auto_type %s = (%s);\n", args[idx], impl.ExpressionToCString(arg, scope))
	}
	ptr, size := args[0], "0"
	op := ""
	switch i.Name {
	case "deref", "read_volatile", "write_volatile":
		size = "sizeof(*" + ptr + ")"
		op = "*(" + ptr + ")"
		if i.Name != "deref" {
			op = "*(volatile typeof(*" + ptr + ")*)" + ptr
		}
		if i.Name == "write_volatile" {
			op += " = " + args[1]
		} else if i.Store != nil {
			op += " = " + impl.ExpressionToCString(i.Store, scope)
		}
	case "drop_in_place":
		size = "sizeof(*" + ptr + ")"
		t := impl.GetTypeOfExpression(i.Args[0], scope)
		if t == nil || !t.Pointer || t.Type == "void" {
			scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@drop_in_place requires a typed pointer", i.Pos)
			return "0"
		}
		CheckReadonlyStore(t, scope, i.Pos)
		op = dropInPlaceCode(t, ptr, scope)
	case "free":
		op = "__builtin_free(" + ptr + ")"
	case "zero":
		size = args[1]
		op = "__builtin_memset(" + ptr + ", 0, " + size + ")"
	case "copy":
		size = args[2]
		op = "__builtin_memcpy(" + ptr + ", " + args[1] + ", " + size + ")"
	case "ptr_add":
		op = ptr + " + " + args[1]
	case "ptr_sub":
		op = ptr + " - " + args[1]
	}
	code.WriteString(impl.setupGuardChecks(scope, ctx, i.Name, ptr, size))
	return "({ " + code.String() + op + "; })"
}
