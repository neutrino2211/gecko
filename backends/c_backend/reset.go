package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// ResetState clears package-level backend state between compilations.
func ResetState() {
	CurrentTypeState = nil
	CurrentMonomorphContext = nil

	CScopeDataMap = &CScopeData{}
	CProgramValues = &CValuesMap{}

	TraitDefinitions = make(map[string]*tokens.Trait)
	TraitDefinitionOrigins = make(map[string]string)
	EnumToCType = make(map[string]string)
	MethodReturnTypes = make(map[string]*tokens.TypeRef)
	LastCImportLibraries = nil
	LastCImportObjects = nil
	ResetTreeshakeAnalysis()

	CurrentBackend = nil
	Methods = make(map[string]*ast.Method)

	ResetGenerics()
	ResetUnsafeHandlerCoverage()

	// Unique-id counters for synthesized @unsafe blocks/handlers. Reset so ids
	// start deterministically at 1 each compilation (avoids unbounded growth and
	// keeps generated names stable across runs).
	unsafeBlockCounter = 0
	unsafeHandlerCounter = 0
}
