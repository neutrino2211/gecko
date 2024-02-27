package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

type CBackendImplementation struct {
	Backend interfaces.BackendInterface
}

var FuncCalls = make(map[string]string)
var Methods = make(map[string]*ast.Method)

type CScopeData map[string]*CFileScope
type CProgramValues map[string]*CValueInformation

type CValueInformation struct {
	IsConst   bool
	GeckoType *tokens.TypeRef
	Value     string
}

type CFileScope struct {
	name string
	text string
}

func (c *CFileScope) Init(name string) {
	c.name = name
	c.text = ""
}

func (c *CFileScope) GetSource() string {
	return "//Gecko standard library\n#include <gecko/gecko.h>\n" + c.text
}

var ScopeData = &CScopeData{}
var ProgramValues = &CProgramValues{}
var CurrentBackend interfaces.BackendInterface = nil
