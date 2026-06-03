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

	CurrentBackend = nil
	Methods = make(map[string]*ast.Method)

	ResetGenerics()
}
