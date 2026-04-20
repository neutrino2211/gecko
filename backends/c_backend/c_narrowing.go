package cbackend

import (
	"fmt"
	"os"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/logger"
	"github.com/neutrino2211/gecko/tokens"
)

// isNarrowingDebugEnabled returns true if narrowing debug output should be shown
func isNarrowingDebugEnabled() bool {
	return os.Getenv("GECKO_DEBUG") != "" || logger.IsDebugEnabled()
}

// narrowingDebug prints a debug message if narrowing debug is enabled
func narrowingDebug(msg string) {
	if isNarrowingDebugEnabled() {
		fmt.Fprintf(os.Stderr, "[DEBUG:narrowing] %s\n", msg)
	}
}

// NullCheckInfo represents a detected null check pattern in a condition
type NullCheckInfo struct {
	// VarName is the variable being checked against nil
	VarName string
	// IsNotNull is true for != nil, false for == nil
	IsNotNull bool
}

// DetectNullCheck analyzes an expression to see if it's a null check pattern.
// Returns the variable name being checked and whether it's a != nil check.
// Detects:
// - @is_not_null(ptr) intrinsic (preferred)
// - @is_null(ptr) intrinsic
// - ptr != nil pattern (legacy)
// - ptr == nil pattern (legacy)
func DetectNullCheck(expr *tokens.Expression) *NullCheckInfo {
	if expr == nil || expr.LogicalOr == nil {
		narrowingDebug("DetectNullCheck: expr or LogicalOr is nil")
		return nil
	}

	// Handle parenthesized expressions - unwrap SubExpression
	lo := expr.LogicalOr
	if lo.LogicalAnd != nil && lo.LogicalAnd.Equality != nil &&
		lo.LogicalAnd.Equality.Comparison != nil &&
		lo.LogicalAnd.Equality.Comparison.Addition != nil &&
		lo.LogicalAnd.Equality.Comparison.Addition.Multiplication != nil &&
		lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary != nil &&
		lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary != nil &&
		lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary.SubExpression != nil {
		// Recursively check the inner expression
		narrowingDebug("DetectNullCheck: Found SubExpression, unwrapping")
		return DetectNullCheck(lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary.SubExpression)
	}

	// Check for @is_not_null(ptr) or @is_null(ptr) intrinsic
	if info := detectIntrinsicNullCheck(expr); info != nil {
		return info
	}

	// For simple null checks, we expect a single LogicalOr with a single LogicalAnd
	if lo.Next != nil {
		// Complex OR expression - don't try to narrow for now
		narrowingDebug("DetectNullCheck: Complex OR expression, skipping")
		return nil
	}

	if lo.LogicalAnd == nil {
		narrowingDebug("DetectNullCheck: LogicalAnd is nil")
		return nil
	}

	la := lo.LogicalAnd
	if la.Next != nil {
		// Complex AND expression - we could support this later
		// For now, skip narrowing
		narrowingDebug("DetectNullCheck: Complex AND expression, skipping")
		return nil
	}

	if la.Equality == nil {
		narrowingDebug("DetectNullCheck: Equality is nil")
		return nil
	}

	eq := la.Equality
	// We need an equality operator (!= or ==) to detect null checks
	if eq.Op == "" || eq.Next == nil {
		narrowingDebug(fmt.Sprintf("DetectNullCheck: No equality operator or no Next (Op='%s')", eq.Op))
		return nil
	}

	// Check if it's != or ==
	if eq.Op != "!=" && eq.Op != "==" {
		narrowingDebug(fmt.Sprintf("DetectNullCheck: Operator is not != or == (Op='%s')", eq.Op))
		return nil
	}

	// Extract left side symbol and right side nil check
	leftSymbol := extractSymbolFromComparison(eq.Comparison)
	rightIsNil := isNilExpression(eq.Next)

	narrowingDebug(fmt.Sprintf("DetectNullCheck: leftSymbol='%s', rightIsNil=%v", leftSymbol, rightIsNil))

	if leftSymbol != "" && rightIsNil {
		return &NullCheckInfo{
			VarName:   leftSymbol,
			IsNotNull: eq.Op == "!=",
		}
	}

	// Check the reverse: nil != symbol
	leftIsNil := isNilExpressionFromComparison(eq.Comparison)
	rightSymbol := extractSymbolFromEquality(eq.Next)

	narrowingDebug(fmt.Sprintf("DetectNullCheck: leftIsNil=%v, rightSymbol='%s'", leftIsNil, rightSymbol))

	if leftIsNil && rightSymbol != "" {
		return &NullCheckInfo{
			VarName:   rightSymbol,
			IsNotNull: eq.Op == "!=",
		}
	}

	return nil
}

