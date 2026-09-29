// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic_test

import (
	"strings"
	"testing"

	"github.com/neutrino2211/gecko/errors"
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
	if res.Status != semantic.ResolutionResolved || res.Signature == nil || res.Signature.ReturnType == nil || res.Signature.ReturnType.Type != "int32" {
		t.Fatalf("expected substituted signature, got %#v", res)
	}
}

func TestCallArgumentCountDiagnostics(t *testing.T) {
	src := `
package main

func add(left: int32, right: int32): int32 {
    return left + right
}

external func main(): int32 {
    let good: int32 = add(1, 2)
    let short: int32 = add(1)
    let long: int32 = add(1, 2, 3)
    return 0
}
`
	_, graph := analyzeSource(t, src)
	var count []semantic.Diagnostic
	for _, diag := range graph.Diagnostics() {
		if diag.Kind == semantic.DiagnosticArgumentCount {
			count = append(count, diag)
		}
	}
	if len(count) != 2 {
		t.Fatalf("expected both call arity diagnostics, got %#v", graph.Diagnostics())
	}
	if len(graph.Diagnostics()) != 2 {
		t.Fatalf("expected only call arity diagnostics, got %#v", graph.Diagnostics())
	}
	if count[0].Pos.Line == 0 || count[1].Pos.Line == 0 || count[0].Pos.Line == count[1].Pos.Line {
		t.Fatalf("expected distinct call positions, got %#v", count)
	}
	for index, want := range []string{"add(1)", "add(1, 2, 3)"} {
		diag := count[index]
		if diag.Code != errors.CodeArgumentCount || src[diag.Pos.Offset:diag.EndOffset] != want {
			t.Fatalf("call diagnostic %d has wrong metadata: %#v", index, diag)
		}
	}
}

func TestExpressionOutcomesDistinguishInvalidAndIncomplete(t *testing.T) {
	src := `package main
func answer(): int32 { return 42 }
external func main(): int32 {
    let good = answer()
    let bad = answer(1)
    let unknown = missing()
    return good
}`
	file, graph := analyzeSource(t, src)
	var fields []*tokens.Field
	for _, entry := range file.Entries {
		if entry.Method == nil || entry.Method.Name != "main" {
			continue
		}
		for _, bodyEntry := range entry.Method.Value {
			if bodyEntry.Field != nil {
				fields = append(fields, bodyEntry.Field)
			}
		}
	}
	if len(fields) != 3 {
		t.Fatalf("expected three expression fields, got %d", len(fields))
	}
	for index, want := range []semantic.ResolutionStatus{semantic.ResolutionResolved, semantic.ResolutionInvalid, semantic.ResolutionIncomplete} {
		outcome := graph.ExpressionOutcome(fields[index].Value)
		if outcome == nil || outcome.Status != want {
			t.Fatalf("expression %d: want %s, got %#v", index, want, outcome)
		}
	}
}

func TestMethodArgumentCountDiagnostics(t *testing.T) {
	src := `
package main

class Counter {
    let value: int32
    func add(self, amount: int32): int32 { return self.value + amount }
    func make(seed: int32): Counter { return Counter { value: seed } }
}

external func main(): int32 {
    let counter: Counter = Counter::make(2)
    let good: int32 = counter.add(3)
    let short: int32 = counter.add()
    let long: int32 = counter.add(3, 4)
    let missing: Counter = Counter::make()
    return 0
}
`
	_, graph := analyzeSource(t, src)
	var count []semantic.Diagnostic
	for _, diag := range graph.Diagnostics() {
		if diag.Kind == semantic.DiagnosticArgumentCount {
			count = append(count, diag)
		}
	}
	if len(count) != 3 || len(graph.Diagnostics()) != 3 {
		t.Fatalf("expected three method arity diagnostics, got %#v", graph.Diagnostics())
	}
	for index, diag := range count {
		if diag.Pos.Line != index+13 {
			t.Fatalf("method diagnostic %d has wrong position: %#v", index, diag)
		}
		if !strings.Contains(diag.Message, "1 argument,") {
			t.Fatalf("method diagnostic %d has wrong count: %#v", index, diag)
		}
		want := []string{"counter.add()", "counter.add(3, 4)", "Counter::make()"}[index]
		if diag.Code != errors.CodeArgumentCount || src[diag.Pos.Offset:diag.EndOffset] != want {
			t.Fatalf("method diagnostic %d has wrong metadata: %#v", index, diag)
		}
	}
}

func TestMethodInComparisonKeepsReceiver(t *testing.T) {
	src := `
package main

class View {
    let value: int32
    func get(self): int32 { return self.value }
}

external func main(): int32 {
    let view: View = View { value: 17 }
    if (view.get() != 17) { return 1 }
    return 0
}
`
	file, graph := analyzeSource(t, src)
	if len(graph.Diagnostics()) != 0 {
		t.Fatalf("unexpected method diagnostics: %#v", graph.Diagnostics())
	}
	var call *tokens.FuncCall
	tokens.WalkSyntaxEntries(file.Entries, func(node any) {
		if candidate, ok := node.(*tokens.FuncCall); ok && candidate.Function == "get" {
			call = candidate
		}
	})
	if call == nil || graph.FuncCallResolution(call) == nil {
		t.Fatal("comparison method call did not resolve")
	}
	if resolution := graph.FuncCallResolution(call); resolution.Status != semantic.ResolutionResolved || resolution.ReceiverType == nil || resolution.ReceiverType.Type != "View" || len(resolution.Arguments) != 0 {
		t.Fatalf("method receiver facts missing: %#v", resolution)
	}
}

