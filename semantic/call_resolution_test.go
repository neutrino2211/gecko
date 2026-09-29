package semantic

import (
	"testing"

	"github.com/neutrino2211/gecko/tokens"
)

func TestRejectedOverloadDoesNotPublishDiagnostic(t *testing.T) {
	program := NewProgram(&tokens.File{})
	program.functionsByName["choose"] = []*FunctionSignature{
		{SymbolID: 1, Name: "choose", Params: []*tokens.Value{{Name: "value", Type: &tokens.TypeRef{Type: "string"}}}, ReturnType: &tokens.TypeRef{Type: "string"}},
		{SymbolID: 2, Name: "choose", Params: []*tokens.Value{{Name: "value", Type: &tokens.TypeRef{Type: "int32"}}}, ReturnType: &tokens.TypeRef{Type: "int32"}},
	}
	call := &tokens.FuncCall{Function: "choose", Arguments: []*tokens.Argument{{Value: (&tokens.Primary{Literal: &tokens.Literal{Number: "7"}}).ToExpression()}}}
	analyzer := &analyzer{program: program}
	result := analyzer.inferFuncCall(call, newFlowEnv(), nil)
	if result == nil || result.Type != "int32" {
		t.Fatalf("selected wrong overload: %#v", result)
	}
	if diagnostics := program.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("rejected overload leaked diagnostics: %#v", diagnostics)
	}
	resolution := program.FuncCallResolution(call)
	if resolution == nil || resolution.Status != ResolutionResolved || resolution.CalleeID != 2 || resolution.Signature == nil || resolution.Signature.ReturnType.Type != "int32" {
		t.Fatalf("selected call facts missing: %#v", resolution)
	}
	if len(resolution.Arguments) != 1 || resolution.Arguments[0].ParameterIndex != 0 || resolution.Arguments[0].ParameterName != "value" || resolution.Arguments[0].ActualType.Type != "int32" || resolution.Arguments[0].ExpectedType.Type != "int32" {
		t.Fatalf("argument mapping missing: %#v", resolution.Arguments)
	}
	resolution.Arguments[0].ExpectedType.Type = "string"
	if program.FuncCallResolution(call).Arguments[0].ExpectedType.Type != "int32" {
		t.Fatal("call facts share mutable type data")
	}
	clonedCall := &tokens.FuncCall{}
	cloned := program.CloneForSyntax(&tokens.File{}, map[any]any{call: clonedCall})
	clonedResult := cloned.FuncCallResolution(clonedCall)
	if clonedResult == nil || clonedResult.CalleeID != 2 {
		t.Fatalf("cloned syntax lost selected call: %#v", clonedResult)
	}
	clonedResult.Signature.Params[0].Type.Type = "string"
	if program.FuncCallResolution(call).Signature.Params[0].Type.Type != "int32" {
		t.Fatal("cloned call shares selected signature")
	}
}

func TestCallOutcomesDistinguishInvalidAndIncomplete(t *testing.T) {
	program := NewProgram(&tokens.File{})
	program.functionsByName["known"] = []*FunctionSignature{{SymbolID: 1, Name: "known", Params: []*tokens.Value{{Name: "value", Type: &tokens.TypeRef{Type: "string"}}}, ReturnType: &tokens.TypeRef{Type: "void"}}}
	analyzer := &analyzer{program: program}
	argument := &tokens.Argument{Value: (&tokens.Primary{Literal: &tokens.Literal{Number: "7"}}).ToExpression()}
	invalid := &tokens.FuncCall{Function: "known", Arguments: []*tokens.Argument{argument}}
	incomplete := &tokens.FuncCall{Function: "missing"}
	analyzer.inferFuncCall(invalid, newFlowEnv(), nil)
	analyzer.inferFuncCall(incomplete, newFlowEnv(), nil)
	if result := program.FuncCallResolution(invalid); result == nil || result.Status != ResolutionInvalid {
		t.Fatalf("known invalid call has wrong outcome: %#v", result)
	}
	if result := program.FuncCallResolution(incomplete); result == nil || result.Status != ResolutionIncomplete {
		t.Fatalf("unresolved call has wrong outcome: %#v", result)
	}
	if len(program.Diagnostics()) != 1 {
		t.Fatalf("expected one invalid-call diagnostic, got %#v", program.Diagnostics())
	}
}

func TestFailedOverloadProbesDoNotPublishNestedCallFacts(t *testing.T) {
	program := NewProgram(&tokens.File{})
	program.functionsByName["make_zero"] = []*FunctionSignature{{
		SymbolID: 3, Name: "make_zero", TypeParams: []*tokens.TypeParam{{Name: "T"}}, ReturnType: &tokens.TypeRef{Type: "T"},
	}}
	program.functionsByName["choose"] = []*FunctionSignature{
		{SymbolID: 1, Name: "choose", Params: []*tokens.Value{{Name: "first", Type: &tokens.TypeRef{Type: "string"}}, {Name: "second", Type: &tokens.TypeRef{Type: "string"}}}, ReturnType: &tokens.TypeRef{Type: "void"}},
		{SymbolID: 2, Name: "choose", Params: []*tokens.Value{{Name: "first", Type: &tokens.TypeRef{Type: "bool"}}, {Name: "second", Type: &tokens.TypeRef{Type: "bool"}}}, ReturnType: &tokens.TypeRef{Type: "void"}},
	}
	inner := &tokens.FuncCall{Function: "make_zero"}
	outer := &tokens.FuncCall{Function: "choose", Arguments: []*tokens.Argument{
		{Value: (&tokens.Primary{Literal: &tokens.Literal{FuncCall: inner}}).ToExpression()},
		{Value: (&tokens.Primary{Literal: &tokens.Literal{Number: "7"}}).ToExpression()},
	}}
	analyzer := &analyzer{program: program}
	analyzer.inferFuncCall(outer, newFlowEnv(), nil)
	if result := program.FuncCallResolution(outer); result == nil || result.Status != ResolutionInvalid {
		t.Fatalf("outer call should be invalid: %#v", result)
	}
	if result := program.FuncCallResolution(inner); result != nil {
		t.Fatalf("rejected candidate published nested call: %#v", result)
	}
	if len(program.Diagnostics()) != 1 {
		t.Fatalf("expected one selected failure, got %#v", program.Diagnostics())
	}
}

func TestClonedExpressionOutcomeIsIndependent(t *testing.T) {
	expression := &tokens.Expression{}
	clonedExpression := &tokens.Expression{}
	program := NewProgram(&tokens.File{})
	program.expressionOutcomes[expression] = &ExpressionOutcome{Status: ResolutionInvalid}
	cloned := program.CloneForSyntax(&tokens.File{}, map[any]any{expression: clonedExpression})
	if outcome := cloned.ExpressionOutcome(clonedExpression); outcome == nil || outcome.Status != ResolutionInvalid {
		t.Fatalf("cloned expression lost outcome: %#v", outcome)
	}
	cloned.expressionOutcomes[clonedExpression].Status = ResolutionResolved
	if outcome := program.ExpressionOutcome(expression); outcome == nil || outcome.Status != ResolutionInvalid {
		t.Fatalf("cloned view changed cached outcome: %#v", outcome)
	}
}