// detectIntrinsicNullCheck checks if the expression is an @is_not_null or @is_null intrinsic
func detectIntrinsicNullCheck(expr *tokens.Expression) *NullCheckInfo {
	if expr == nil || expr.LogicalOr == nil {
		return nil
	}

	lo := expr.LogicalOr
	if lo.Next != nil || lo.LogicalAnd == nil {
		return nil
	}

	la := lo.LogicalAnd
	if la.Next != nil || la.Equality == nil {
		return nil
	}

	eq := la.Equality
	if eq.Next != nil || eq.Comparison == nil {
		return nil
	}

	c := eq.Comparison
	if c.Next != nil || c.Addition == nil {
		return nil
	}

	a := c.Addition
	if a.Next != nil || a.Multiplication == nil {
		return nil
	}

	m := a.Multiplication
	if m.Next != nil || m.Unary == nil {
		return nil
	}

	u := m.Unary
	if u.Primary == nil || u.Primary.Literal == nil {
		return nil
	}

	lit := u.Primary.Literal
	if lit.Intrinsic == nil {
		return nil
	}

	intr := lit.Intrinsic
	if intr.Name != "is_not_null" && intr.Name != "is_null" {
		return nil
	}

	if len(intr.Args) != 1 {
		return nil
	}

	// Extract the symbol from the intrinsic argument
	varName := extractSymbolFromExpression(intr.Args[0])
	if varName == "" {
		narrowingDebug("detectIntrinsicNullCheck: Could not extract symbol from intrinsic argument")
		return nil
	}

	isNotNull := intr.Name == "is_not_null"
	narrowingDebug(fmt.Sprintf("detectIntrinsicNullCheck: Found @%s(%s)", intr.Name, varName))

	return &NullCheckInfo{
		VarName:   varName,
		IsNotNull: isNotNull,
	}
}

// extractSymbolFromExpression extracts a simple symbol name from an Expression
func extractSymbolFromExpression(expr *tokens.Expression) string {
	if expr == nil || expr.LogicalOr == nil {
		return ""
	}
	lo := expr.LogicalOr
	if lo.Next != nil || lo.LogicalAnd == nil {
		return ""
	}
	la := lo.LogicalAnd
	if la.Next != nil || la.Equality == nil {
		return ""
	}
	return extractSymbolFromEquality(la.Equality)
}

// extractSymbolFromComparison extracts a simple symbol name from a Comparison node
func extractSymbolFromComparison(c *tokens.Comparison) string {
	if c == nil || c.Addition == nil {
		return ""
	}

	// Follow the chain: Comparison -> Addition -> Multiplication -> Unary -> Primary -> Literal
	return extractSymbolFromAddition(c.Addition)
}

func extractSymbolFromAddition(a *tokens.Addition) string {
	if a == nil || a.Multiplication == nil {
		return ""
	}
	if a.Next != nil {
		// Has arithmetic operations, not a simple symbol
		return ""
	}
	return extractSymbolFromMultiplication(a.Multiplication)
}

