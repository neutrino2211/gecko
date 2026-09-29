package semantic

func (a *analyzer) candidateProbe() *analyzer {
	probe := *a
	program := *a.program
	program.Symbols = cloneMapValues(a.program.Symbols, cloneSymbol)
	program.TypeVars = cloneMapValues(a.program.TypeVars, cloneTypeVar)
	program.expressionTypes = cloneMapValues(a.program.expressionTypes, CloneTypeRef)
	program.expressionOutcomes = cloneMapValues(a.program.expressionOutcomes, cloneExpressionOutcome)
	program.literalTypes = cloneMapValues(a.program.literalTypes, CloneTypeRef)
	program.funcCalls = cloneMapValues(a.program.funcCalls, cloneCallResolution)
	program.methodCalls = cloneMapValues(a.program.methodCalls, cloneCallResolution)
	program.chainCalls = cloneMapValues(a.program.chainCalls, cloneCallResolution)
	program.ifFacts = cloneMapValues(a.program.ifFacts, cloneIfFacts)
	program.entryFacts = cloneMapValues(a.program.entryFacts, CloneFlowFacts)
	program.diagnostics = a.program.Diagnostics()
	program.unsafeBindNames = cloneMapValues(a.program.unsafeBindNames, identity[string])
	program.unsafeErrorNames = cloneMapValues(a.program.unsafeErrorNames, identity[string])
	program.occurrences = cloneSlice(a.program.occurrences)
	program.occurrenceSet = cloneMapValues(a.program.occurrenceSet, identity[bool])
	program.occurrencesByFile = cloneMapValues(a.program.occurrencesByFile, cloneSlice[Occurrence])
	program.occurrencesBySymbol = cloneMapValues(a.program.occurrencesBySymbol, cloneSlice[Occurrence])
	program.scopes = cloneScopes(a.program.scopes)
	program.cloneClasses(a.program)
	probe.program = &program
	probe.nameForID = cloneMapValues(a.nameForID, identity[string])
	probe.setupNames = cloneMapValues(a.setupNames, identity[bool])
	return &probe
}
