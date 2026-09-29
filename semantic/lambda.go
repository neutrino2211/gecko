package semantic

import (
	"fmt"

	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) inferLambda(lambda *tokens.Lambda, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if lambda == nil {
		return nil
	}
	if env == nil {
		env = newFlowEnv()
	}
	var expectedFunction *tokens.FuncType
	if expected != nil {
		expectedFunction = expected.FuncType
	}
	bodyEnv := env.clone()
	parameterTypes := make([]*tokens.TypeRef, len(lambda.Params))
	owner := "lambda"
	if a.currentFunction != nil {
		owner = a.currentFunction.FullName
	}
	for index, parameter := range lambda.Params {
		if parameter == nil {
			continue
		}
		a.recordTypeRef(parameter.Type)
		parameterType := CloneTypeRef(parameter.Type)
		if parameterType == nil && expectedFunction != nil && index < len(expectedFunction.ParamTypes) {
			parameterType = CloneTypeRef(expectedFunction.ParamTypes[index])
		}
		parameterTypes[index] = CloneTypeRef(parameterType)
		fullName := fmt.Sprintf("%s::lambda@%d::%s", owner, lambda.Pos.Offset, parameter.Name)
		id := a.addSymbol(SymbolVariable, parameter.Name, fullName, parameterType, parameter.Pos)
		a.program.Symbols[id].Local = true
		a.nameForID[id] = parameter.Name
		bodyEnv.bind(parameter.Name, parameterType, id)
	}
	a.recordTypeRef(lambda.ReturnType)
	returnType := CloneTypeRef(lambda.ReturnType)
	if returnType == nil && expectedFunction != nil {
		returnType = CloneTypeRef(expectedFunction.ReturnType)
	}
	previousReturn := a.currentReturn
	a.currentReturn = returnType
	_ = a.analyzeEntries(lambda.Body, bodyEnv, lambda.Pos, lambda.EndPos)
	a.currentReturn = previousReturn
	if expectedFunction != nil {
		return CloneTypeRef(expected)
	}
	return &tokens.TypeRef{FuncType: &tokens.FuncType{
		ParamTypes: parameterTypes,
		ReturnType: returnType,
	}}
}
