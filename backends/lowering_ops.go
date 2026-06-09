// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

// LoweredOperationKind identifies a normalized, backend-agnostic lowered op.
type LoweredOperationKind string

const (
	LoweredOpMethod         LoweredOperationKind = "method"
	LoweredOpField          LoweredOperationKind = "field"
	LoweredOpClass          LoweredOperationKind = "class"
	LoweredOpImplementation LoweredOperationKind = "implementation"
	LoweredOpTrait          LoweredOperationKind = "trait"
	LoweredOpEnum           LoweredOperationKind = "enum"
	LoweredOpDeclaration    LoweredOperationKind = "declaration"
	LoweredOpIntrinsic      LoweredOperationKind = "intrinsic"
	LoweredOpMethodCall     LoweredOperationKind = "method_call"
	LoweredOpFuncCall       LoweredOperationKind = "func_call"
	LoweredOpReturnLiteral  LoweredOperationKind = "return_literal"
	LoweredOpReturn         LoweredOperationKind = "return"
	LoweredOpIf             LoweredOperationKind = "if"
	LoweredOpLoop           LoweredOperationKind = "loop"
	LoweredOpAssignment     LoweredOperationKind = "assignment"
	LoweredOpAsm            LoweredOperationKind = "asm"
	LoweredOpBreak          LoweredOperationKind = "break"
	LoweredOpContinue       LoweredOperationKind = "continue"
	LoweredOpCImport        LoweredOperationKind = "cimport"
	LoweredOpForeign        LoweredOperationKind = "foreign"
)

// LoweredOperation is the shared lowering payload that backend emitters consume.
type LoweredOperation struct {
	Kind           LoweredOperationKind
	Method         *tokens.Method
	Field          *tokens.Field
	Class          *tokens.Class
	Implementation *tokens.Implementation
	Trait          *tokens.Trait
	Enum           *tokens.Enum
	Declaration    *tokens.Declaration
	Intrinsic      *tokens.Intrinsic
	MethodCall     *tokens.MethodCall
	FuncCall       *tokens.FuncCall
	ReturnLiteral  *tokens.Expression
	If             *tokens.If
	Loop           *tokens.Loop
	Assignment     *tokens.Assignment
	Asm            *tokens.Asm
	CImport        *tokens.CImport
	Foreign        *tokens.Foreign
}

func lowerEntry(entry *tokens.Entry) (LoweredOperation, bool) {
	if entry == nil {
		return LoweredOperation{}, false
	}

	switch {
	case entry.Method != nil:
		return LoweredOperation{Kind: LoweredOpMethod, Method: entry.Method}, true
	case entry.Field != nil:
		return LoweredOperation{Kind: LoweredOpField, Field: entry.Field}, true
	case entry.Class != nil:
		return LoweredOperation{Kind: LoweredOpClass, Class: entry.Class}, true
	case entry.Implementation != nil:
		return LoweredOperation{Kind: LoweredOpImplementation, Implementation: entry.Implementation}, true
	case entry.Trait != nil:
		return LoweredOperation{Kind: LoweredOpTrait, Trait: entry.Trait}, true
	case entry.Enum != nil:
		return LoweredOperation{Kind: LoweredOpEnum, Enum: entry.Enum}, true
	case entry.Declaration != nil:
		return LoweredOperation{Kind: LoweredOpDeclaration, Declaration: entry.Declaration}, true
	case entry.Intrinsic != nil:
		return LoweredOperation{Kind: LoweredOpIntrinsic, Intrinsic: entry.Intrinsic}, true
	case entry.MethodCall != nil:
		return LoweredOperation{Kind: LoweredOpMethodCall, MethodCall: entry.MethodCall}, true
	case entry.FuncCall != nil:
		return LoweredOperation{Kind: LoweredOpFuncCall, FuncCall: entry.FuncCall}, true
	case entry.Return != nil:
		return LoweredOperation{Kind: LoweredOpReturnLiteral, ReturnLiteral: entry.Return}, true
	case entry.VoidReturn != nil:
		return LoweredOperation{Kind: LoweredOpReturn}, true
	case entry.If != nil:
		return LoweredOperation{Kind: LoweredOpIf, If: entry.If}, true
	case entry.Loop != nil:
		return LoweredOperation{Kind: LoweredOpLoop, Loop: entry.Loop}, true
	case entry.Assignment != nil:
		return LoweredOperation{Kind: LoweredOpAssignment, Assignment: entry.Assignment}, true
	case entry.Asm != nil:
		return LoweredOperation{Kind: LoweredOpAsm, Asm: entry.Asm}, true
	case entry.Break != nil:
		return LoweredOperation{Kind: LoweredOpBreak}, true
	case entry.Continue != nil:
		return LoweredOperation{Kind: LoweredOpContinue}, true
	case entry.CImport != nil:
		return LoweredOperation{Kind: LoweredOpCImport, CImport: entry.CImport}, true
	case entry.Foreign != nil:
		return LoweredOperation{Kind: LoweredOpForeign, Foreign: entry.Foreign}, true
	default:
		return LoweredOperation{}, false
	}
}