func TestMethodStatementRetainsSelectedCallFacts(t *testing.T) {
	src := `package main
class Counter {
    let value: int32
    func add(self, amount: int32): void { self.value = self.value + amount }
}
external func main(): int32 {
    let counter: Counter = Counter { value: 1 }
    counter.add(2)
    return counter.value
}`
	file, graph := analyzeSource(t, src)
	var statement *tokens.FuncCall
	tokens.WalkSyntaxEntries(file.Entries, func(node any) {
		if call, ok := node.(*tokens.FuncCall); ok && call.Function == "add" {
			statement = call
		}
	})
	resolution := graph.FuncCallResolution(statement)
	if resolution == nil || resolution.Status != semantic.ResolutionResolved || resolution.ReceiverType == nil || resolution.ReceiverType.Type != "Counter" {
		t.Fatalf("method statement lost receiver or selection: statement=%#v resolution=%#v", statement, resolution)
	}
	if len(resolution.Arguments) != 1 || resolution.Arguments[0].ParameterIndex != 1 || resolution.Arguments[0].ExpectedType.Type != "int32" {
		t.Fatalf("method argument mapping is wrong: %#v", resolution.Arguments)
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

func TestFieldOccurrencesKeepClassIdentity(t *testing.T) {
	src := "package test\nclass A { let value: int32 }\nclass B { let value: int32 }\nfunc main(): void {\n    let a: A = A { value: 1 }\n    let b: B = B { value: 2 }\n    let x: int32 = a.value\n    b.value = 3\n}\n"
	_, graph := analyzeSource(t, src)
	lookup := func(offset int) semantic.Occurrence {
		t.Helper()
		occurrence, ok := graph.OccurrenceAt("test.gecko", offset)
		if !ok {
			t.Fatalf("missing field occurrence at %d", offset)
		}
		return occurrence
	}
	aDeclaration := lookup(strings.Index(src, "let value: int32") + len("let "))
	bDeclaration := lookup(strings.LastIndex(src, "let value: int32") + len("let "))
	aRead := lookup(strings.Index(src, "a.value") + len("a."))
	bWrite := lookup(strings.Index(src, "b.value") + len("b."))
	aInitializer := lookup(strings.Index(src, "A { value") + len("A { "))
	bInitializer := lookup(strings.Index(src, "B { value") + len("B { "))
	if aDeclaration.SymbolID == bDeclaration.SymbolID {
		t.Fatal("same-named fields in different classes share an identity")
	}
	if aRead.SymbolID != aDeclaration.SymbolID || aInitializer.SymbolID != aDeclaration.SymbolID {
		t.Fatalf("A field uses have different identities: %#v %#v %#v", aDeclaration, aRead, aInitializer)
	}
	if bWrite.SymbolID != bDeclaration.SymbolID || bInitializer.SymbolID != bDeclaration.SymbolID {
		t.Fatalf("B field uses have different identities: %#v %#v %#v", bDeclaration, bWrite, bInitializer)
	}
}

func TestClassOccurrencesIncludeTypeAnnotations(t *testing.T) {
	src := "package test\nclass Point { let x: int32 }\nfunc wrap(p: Point): Point {\n    let q: Point = Point { x: 1 }\n    return q\n}\n"
	_, graph := analyzeSource(t, src)
	declaration, ok := graph.OccurrenceAt("test.gecko", strings.Index(src, "class Point")+len("class "))
	if !ok {
		t.Fatal("missing class declaration")
	}
	if len(graph.OccurrencesFor(declaration.SymbolID, true)) != 5 {
		t.Fatalf("expected declaration, three type annotations, and initializer: %#v", graph.OccurrencesFor(declaration.SymbolID, true))
	}
}

func TestTraitOccurrencesLinkParentsImplementationsAndTypeAnnotations(t *testing.T) {
	src := "package test\ntrait Parent { func read(self): int32 }\ntrait Child: Parent { func next(self): int32 }\nclass Box {}\nimpl Parent for Box { func read(self): int32 { return 1 } }\nfunc use(value: Parent): void {}\n"
	_, graph := analyzeSource(t, src)
	positions := []int{
		strings.Index(src, "trait Parent") + len("trait "),
		strings.Index(src, "Child: Parent") + len("Child: "),
		strings.Index(src, "impl Parent") + len("impl "),
		strings.Index(src, "value: Parent") + len("value: "),
	}
	var declarationID int64
	for index, position := range positions {
		occurrence, ok := graph.OccurrenceAt("test.gecko", position)
		if !ok {
			t.Fatalf("missing trait occurrence %d", index)
		}
		if index == 0 {
			declarationID = occurrence.SymbolID
			if symbol := graph.SymbolByID(declarationID); symbol == nil || symbol.Kind != semantic.SymbolTrait {
				t.Fatalf("incorrect trait declaration: %#v", symbol)
			}
		} else if occurrence.SymbolID != declarationID {
			t.Fatalf("trait use %d points to another symbol: %#v", index, occurrence)
		}
	}
	method, ok := graph.OccurrenceAt("test.gecko", strings.Index(src, "func read(self)")+len("func "))
	if !ok || graph.SymbolByID(method.SymbolID).Kind != semantic.SymbolMethod || !method.Declaration {
		t.Fatalf("trait method declaration was not indexed: %#v", method)
	}
}
