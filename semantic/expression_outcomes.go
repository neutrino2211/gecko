package semantic

import "github.com/neutrino2211/gecko/tokens"

func cloneExpressionOutcome(outcome *ExpressionOutcome) *ExpressionOutcome {
	if outcome == nil {
		return nil
	}
	copy := *outcome
	return &copy
}

func (a *analyzer) recordExpressionOutcome(expr *tokens.Expression, before int) {
	outcome := &ExpressionOutcome{Status: ResolutionIncomplete}
	if a.program.expressionTypes[expr] != nil {
		outcome.Status = ResolutionResolved
	}
	for index := before; index < len(a.program.diagnostics); index++ {
		if a.program.diagnostics[index].Severity == SeverityError {
			outcome.Status = ResolutionInvalid
			break
		}
	}
	a.program.expressionOutcomes[expr] = outcome
}
