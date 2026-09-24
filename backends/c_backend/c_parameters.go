package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

func registerOwnedParameters(scope *ast.Ast, info *CScopeInformation, arguments []*tokens.Value) {
	for _, argument := range arguments {
		if argument.Name == "self" || argument.Type == nil || dropMethodForType(argument.Type, scope) == "" {
			continue
		}
		variable, exists := scope.Variables[argument.Name]
		if !exists {
			continue
		}
		variable.DropFlag = "__owned_" + variable.GetFullName()
		scope.Variables[argument.Name] = variable
		info.LocalVarOrder = append(info.LocalVarOrder, argument.Name)
		info.Code += "int " + variable.DropFlag + " = 1;\n"
	}
}
