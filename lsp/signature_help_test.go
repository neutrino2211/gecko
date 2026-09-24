// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"strings"
	"testing"
)

func TestSignatureHelpFreeFunction(t *testing.T) {
	content := `package test

func add(a: int, b: int): int {
    return a + b
}

func test(): void {
    let result: int = add(
}
`
	// Line 7 (0-indexed), col 26 (after "add(")
	result := GetSignatureHelp(newTestCtx(content), content, "test.gecko", 7, 26)

	if result == nil {
		t.Fatal("Expected signature help, got nil")
	}

	if len(result.Signatures) != 1 {
		t.Fatalf("Expected 1 signature, got %d", len(result.Signatures))
	}

	sig := result.Signatures[0]
	expectedLabel := "add(a: int, b: int): int"
	if sig.Label != expectedLabel {
		t.Errorf("Expected label '%s', got '%s'", expectedLabel, sig.Label)
	}

	if len(sig.Parameters) != 2 {
		t.Fatalf("Expected 2 parameters, got %d", len(sig.Parameters))
	}

	if result.ActiveParameter != 0 {
		t.Errorf("Expected active parameter 0, got %d", result.ActiveParameter)
	}

	t.Logf("Signature: %s, Active: %d", sig.Label, result.ActiveParameter)
}

func TestSignatureHelpSecondParameter(t *testing.T) {
	content := `package test

func add(a: int, b: int): int {
    return a + b
}

func test(): void {
    let result: int = add(1,
}
`
	// Line 7, col 28 (after "add(1,")
	result := GetSignatureHelp(newTestCtx(content), content, "test.gecko", 7, 28)

	if result == nil {
		t.Fatal("Expected signature help, got nil")
	}

	// Active parameter should be 1 (second parameter)
	if result.ActiveParameter != 1 {
		t.Errorf("Expected active parameter 1, got %d", result.ActiveParameter)
	}
}

func TestSignatureHelpStaticMethod(t *testing.T) {
	content := `package test

class Point {
    public let x: int
    public let y: int
}

impl Point {
    public func new(x: int, y: int): Point {
        return Point { x: x, y: y }
    }
}

func test(): void {
    let p: Point = Point::new(
}
`
	// Line 14 (0-indexed), col 30 (after "Point::new(")
	result := GetSignatureHelp(newTestCtx(content), content, "test.gecko", 14, 30)

	if result == nil {
		t.Fatal("Expected signature help for static method, got nil")
	}

	if len(result.Signatures) != 1 {
		t.Fatalf("Expected 1 signature, got %d", len(result.Signatures))
	}

	sig := result.Signatures[0]
	// Should show the static method signature
	if !strings.Contains(sig.Label, "x: int") || !strings.Contains(sig.Label, "y: int") {
		t.Errorf("Expected signature with x and y parameters, got: %s", sig.Label)
	}

	t.Logf("Static method signature: %s", sig.Label)
}

func TestSignatureHelpInstanceMethod(t *testing.T) {
	content := `package test

class Calculator {
    let value: int
}

impl Calculator {
    func add(self, n: int): int {
        return self.value + n
    }
}

func test(): void {
    let calc: Calculator = Calculator { value: 10 }
    let result: int = calc.add(
}
`
	// Line 14 (0-indexed), col 31 (after "calc.add(")
	result := GetSignatureHelp(newTestCtx(content), content, "test.gecko", 14, 31)

	if result == nil {
		t.Fatal("Expected signature help for instance method, got nil")
	}

	sig := result.Signatures[0]
	// Should show the method signature
	if !strings.Contains(sig.Label, "n: int") {
		t.Errorf("Expected signature with n parameter, got: %s", sig.Label)
	}

	t.Logf("Instance method signature: %s", sig.Label)
}
