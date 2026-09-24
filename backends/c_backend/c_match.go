// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// NewMatch handles match expressions by generating if-else chains
func (impl *CBackendImplementation) NewMatch(scope *ast.Ast, m *tokens.Match) {
	info := CGetScopeInformation(scope)

	scrutinee := impl.ExpressionToCString(m.Scrutinee, scope)

	// Generate if-else chain for each case
	for i, caseClause := range m.Cases {
		cond := impl.MatchPatternToCondition(caseClause.Pattern, scrutinee, scope)

		// Add guard if present
		if caseClause.Guard != nil {
			guardExpr := impl.ExpressionToCString(caseClause.Guard, scope)
			cond = "(" + cond + ") && (" + guardExpr + ")"
		}

		// Check if this is a default case (wildcard "_" or bare "default" identifier)
		isDefault := caseClause.Pattern != nil && len(caseClause.Pattern.Alts) == 1 &&
			caseClause.Pattern.Alts[0].Inner != nil &&
			(caseClause.Pattern.Alts[0].Inner.Wildcard != nil ||
				(caseClause.Pattern.Alts[0].Inner.Literal != nil &&
					caseClause.Pattern.Alts[0].Inner.Literal.GetBareIdentifier() == "default"))

		if isDefault && i > 0 {
			info.Code += "    else {\n"
		} else if i == 0 {
			info.Code += fmt.Sprintf("    if (%s) {\n", cond)
		} else {
			info.Code += fmt.Sprintf("    else if (%s) {\n", cond)
		}

		caseScope := newLexicalScope(scope, false)
		caseInfo := CGetScopeInformation(caseScope)
		// Emit destructuring bindings before body entries
		if caseClause.Pattern != nil {
			impl.EmitDestructuringBindings(caseClause.Pattern, scrutinee, caseScope)
		}

		entries, expr := impl.GetMatchCaseBody(caseClause.Body)
		for _, entry := range entries {
			impl.processEntry(caseScope, entry)
		}
		if expr != nil {
			impl.NewReturnLiteral(caseScope, expr)
		}
		impl.emitDefers(caseScope, caseInfo)
		impl.generateDropCalls(caseScope, caseInfo)
		info.Code += caseInfo.Code + "    }\n"
	}
}

// MatchPatternToCondition compiles a MatchPattern into a C boolean expression
func (impl *CBackendImplementation) MatchPatternToCondition(pattern *tokens.MatchPattern, scrutinee string, scope *ast.Ast) string {
	if pattern == nil {
		return "1" // always true (default case)
	}

	if len(pattern.Alts) == 0 {
		return "1"
	}

	// Single "default"/"_" wildcard
	if len(pattern.Alts) == 1 && pattern.Alts[0].Inner != nil {
		if pattern.Alts[0].Inner.Wildcard != nil {
			return "1"
		}
		// Check for bare "default" identifier
		if pattern.Alts[0].Inner.Literal != nil && pattern.Alts[0].Inner.Literal.GetBareIdentifier() == "default" {
			return "1"
		}
	}

	var parts []string
	for _, alt := range pattern.Alts {
		cond := impl.MatchPatternAltToCondition(alt, scrutinee, scope)
		parts = append(parts, cond)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " || ") + ")"
}

// MatchPatternAltToCondition compiles a single pattern alternative into a C boolean expression
func (impl *CBackendImplementation) MatchPatternAltToCondition(alt *tokens.MatchPatternAlt, scrutinee string, scope *ast.Ast) string {
	if alt == nil || alt.Inner == nil {
		return "1"
	}
	inner := alt.Inner

	if inner.Wildcard != nil {
		return "1"
	}

	if inner.Literal != nil {
		pattern := impl.PrimaryToCString(inner.Literal, scope)
		return fmt.Sprintf("%s == %s", scrutinee, pattern)
	}

	if inner.Range != nil {
		start := impl.PrimaryToCString(inner.Range.Start, scope)
		end := impl.PrimaryToCString(inner.Range.End, scope)
		return fmt.Sprintf("%s >= %s && %s <= %s", scrutinee, start, scrutinee, end)
	}

	if inner.Destructure != nil {
		var checks []string
		for _, f := range inner.Destructure.Fields {
			fieldExpr := fmt.Sprintf("%s.%s", scrutinee, f.Name)
			if f.Pattern != nil {
				fieldCond := impl.MatchPatternToCondition(f.Pattern, fieldExpr, scope)
				checks = append(checks, fieldCond)
			}
		}
		if len(checks) == 0 {
			return "1"
		}
		if len(checks) == 1 {
			return checks[0]
		}
		return "(" + strings.Join(checks, " && ") + ")"
	}

	return "1"
}

