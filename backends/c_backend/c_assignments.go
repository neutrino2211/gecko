// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// NewAssignment handles variable assignment
func (impl *CBackendImplementation) NewAssignment(scope *ast.Ast, a *tokens.Assignment) {
	// Type check the assignment
	impl.CheckAssignmentType(a, scope)

	info := CGetScopeInformation(scope)
	resolveScope := scope
	if a.Global {
		resolveScope = scope.GetRoot()
	}

	// Resolve the variable
	varOpt := resolveScope.ResolveSymbolAsVariable(a.Name)
	varName := a.Name
	isPointer := false
	if !varOpt.IsNil() {
		variable := varOpt.Unwrap()
		isPointer = variable.IsPointer
		info := CGetScopeInformation(scope)
		varName = resolveVariableWithIdentifier(variable, scope, info)
	} else if a.Global {
		scope.ErrorScope.NewCompileTimeError("Assignment Error", "unable to resolve global variable '"+a.Name+"'", a.Pos)
		return
	}

	// Infer lambda param types from variable's declared type
	if a.Value != nil {
		if lambda := a.Value.GetLambda(); lambda != nil && !varOpt.IsNil() {
			variable := varOpt.Unwrap()
			fullName := variable.GetFullName()
			if valInfo, ok := (*CProgramValues)[fullName]; ok && valInfo != nil && valInfo.GeckoType != nil && valInfo.GeckoType.FuncType != nil {
				InferLambdaTypes(lambda, valInfo.GeckoType.FuncType)
			}
		}
	}

	// `@unsafe with ... { }` used as the RHS of an assignment reuses an already
	// declared variable: emit the block with a fresh temp Result holder, then
	// assign that temp into the target (so the variable is not redeclared).
	if ub := a.Value.GetUnsafeBlock(); ub != nil {
		unsafeBlockCounter++
		tmpName := fmt.Sprintf("__gecko_unsafe_res%d", unsafeBlockCounter)
		tokens.UnsafeBlockBindNames[ub] = tmpName
		impl.NewUnsafeBlock(scope, ub)
		info.Code += "    " + varName + " = " + tmpName + ";\n"
		return
	}

	value := impl.ExpressionToCString(a.Value, scope)

	// Check if we're assigning a lambda with captures - emit capture initialization
	// This must happen AFTER ExpressionToCString because that's where PendingCaptureInit is populated
	if a.Value != nil {
		if lambda := a.Value.GetLambda(); lambda != nil {
			key := fmt.Sprintf("%d:%d", lambda.Pos.Line, lambda.Pos.Column)
			if initCode, ok := PendingCaptureInit[key]; ok && initCode != "" {
				info.Code += initCode
				delete(PendingCaptureInit, key)
			}
		}
	}

	applyMoveState := func() {
		// RHS ownership transfer happens after expression lowering.
		impl.ApplyMoveFromExpression(a.Value, scope)

		// Direct variable assignment reinitializes the target binding.
		if a.Field == "" && a.Index == nil && CurrentTypeState != nil {
			if varOpt := resolveScope.ResolveSymbolAsVariable(a.Name); !varOpt.IsNil() {
				CurrentTypeState.ClearMoved(varOpt.Unwrap().GetFullName())
			}
		}
	}

	// Determine the assignment operator
	assignOp := "="
	if a.Op != "" {
		assignOp = a.Op
	}

	// Handle field assignment (e.g., p.x = value)
	if a.Field != "" {
		accessor := "."
		if isPointer {
			if varOpt.IsNil() || !varOpt.Unwrap().IsReceiver {
				requireUnsafe(scope, a.Pos, "raw pointer field store")
			}
			accessor = "->"
		}
		if a.Index != nil {
			indexed := &tokens.Literal{SymbolModule: a.Name, Symbol: a.Field}
			if t := impl.GetTypeOfLiteral(indexed, scope); t != nil && t.Pointer {
				requireUnsafe(scope, a.Pos, "raw pointer indexed store")
			}
			indexStr := impl.ExpressionToCString(a.Index, scope)
			info.Code += fmt.Sprintf("    %s%s%s[%s] %s %s;\n", varName, accessor, a.Field, indexStr, assignOp, value)
		} else {
			info.Code += fmt.Sprintf("    %s%s%s %s %s;\n", varName, accessor, a.Field, assignOp, value)
		}
		applyMoveState()
		return
	}

	// Handle indexed assignment (e.g., arr[0] = value)
	if a.Index != nil {
		if isPointer {
			requireUnsafe(scope, a.Pos, "raw pointer indexed store")
		}
		indexStr := impl.ExpressionToCString(a.Index, scope)
		info.Code += fmt.Sprintf("    %s[%s] %s %s;\n", varName, indexStr, assignOp, value)
		applyMoveState()
		return
	}

	if !varOpt.IsNil() && varOpt.Unwrap().DropFlag != "" && assignOp == "=" {
		variable := varOpt.Unwrap()
		if source, ok := extractPlainSymbolFromExpression(a.Value); ok && source == a.Name {
			return
		}
		info.Code += "{ __auto_type __replacement = (" + value + ");\n"
		applyMoveState()
		for _, d := range impl.getDroppableVariables(variable.Parent) {
			if d.Name == varName {
				info.Code += dropVariableCode(d)
				break
			}
		}
		info.Code += varName + " = __replacement;\n" + variable.DropFlag + " = 1;\n}\n"
		return
	}
	info.Code += fmt.Sprintf("    %s %s %s;\n", varName, assignOp, value)
	applyMoveState()
}
