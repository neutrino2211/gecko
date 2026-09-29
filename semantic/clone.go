package semantic

import (
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

func cloneDiagnostic(diagnostic Diagnostic) Diagnostic {
	diagnostic.Related = append([]errors.RelatedLocation(nil), diagnostic.Related...)
	diagnostic.Fixes = append([]errors.SuggestedFix(nil), diagnostic.Fixes...)
	return diagnostic
}

func remapNodes[K comparable, V any](values map[K]V, nodes map[any]any) map[K]V {
	result := make(map[K]V, len(values))
	for key, value := range values {
		if replacement, ok := nodes[key]; ok {
			result[replacement.(K)] = value
		}
	}
	return result
}

func (p *Program) CloneForSyntax(file *tokens.File, nodes map[any]any) *Program {
	if p == nil {
		return nil
	}
	copy := *p
	copy.File = file
	copy.Symbols = cloneMapValues(p.Symbols, cloneSymbol)
	copy.TypeVars = cloneMapValues(p.TypeVars, cloneTypeVar)
	copy.expressionTypes = remapNodes(p.expressionTypes, nodes)
	copy.expressionOutcomes = cloneMapValues(remapNodes(p.expressionOutcomes, nodes), cloneExpressionOutcome)
	for node, typ := range copy.expressionTypes {
		copy.expressionTypes[node] = CloneTypeRef(typ)
	}
	copy.literalTypes = remapNodes(p.literalTypes, nodes)
	for node, typ := range copy.literalTypes {
		copy.literalTypes[node] = CloneTypeRef(typ)
	}
	copy.funcCalls = cloneMapValues(remapNodes(p.funcCalls, nodes), cloneCallResolution)
	copy.methodCalls = cloneMapValues(remapNodes(p.methodCalls, nodes), cloneCallResolution)
	copy.chainCalls = cloneMapValues(remapNodes(p.chainCalls, nodes), cloneCallResolution)
	copy.ifFacts = cloneMapValues(remapNodes(p.ifFacts, nodes), cloneIfFacts)
	copy.entryFacts = cloneMapValues(remapNodes(p.entryFacts, nodes), CloneFlowFacts)
	copy.unsafeBindNames = remapNodes(p.unsafeBindNames, nodes)
	copy.unsafeErrorNames = remapNodes(p.unsafeErrorNames, nodes)
	copy.diagnostics = p.Diagnostics()
	copy.occurrences = append([]Occurrence(nil), p.occurrences...)
	copy.occurrenceSet = cloneMapValues(p.occurrenceSet, identity[bool])
	copy.occurrencesByFile = cloneMapValues(p.occurrencesByFile, cloneSlice[Occurrence])
	copy.occurrencesBySymbol = cloneMapValues(p.occurrencesBySymbol, cloneSlice[Occurrence])
	copy.scopes = cloneScopes(p.scopes)
	copy.classSymbolIDs = cloneMapValues(p.classSymbolIDs, cloneSlice[int64])
	copy.traitSymbolIDs = cloneMapValues(p.traitSymbolIDs, cloneSlice[int64])
	copy.enumSymbolIDs = cloneMapValues(p.enumSymbolIDs, cloneSlice[int64])
	copy.enumCaseIDs = cloneMapValues(p.enumCaseIDs, func(value map[string]int64) map[string]int64 {
		return cloneMapValues(value, identity[int64])
	})
	copy.traitParents = cloneMapValues(p.traitParents, identity[string])
	copy.traitParentIDs = cloneMapValues(p.traitParentIDs, identity[int64])
	copy.borrowHooks = cloneMapValues(p.borrowHooks, func(value map[string]string) map[string]string {
		return cloneMapValues(value, identity[string])
	})
	copy.typeTraits = cloneMapValues(p.typeTraits, func(value map[string]bool) map[string]bool {
		return cloneMapValues(value, identity[bool])
	})
	copy.globalsByName = cloneMapValues(p.globalsByName, CloneTypeRef)
	copy.globalSymbolIDs = cloneMapValues(p.globalSymbolIDs, identity[int64])
	copy.cloneSignatures(p, nodes)
	copy.cloneClasses(p)
	return &copy
}

func (p *Program) UnsafeBlockNames() (map[*tokens.UnsafeBlock]string, map[*tokens.UnsafeBlock]string) {
	bindings := make(map[*tokens.UnsafeBlock]string, len(p.unsafeBindNames))
	errors := make(map[*tokens.UnsafeBlock]string, len(p.unsafeErrorNames))
	for block, name := range p.unsafeBindNames {
		bindings[block] = name
	}
	for block, name := range p.unsafeErrorNames {
		errors[block] = name
	}
	return bindings, errors
}
