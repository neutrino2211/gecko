// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// NewReturn generates a void return statement
func (impl *CBackendImplementation) NewReturn(scope *ast.Ast) {
	info := CGetScopeInformation(scope)
	// Emit deferred expressions before returning (in reverse order)
	impl.cleanupThrough(scope, info, false)
	info.Code += "    return;\n"
}

// emitDefers emits all deferred expressions from the scope's defer stack in reverse order.
func (impl *CBackendImplementation) emitDefers(scope *ast.Ast, info *CScopeInformation) {
	stack := CGetScopeInformation(scope).DeferStack
	for i := len(stack) - 1; i >= 0; i-- {
		info.Code += stack[i]
	}
}

// NewReturnLiteral generates a return statement with a value
func (impl *CBackendImplementation) NewReturnLiteral(scope *ast.Ast, literal *tokens.Expression) {
	info := CGetScopeInformation(scope)

	// Validate return type matches declared function return type
	impl.CheckReturnType(literal, scope)

	val := impl.ExpressionToCString(literal, scope)
	impl.ApplyMoveFromExpression(literal, scope)

	// Emit capture initialization for lambdas (must be after ExpressionToCString stores it)
	if lambda := literal.GetLambda(); lambda != nil {
		key := fmt.Sprintf("%d:%d", lambda.Pos.Line, lambda.Pos.Column)
		if initCode, ok := PendingCaptureInit[key]; ok && initCode != "" {
			info.Code += initCode
			delete(PendingCaptureInit, key)
		}
	}

	returnType := impl.inferExpressionType(literal, scope)
	if declared := getCurrentFuncReturnType(scope); declared != nil {
		returnType = TypeRefToCType(declared, scope)
	}
	if returnType == "" || returnType == "void" {
		impl.cleanupThrough(scope, info, false)
		info.Code += "    return " + val + ";\n"
		return
	}
	declaration := returnType + " __drop_ret_val"
	if IsFuncPointerType(returnType) {
		declaration = FormatFuncPointerDecl(returnType, "__drop_ret_val")
	}
	info.Code += fmt.Sprintf("    { %s = %s;\n", declaration, val)
	impl.cleanupThrough(scope, info, false)
	info.Code += "    return __drop_ret_val; }\n"

}

// getDroppableVariables returns variables in scope that implement Drop trait
type droppableVariable struct {
	Name       string
	DropMethod string
	ClassName  string
	Flag       string
}

func (impl *CBackendImplementation) getDroppableVariables(scope *ast.Ast) []droppableVariable {
	var droppables []droppableVariable
	if scope.Config != nil && scope.Config.ManualMemory {
		return droppables
	}

	for _, name := range CGetScopeInformation(scope).LocalVarOrder {
		variable, exists := scope.Variables[name]
		if !exists || variable.IsGlobal {
			continue
		}
		fullName := variable.GetFullName()
		if variable.DropFlag == "" && CurrentTypeState != nil && CurrentTypeState.IsMoved(fullName) {
			continue
		}
		valueInfo, ok := (*CProgramValues)[fullName]
		if !ok || valueInfo == nil {
			continue
		}
		method := dropMethodForType(valueInfo.GeckoType, scope)
		if method == "" {
			continue
		}
		droppables = append(droppables, droppableVariable{
			Name: CVariableIdentifier(&variable), DropMethod: method, ClassName: valueInfo.GeckoType.Type, Flag: variable.DropFlag,
		})
	}

	return droppables
}

// generateDropCalls generates drop method calls for all droppable variables
func (impl *CBackendImplementation) generateDropCalls(scope *ast.Ast, info *CScopeInformation) {
	droppables := impl.getDroppableVariables(scope)

	// Drop in reverse order of declaration (LIFO)
	for i := len(droppables) - 1; i >= 0; i-- {
		d := droppables[i]
		info.Code += dropVariableCode(d)
	}
}

// NewDefer handles defer statements by generating the expression
func (impl *CBackendImplementation) NewDefer(scope *ast.Ast, d *tokens.Defer) {
	info := CGetScopeInformation(scope)
	info.PreparingDefer = true
	expr := impl.ExpressionToCString(d.Expression, scope)
	info.PreparingDefer = false
	// Push deferred expression onto the scope's defer stack.
	// It will be emitted in reverse order at scope exit.
	info.DeferStack = append(info.DeferStack, fmt.Sprintf("    %s;\n", expr))
}

func dropVariableCode(d droppableVariable) string {
	call := fmt.Sprintf("%s(&%s);\n", d.DropMethod, d.Name)
	if d.Flag != "" {
		return "if (" + d.Flag + ") { " + d.Flag + " = 0; " + call + "}\n"
	}
	return call
}
