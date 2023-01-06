package ast

import "github.com/neutrino2211/Gecko/errors"

type Ast struct {
	Scope      string
	Imports    []string
	Methods    map[string]Method
	Variables  map[string]Variable
	Parent     *Ast
	ErrorScope *errors.ErrorScope
}

func (a *Ast) FullScopeName() string {
	r := a.Scope
	parent := a.Parent

	for parent != nil {
		r = parent.Scope + "." + r
		parent = parent.Parent
	}

	return r
}

func (a *Ast) ResolveSymbolAsVariable(symbol string) *Variable {
	scope := a
	symbolVariable, ok := scope.Variables[symbol]

	for !ok {
		if scope.Parent == nil {
			return nil
		}

		scope = scope.Parent
		symbolVariable, ok = scope.Variables[symbol]
	}

	return &symbolVariable
}

func (a *Ast) ResolveMethod(mth string) *Method {
	scope := a
	mthMethod, ok := scope.Methods[mth]

	for !ok {
		if scope.Parent == nil {
			return nil
		}

		scope = scope.Parent
		mthMethod, ok = scope.Methods[mth]
	}

	return &mthMethod
}

func (a *Ast) ToCString() string {
	r := ""
	// solve
	return r
}