func extractSymbolFromMultiplication(m *tokens.Multiplication) string {
	if m == nil || m.Unary == nil {
		return ""
	}
	if m.Next != nil {
		// Has arithmetic operations, not a simple symbol
		return ""
	}
	return extractSymbolFromUnary(m.Unary)
}

func extractSymbolFromUnary(u *tokens.Unary) string {
	if u == nil {
		return ""
	}
	if u.Op != "" {
		// Has unary operator, not a simple symbol reference
		return ""
	}
	if u.Primary == nil {
		return ""
	}
	return extractSymbolFromPrimary(u.Primary)
}

func extractSymbolFromPrimary(p *tokens.Primary) string {
	if p == nil || p.Literal == nil {
		return ""
	}
	return extractSymbolFromLiteral(p.Literal)
}

func extractSymbolFromLiteral(l *tokens.Literal) string {
	if l == nil {
		return ""
	}
	// Only simple symbol references, not struct literals, arrays, etc.
	if l.Symbol != "" && l.SymbolModule == "" && len(l.Chain) == 0 && l.ArrayIndex == nil {
		return l.Symbol
	}
	return ""
}

// isNilExpression checks if an Equality node represents just "nil"
func isNilExpression(eq *tokens.Equality) bool {
	if eq == nil || eq.Comparison == nil {
		return false
	}
	if eq.Next != nil {
		// Has another equality, not simple nil
		return false
	}
	return isNilExpressionFromComparison(eq.Comparison)
}

func isNilExpressionFromComparison(c *tokens.Comparison) bool {
	if c == nil || c.Addition == nil {
		return false
	}
	if c.Next != nil {
		return false
	}
	return isNilExpressionFromAddition(c.Addition)
}

func isNilExpressionFromAddition(a *tokens.Addition) bool {
	if a == nil || a.Multiplication == nil {
		return false
	}
	if a.Next != nil {
		return false
	}
	return isNilExpressionFromMultiplication(a.Multiplication)
}

func isNilExpressionFromMultiplication(m *tokens.Multiplication) bool {
	if m == nil || m.Unary == nil {
		return false
	}
	if m.Next != nil {
		return false
	}
	return isNilExpressionFromUnary(m.Unary)
}

func isNilExpressionFromUnary(u *tokens.Unary) bool {
	if u == nil || u.Primary == nil {
		return false
	}
	if u.Op != "" {
		return false
	}
	return isNilLiteral(u.Primary.Literal)
}

func isNilLiteral(l *tokens.Literal) bool {
	if l == nil {
		return false
	}
	// nil is captured as a Symbol (PlainSymbol) with value "nil"
	return l.Symbol == "nil" && l.SymbolModule == ""
}

// extractSymbolFromEquality extracts symbol from right side of equality
func extractSymbolFromEquality(eq *tokens.Equality) string {
	if eq == nil || eq.Comparison == nil {
		return ""
	}
	return extractSymbolFromComparison(eq.Comparison)
}

// ApplyNullNarrowing applies type narrowing based on a null check to the given TypeState.
// Called when entering an if-body where the condition was a null check.
func ApplyNullNarrowing(ts *ast.TypeState, info *NullCheckInfo, scope *ast.Ast) {
	if ts == nil || info == nil {
		return
	}

	// Only narrow for != nil checks (variable is non-null in the if body)
	if !info.IsNotNull {
		narrowingDebug(fmt.Sprintf("ApplyNullNarrowing: Skipping == nil check for '%s'", info.VarName))
		return
	}

	// Verify the variable exists in scope
	varOpt := scope.ResolveSymbolAsVariable(info.VarName)
	if varOpt.IsNil() {
		// Variable not found, skip narrowing
		narrowingDebug(fmt.Sprintf("ApplyNullNarrowing: Variable '%s' not found in scope, skipping", info.VarName))
		return
	}

	// Apply the narrowing
	ts.SetNonNull(info.VarName)

	narrowingDebug(fmt.Sprintf("ApplyNullNarrowing: '%s != nil' - marked as non-null in if-body", info.VarName))
}
