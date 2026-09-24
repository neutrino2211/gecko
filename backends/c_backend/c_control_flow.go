// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

// NewIf handles if statements (basic implementation for now)
func (impl *CBackendImplementation) NewIf(scope *ast.Ast, i *tokens.If) {
	info := CGetScopeInformation(scope)

	condition := impl.ExpressionToCString(i.Expression, scope)
	info.Code += fmt.Sprintf("    if (%s) {\n", condition)

	var savedTypeState *ast.TypeState
	var semanticFactsThenApplied bool
	var semanticAfterFacts *semantic.FlowFacts

	// Prefer frontend semantic facts when available.
	if CurrentSemanticProgram != nil {
		ifFacts := CurrentSemanticProgram.IfFlowFacts(i)
		if ifFacts != nil {
			savedTypeState = CurrentTypeState
			if savedTypeState != nil {
				CurrentTypeState = savedTypeState.Fork(1)
			} else {
				CurrentTypeState = ast.NewTypeState()
			}
			applySemanticFlowFacts(CurrentTypeState, ifFacts.Then)
			semanticFactsThenApplied = true
			semanticAfterFacts = ifFacts.After
		}
	}

	// Fallback to legacy local null-check narrowing when semantic facts are unavailable.
	nullCheck := DetectNullCheck(i.Expression)
	if !semanticFactsThenApplied && nullCheck != nil {
		checkType := "== nil"
		if nullCheck.IsNotNull {
			checkType = "!= nil"
		}
		narrowingDebug(fmt.Sprintf("Detected null check in if condition: %s %s", nullCheck.VarName, checkType))

		// Save current type state and create a forked state for the if-body
		savedTypeState = CurrentTypeState
		if savedTypeState != nil {
			CurrentTypeState = savedTypeState.Fork(1) // Path 1 = then branch
		} else {
			CurrentTypeState = ast.NewTypeState()
		}

		// Apply narrowing: if condition is "x != nil", mark x as non-null in the if-body
		ApplyNullNarrowing(CurrentTypeState, nullCheck, scope)
	}

	// Process if body with the narrowed type state
	impl.processScopedEntries(scope, i.Value, false)

	// Restore the original type state after processing the if-body
	if semanticFactsThenApplied || nullCheck != nil {
		CurrentTypeState = savedTypeState
	}

	info.Code += "    }\n"

	// Handle else if
	if i.ElseIf != nil {
		impl.NewElseIf(scope, i.ElseIf)
	}

	// Handle else
	if i.Else != nil {
		impl.NewElse(scope, i.Else)
	}

	// Re-apply semantic post-dominator facts after completing the full if/else chain.
	if semanticAfterFacts != nil {
		if CurrentTypeState == nil {
			CurrentTypeState = ast.NewTypeState()
		}
		applySemanticFlowFacts(CurrentTypeState, semanticAfterFacts)
	}
}

// NewElseIf handles else if statements
func (impl *CBackendImplementation) NewElseIf(scope *ast.Ast, ei *tokens.ElseIf) {
	info := CGetScopeInformation(scope)

	condition := impl.ExpressionToCString(ei.Expression, scope)
	info.Code += fmt.Sprintf("    else if (%s) {\n", condition)

	// Process else if body
	impl.processScopedEntries(scope, ei.Value, false)

	info.Code += "    }\n"

	// Handle chained else if
	if ei.ElseIf != nil {
		impl.NewElseIf(scope, ei.ElseIf)
	}

	// Handle else
	if ei.Else != nil {
		impl.NewElse(scope, ei.Else)
	}
}

// NewElse handles else statements
func (impl *CBackendImplementation) NewElse(scope *ast.Ast, e *tokens.Else) {
	info := CGetScopeInformation(scope)

	info.Code += "    else {\n"

	// Process else body
	impl.processScopedEntries(scope, e.Value, false)

	info.Code += "    }\n"
}

// NewLoop handles loop statements (basic implementation)
func (impl *CBackendImplementation) NewLoop(scope *ast.Ast, l *tokens.Loop) {
	info := CGetScopeInformation(scope)

	if l.ForExpression != nil {
		// for condition style: for condition { ... }
		condition := impl.ExpressionToCString(l.ForExpression, scope)
		info.Code += fmt.Sprintf("    while (%s) {\n", condition)
	} else if l.WhileExpr != nil {
		// while condition style: while condition { ... }
		condition := impl.ExpressionToCString(l.WhileExpr, scope)
		info.Code += fmt.Sprintf("    while (%s) {\n", condition)
	} else if l.ForOf != nil {
		// for-of loop: iterate over array
		// This is a simplified implementation
		info.Code += "    /* for-of loop not fully implemented */\n"
		info.Code += "    {\n"
	} else if l.ForIn != nil {
		// for-in loop: for x in iterator { ... }
		// Desugars to: while (iter.has_next()) { let x = iter.next(); ... }
		impl.generateForInLoop(scope, l)
		return
	} else {
		// Infinite loop
		info.Code += "    while (1) {\n"
	}

	// Process loop body
	impl.processScopedEntries(scope, l.Value, true)

	info.Code += "    }\n"
}

// NewIncDec handles i++ and i-- statements
func (impl *CBackendImplementation) NewIncDec(scope *ast.Ast, incDec *tokens.IncDec) {
	info := CGetScopeInformation(scope)
	variable := scope.ResolveSymbolAsVariable(incDec.Target)
	if variable.IsNil() {
		return
	}
	v := variable.Unwrap()
	varName := CVariableIdentifier(v)
	if v.IsGlobal {
		varName = v.GetFullName()
	}
	op := "+"
	if incDec.Op == "--" {
		op = "-"
	}
	info.Code += fmt.Sprintf("    %s = %s %s 1;\n", varName, varName, op)
}

// NewAsm handles inline assembly statements
func (impl *CBackendImplementation) NewAsm(scope *ast.Ast, asm *tokens.Asm) {
	info := CGetScopeInformation(scope)

	// Strip quotes from the assembly code string (participle captures them)
	asmCode := asm.Code
	if len(asmCode) >= 2 && asmCode[0] == '"' && asmCode[len(asmCode)-1] == '"' {
		asmCode = asmCode[1 : len(asmCode)-1]
	}

	// Generate GNU-style inline assembly with volatile qualifier
	info.Code += fmt.Sprintf("    __asm__ volatile (\"%s\");\n", asmCode)
}

// NewBreak generates a break statement
func (impl *CBackendImplementation) NewBreak(scope *ast.Ast) {
	info := CGetScopeInformation(scope)
	impl.cleanupThrough(scope, info, true)
	info.Code += "    break;\n"
}

// NewContinue generates a continue statement
func (impl *CBackendImplementation) NewContinue(scope *ast.Ast) {
	info := CGetScopeInformation(scope)
	impl.cleanupThrough(scope, info, true)
	info.Code += "    continue;\n"
}
