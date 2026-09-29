package semantic

import (
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) validateImplementationCoherence(implementation *tokens.Implementation) {
	if implementation == nil || a.currentFile == nil {
		return
	}
	className := implementation.GetName()
	if implementation.GetFor() != "" {
		className = implementation.GetFor()
	}
	classID := a.classSymbolID(className, "")
	if classID == 0 {
		return
	}
	class := a.program.SymbolByID(classID)
	currentModule := moduleNameForFile(a.currentFile)
	classModule := symbolModule(class)
	if classModule == currentModule {
		return
	}
	typeName := className
	if classModule != "" {
		typeName = classModule + "." + className
	}
	if implementation.GetFor() == "" {
		a.program.addDiagnostic(Diagnostic{
			Severity: SeverityError, Title: "Coherence Error", Code: errors.CodeCoherence,
			Message: "cannot add inherent impl for foreign type '" + typeName + "'",
			Help:    "inherent impls are only allowed in the defining package '" + classModule + "'",
			Pos:     implementation.Pos, EndOffset: implementation.EndPos.Offset,
		})
		return
	}
	traitName := implementation.GetName()
	traitID := a.traitSymbolID(traitName, "")
	if traitID == 0 {
		return
	}
	traitModule := symbolModule(a.program.SymbolByID(traitID))
	if traitModule == currentModule {
		return
	}
	qualifiedTrait := traitName
	if traitModule != "" {
		qualifiedTrait = traitModule + "." + traitName
	}
	a.program.addDiagnostic(Diagnostic{
		Severity: SeverityError, Title: "Coherence Error", Code: errors.CodeCoherence,
		Message: "orphan impl is not allowed: both trait '" + qualifiedTrait + "' and type '" + typeName + "' are foreign",
		Help:    "define a local trait or wrap the foreign type in a local newtype",
		Pos:     implementation.Pos, EndOffset: implementation.EndPos.Offset,
	})
}
