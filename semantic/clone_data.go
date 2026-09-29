package semantic

import "github.com/neutrino2211/gecko/tokens"

func identity[T any](value T) T { return value }

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	return append([]T{}, values...)
}

func cloneMapValues[K comparable, V any](values map[K]V, clone func(V) V) map[K]V {
	if values == nil {
		return nil
	}
	copy := make(map[K]V, len(values))
	for key, value := range values {
		copy[key] = clone(value)
	}
	return copy
}

func cloneSymbol(symbol *Symbol) *Symbol {
	if symbol == nil {
		return nil
	}
	copy := *symbol
	copy.Type = CloneTypeRef(symbol.Type)
	return &copy
}

func cloneTypeVar(variable *TypeVar) *TypeVar {
	if variable == nil {
		return nil
	}
	copy := *variable
	copy.Traits = cloneSlice(variable.Traits)
	return &copy
}

func cloneIfFacts(facts *IfFacts) *IfFacts {
	if facts == nil {
		return nil
	}
	return &IfFacts{
		Then:  CloneFlowFacts(facts.Then),
		Else:  CloneFlowFacts(facts.Else),
		After: CloneFlowFacts(facts.After),
	}
}

func cloneScopes(scopes map[string][]scopeSnapshot) map[string][]scopeSnapshot {
	return cloneMapValues(scopes, func(items []scopeSnapshot) []scopeSnapshot {
		copy := make([]scopeSnapshot, len(items))
		for index, item := range items {
			copy[index] = item
			copy[index].bindings = cloneMapValues(item.bindings, func(variable *boundVar) *boundVar {
				if variable == nil {
					return nil
				}
				cloned := *variable
				cloned.Type = CloneTypeRef(variable.Type)
				return &cloned
			})
		}
		return copy
	})
}

func cloneSignature(signature *FunctionSignature, nodes map[any]any) *FunctionSignature {
	if signature == nil {
		return nil
	}
	copy := *signature
	copy.ReturnType = CloneTypeRef(signature.ReturnType)
	copy.TypeParams = cloneTypeParams(signature.TypeParams)
	copy.Params = make([]*tokens.Value, len(signature.Params))
	for index, parameter := range signature.Params {
		if parameter == nil {
			continue
		}
		cloned := *parameter
		cloned.Type = CloneTypeRef(parameter.Type)
		if parameter.Default != nil {
			if mapped, ok := nodes[parameter.Default]; ok {
				cloned.Default = mapped.(*tokens.Expression)
			} else {
				cloned.Default = tokens.NewSyntaxCloner().Expression(parameter.Default)
			}
		}
		copy.Params[index] = &cloned
	}
	return &copy
}

func (copy *Program) cloneSignatures(original *Program, nodes map[any]any) {
	cloned := make(map[*FunctionSignature]*FunctionSignature)
	clone := func(signature *FunctionSignature) *FunctionSignature {
		if signature == nil {
			return nil
		}
		if existing := cloned[signature]; existing != nil {
			return existing
		}
		result := cloneSignature(signature, nodes)
		cloned[signature] = result
		return result
	}
	cloneList := func(signatures []*FunctionSignature) []*FunctionSignature {
		if signatures == nil {
			return nil
		}
		result := make([]*FunctionSignature, len(signatures))
		for index, signature := range signatures {
			result[index] = clone(signature)
		}
		return result
	}
	cloneNames := func(groups map[string][]*FunctionSignature) map[string][]*FunctionSignature {
		return cloneMapValues(groups, cloneList)
	}
	copy.functionsByName = cloneNames(original.functionsByName)
	copy.moduleFunctions = cloneMapValues(original.moduleFunctions, cloneNames)
	copy.staticMethods = cloneMapValues(original.staticMethods, cloneNames)
	copy.signatureBySymbolID = cloneMapValues(original.signatureBySymbolID, clone)
}

func cloneClassInfo(class *ClassInfo) *ClassInfo {
	if class == nil {
		return nil
	}
	copy := *class
	copy.TypeParams = cloneTypeParams(class.TypeParams)
	copy.Fields = cloneMapValues(class.Fields, CloneTypeRef)
	copy.FieldSymbolIDs = cloneMapValues(class.FieldSymbolIDs, identity[int64])
	return &copy
}

func (copy *Program) cloneClasses(original *Program) {
	cloned := make(map[*ClassInfo]*ClassInfo)
	clone := func(class *ClassInfo) *ClassInfo {
		if class == nil {
			return nil
		}
		if existing := cloned[class]; existing != nil {
			return existing
		}
		result := cloneClassInfo(class)
		cloned[class] = result
		return result
	}
	copy.classes = cloneMapValues(original.classes, clone)
	copy.classInfosByID = cloneMapValues(original.classInfosByID, clone)
}