// EmitDestructuringBindings emits variable bindings for destructuring and OR-pattern bindings
func (impl *CBackendImplementation) EmitDestructuringBindings(pattern *tokens.MatchPattern, scrutinee string, scope *ast.Ast) {
	if pattern == nil {
		return
	}
	info := CGetScopeInformation(scope)

	for _, alt := range pattern.Alts {
		if alt == nil || alt.Binding == nil || alt.Inner == nil {
			continue
		}
		bindingName := *alt.Binding
		// Only bind the first matching alt (for OR patterns, this is an approximation)
		info.Code += fmt.Sprintf("    int32_t %s = %s;\n", bindingName, scrutinee)

		// Also emit destructuring bindings for struct fields
		if alt.Inner.Destructure != nil {
			for _, f := range alt.Inner.Destructure.Fields {
				if f.Pattern != nil && f.Pattern.Alts != nil {
					for _, innerAlt := range f.Pattern.Alts {
						if innerAlt != nil && innerAlt.Binding != nil {
							fieldBinding := *innerAlt.Binding
							info.Code += fmt.Sprintf("    int32_t %s = %s.%s;\n", fieldBinding, bindingName, f.Name)
						}
					}
				}
			}
		}
	}
}

// GetMatchCaseBody returns the entries and optional expression from a MatchCaseBody
func (impl *CBackendImplementation) GetMatchCaseBody(body *tokens.MatchCaseBody) ([]*tokens.Entry, *tokens.Expression) {
	if body == nil {
		return nil, nil
	}
	if body.Expr != nil {
		return nil, body.Expr
	}
	return body.Entries, nil
}

// MatchAsExpressionToCString compiles a match expression into a C expression using GCC statement expressions
func (impl *CBackendImplementation) MatchAsExpressionToCString(m *tokens.Match, scope *ast.Ast) string {
	info := CGetScopeInformation(scope)
	scrutinee := impl.ExpressionToCString(m.Scrutinee, scope)

	// Generate a temp variable name using position
	tmpVar := fmt.Sprintf("____match_%d_%d", m.Pos.Line, m.Pos.Column)

	// Declare the temp variable
	info.Code += fmt.Sprintf("    int32_t %s;\n", tmpVar)

	// Generate if-else chain that assigns to temp
	for i, caseClause := range m.Cases {
		cond := impl.MatchPatternToCondition(caseClause.Pattern, scrutinee, scope)

		if caseClause.Guard != nil {
			guardExpr := impl.ExpressionToCString(caseClause.Guard, scope)
			cond = "(" + cond + ") && (" + guardExpr + ")"
		}

		isDefault := caseClause.Pattern != nil && len(caseClause.Pattern.Alts) == 1 &&
			caseClause.Pattern.Alts[0].Inner != nil &&
			(caseClause.Pattern.Alts[0].Inner.Wildcard != nil ||
				(caseClause.Pattern.Alts[0].Inner.Literal != nil &&
					caseClause.Pattern.Alts[0].Inner.Literal.GetBareIdentifier() == "default"))

		if isDefault && i > 0 {
			info.Code += "    else {\n"
		} else if i == 0 {
			info.Code += fmt.Sprintf("    if (%s) {\n", cond)
		} else {
			info.Code += fmt.Sprintf("    else if (%s) {\n", cond)
		}

		caseScope := newLexicalScope(scope, false)
		caseInfo := CGetScopeInformation(caseScope)
		// Emit destructuring bindings
		if caseClause.Pattern != nil {
			impl.EmitDestructuringBindings(caseClause.Pattern, scrutinee, caseScope)
		}

		entries, expr := impl.GetMatchCaseBody(caseClause.Body)
		for _, entry := range entries {
			impl.processEntry(caseScope, entry)
		}
		if expr != nil {
			exprStr := impl.ExpressionToCString(expr, caseScope)
			caseInfo.Code += fmt.Sprintf("        %s = %s;\n", tmpVar, exprStr)
		}
		impl.emitDefers(caseScope, caseInfo)
		impl.generateDropCalls(caseScope, caseInfo)
		info.Code += caseInfo.Code + "    }\n"
	}

	return tmpVar
}
