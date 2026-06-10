package semantic_test

import (
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

func analyzeSource(t *testing.T, src string) (*tokens.File, *semantic.Program) {
	t.Helper()
	file, err := parser.Parser.ParseString("test.gecko", src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	file.Path = "test.gecko"
	file.Content = src
	file.ComputeRanges()
	return file, semantic.Analyze(file)
}

func findFirstFuncCall(file *tokens.File, name string) *tokens.FuncCall {
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		if entry.Method != nil {
			if call := findFuncCallInEntries(entry.Method.Value, name); call != nil {
				return call
			}
		}
	}
	return nil
}

func findFuncCallInEntries(entries []*tokens.Entry, name string) *tokens.FuncCall {
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.Field != nil && entry.Field.Value != nil {
			if call := funcCallFromExpression(entry.Field.Value, name); call != nil {
				return call
			}
		}
		if entry.Return != nil {
			if call := funcCallFromExpression(entry.Return, name); call != nil {
				return call
			}
		}
		if entry.If != nil {
			if call := findFuncCallInEntries(entry.If.Value, name); call != nil {
				return call
			}
			if entry.If.Else != nil {
				if call := findFuncCallInEntries(entry.If.Else.Value, name); call != nil {
					return call
				}
			}
		}
	}
	return nil
}

func funcCallFromExpression(expr *tokens.Expression, name string) *tokens.FuncCall {
	if expr == nil || expr.GetLogicalOr() == nil {
		return nil
	}
	lo := expr.GetLogicalOr()
	if lo.LogicalAnd == nil || lo.LogicalAnd.Equality == nil || lo.LogicalAnd.Equality.Comparison == nil || lo.LogicalAnd.Equality.Comparison.Addition == nil || lo.LogicalAnd.Equality.Comparison.Addition.Multiplication == nil || lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary == nil || lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary == nil {
		return nil
	}
	primary := lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary
	if primary.Literal == nil || primary.Literal.FuncCall == nil {
		return nil
	}
	if primary.Literal.FuncCall.Function != name {
		return nil
	}
	return primary.Literal.FuncCall
}

func hasDiagKind(diags []semantic.Diagnostic, kind semantic.DiagnosticKind) bool {
	for _, diag := range diags {
		if diag.Kind == kind {
			return true
		}
	}
	return false
}

func TestGenericInferenceFromExpectedContext(t *testing.T) {
	src := `
package main

func make_zero<T>(): T {
    let x: T
    return x
}

external func main(): int32 {
    let value: int32 = make_zero()
    return value
}
`
	file, graph := analyzeSource(t, src)
	call := findFirstFuncCall(file, "make_zero")
	if call == nil {
		t.Fatal("expected make_zero() call")
	}
	res := graph.FuncCallResolution(call)
	if res == nil {
		t.Fatal("expected semantic call resolution")
	}
	arg := res.InferredTypeArgs["T"]
	if arg == nil || arg.Type != "int32" {
		t.Fatalf("expected T to infer to int32, got %#v", arg)
	}
}

func TestGenericInferenceAmbiguityDiagnostic(t *testing.T) {
	src := `
package main

func make_zero<T>(): T {
    let x: T
    return x
}

external func main(): int32 {
    let value = make_zero()
    return 0
}
`
	_, graph := analyzeSource(t, src)
	diags := graph.Diagnostics()
	if !hasDiagKind(diags, semantic.DiagnosticInferenceAmbiguity) {
		t.Fatalf("expected inference ambiguity diagnostic, got %#v", diags)
	}
	foundHelp := false
	for _, diag := range diags {
		if diag.Kind == semantic.DiagnosticInferenceAmbiguity && strings.Contains(diag.Help, "<...>") {
			foundHelp = true
			break
		}
	}
	if !foundHelp {
		t.Fatalf("expected ambiguity diagnostic help suggesting explicit type args, got %#v", diags)
	}
}

func TestTraitConstraintFailureDiagnostic(t *testing.T) {
	src := `
package main

trait Shape {
    func area(self): int32
}

class Point {
    let x: int32
}

func takes_shape<T is Shape>(v: T): int32 {
    return 1
}

external func main(): int32 {
    let p: Point
    let out = takes_shape(p)
    return out
}
`
	_, graph := analyzeSource(t, src)
	if !hasDiagKind(graph.Diagnostics(), semantic.DiagnosticConstraintFailure) {
		t.Fatalf("expected generic constraint failure diagnostic, got %#v", graph.Diagnostics())
	}
}

func TestNarrowingAfterEarlyReturn(t *testing.T) {
	src := `
package main

func require_nonnull(p: int32*!): int32 {
    return 1
}

func takes(ptr: int32*): int32 {
    if (ptr == nil) {
        return 0
    }
    return require_nonnull(ptr)
}
`
	_, graph := analyzeSource(t, src)
	if len(graph.Diagnostics()) > 0 {
		t.Fatalf("expected no diagnostics for post-return narrowing, got %#v", graph.Diagnostics())
	}
}
