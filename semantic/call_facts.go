package semantic

import "github.com/neutrino2211/gecko/tokens"

func resolvedCall(attempt *resolutionAttempt, receiver *tokens.TypeRef, explicit bool) *CallResolution {
	if attempt == nil || attempt.signature == nil {
		return nil
	}
	resolution := &CallResolution{
		Status:           ResolutionResolved,
		CalleeID:         attempt.signature.SymbolID,
		ReceiverType:     CloneTypeRef(receiver),
		Signature:        substitutedCallSignature(attempt),
		Arguments:        make([]CallArgumentBinding, len(attempt.arguments)),
		ReturnType:       CloneTypeRef(attempt.returnTyp),
		InferredTypeArgs: make(map[string]*tokens.TypeRef, len(attempt.subst)),
		UsedExplicitArgs: explicit,
	}
	for _, diagnostic := range attempt.diagnostics {
		if diagnostic.Severity == SeverityError {
			resolution.Status = ResolutionInvalid
			break
		}
	}
	for name, typ := range attempt.subst {
		resolution.InferredTypeArgs[name] = CloneTypeRef(typ)
	}
	for index, binding := range attempt.arguments {
		resolution.Arguments[index] = binding
		resolution.Arguments[index].ActualType = CloneTypeRef(binding.ActualType)
		resolution.Arguments[index].ExpectedType = CloneTypeRef(binding.ExpectedType)
	}
	return resolution
}

func substitutedCallSignature(attempt *resolutionAttempt) *FunctionSignature {
	signature := cloneSignature(attempt.signature, nil)
	for _, parameter := range signature.Params {
		if parameter == nil {
			continue
		}
		parameter.Type = resolveSelfType(SubstituteTypeParams(parameter.Type, attempt.subst), signature.OwnerType)
	}
	signature.ReturnType = CloneTypeRef(attempt.returnTyp)
	return signature
}