func lowerEntries(entries []*tokens.Entry) []LoweredOperation {
	lowered := make([]LoweredOperation, 0, len(entries))
	for _, entry := range entries {
		if op, ok := lowerEntry(entry); ok {
			lowered = append(lowered, op)
		}
	}
	return lowered
}

// LoweredOperationEmitter emits lowered operations to a target backend representation.
type LoweredOperationEmitter interface {
	EmitLoweredOperation(scope *ast.Ast, op LoweredOperation)
}

// compatibilityEmitter adapts the current BackendCodegenImplementations contract.
type compatibilityEmitter struct {
	impl interfaces.BackendCodegenImplementations
}

func newCompatibilityEmitter(impl interfaces.BackendCodegenImplementations) LoweredOperationEmitter {
	return &compatibilityEmitter{impl: impl}
}

func (e *compatibilityEmitter) EmitLoweredOperation(scope *ast.Ast, op LoweredOperation) {
	switch op.Kind {
	case LoweredOpMethod:
		e.impl.NewMethod(scope, op.Method)
	case LoweredOpField:
		e.impl.NewVariable(scope, op.Field)
	case LoweredOpClass:
		e.impl.NewClass(scope, op.Class)
	case LoweredOpImplementation:
		e.impl.NewImplementation(scope, op.Implementation)
	case LoweredOpTrait:
		e.impl.NewTrait(scope, op.Trait)
	case LoweredOpEnum:
		e.impl.NewEnum(scope, op.Enum)
	case LoweredOpDeclaration:
		e.impl.NewDeclaration(scope, op.Declaration)
	case LoweredOpIntrinsic:
		e.impl.IntrinsicStatement(scope, op.Intrinsic)
	case LoweredOpMethodCall:
		e.impl.MethodCall(scope, op.MethodCall)
	case LoweredOpFuncCall:
		e.impl.FuncCall(scope, op.FuncCall)
	case LoweredOpReturnLiteral:
		e.impl.NewReturnLiteral(scope, op.ReturnLiteral)
	case LoweredOpReturn:
		e.impl.NewReturn(scope)
	case LoweredOpIf:
		e.impl.NewIf(scope, op.If)
	case LoweredOpLoop:
		e.impl.NewLoop(scope, op.Loop)
	case LoweredOpAssignment:
		e.impl.NewAssignment(scope, op.Assignment)
	case LoweredOpAsm:
		e.impl.NewAsm(scope, op.Asm)
	case LoweredOpBreak:
		e.impl.NewBreak(scope)
	case LoweredOpContinue:
		e.impl.NewContinue(scope)
	case LoweredOpCImport:
		e.impl.NewCImport(scope, op.CImport)
	case LoweredOpForeign:
		e.impl.NewForeign(scope, op.Foreign)
	}
}

// SharedLoweringPipeline lowers entries into normalized operations and emits them.
type SharedLoweringPipeline struct {
	emitter LoweredOperationEmitter
}

func newSharedLoweringPipeline(emitter LoweredOperationEmitter) *SharedLoweringPipeline {
	return &SharedLoweringPipeline{emitter: emitter}
}

func (p *SharedLoweringPipeline) EmitEntries(scope *ast.Ast, entries []*tokens.Entry) {
	ops := lowerEntries(entries)
	for _, op := range ops {
		if op.Kind == LoweredOpTrait && op.Trait != nil {
			hooks.ProcessTraitHooks(op.Trait, scope.Scope, scope.ErrorScope)
		}
		p.emitter.EmitLoweredOperation(scope, op)
	}
}
