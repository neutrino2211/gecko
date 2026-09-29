package semantic

import (
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

type pendingTraitParent struct {
	childID int64
	name    string
	file    *tokens.File
}

func (a *analyzer) resolveTraitParentIDs() {
	for _, parent := range a.pendingTraitParents {
		if id := a.resolveTraitParentID(parent); id != 0 {
			a.program.traitParentIDs[parent.childID] = id
		}
	}
}

func (a *analyzer) resolveTraitParentID(parent pendingTraitParent) int64 {
	ids := a.program.traitSymbolIDs[parent.name]
	if len(ids) == 0 || parent.file == nil {
		return 0
	}
	module := moduleNameForFile(parent.file)
	for _, id := range ids {
		if symbol := a.program.SymbolByID(id); symbol != nil && symbol.FullName == module+"::"+parent.name {
			return id
		}
	}
	if id := a.explicitlyImportedSymbolID(parent.file, parent.name, SymbolTrait); id != 0 {
		return id
	}
	if len(ids) == 1 {
		return ids[0]
	}
	return 0
}

func (a *analyzer) registerTraitMethodSignature(module, traitName string, member *tokens.ImplementationField) {
	full := traitName + "::" + member.Name
	if module != "" {
		full = module + "::" + full
	}
	id := a.addSymbol(SymbolMethod, member.Name, full, member.Type, member.Pos)
	signature := &FunctionSignature{
		SymbolID:   id,
		Name:       member.Name,
		FullName:   full,
		Module:     module,
		OwnerType:  traitName,
		ReturnType: CloneTypeRef(member.Type),
		Params:     cloneValues(member.Arguments),
	}
	a.program.signatureBySymbolID[id] = signature
	if a.program.staticMethods[traitName] == nil {
		a.program.staticMethods[traitName] = make(map[string][]*FunctionSignature)
	}
	a.program.staticMethods[traitName][member.Name] = append(a.program.staticMethods[traitName][member.Name], signature)
}

func (a *analyzer) methodCandidatesForReceiver(receiver *tokens.TypeRef, name string) ([]*FunctionSignature, *tokens.TypeRef) {
	current := CloneTypeRef(receiver)
	if current != nil && current.Module == "" && a.currentFunction != nil {
		parameters := a.currentFunction.TypeParams
		if a.currentFunction.OwnerType != "" {
			id := a.classSymbolID(a.currentFunction.OwnerType, "")
			if class := a.program.classInfosByID[id]; class != nil {
				parameters = append(append([]*tokens.TypeParam{}, parameters...), class.TypeParams...)
			}
		}
		for _, parameter := range parameters {
			if parameter == nil || parameter.Name != current.Type {
				continue
			}
			var selected []*FunctionSignature
			var selectedReceiver *tokens.TypeRef
			for _, traitName := range parameter.AllTraits() {
				if traitName == current.Type {
					continue
				}
				id := a.traitSymbolID(traitName, "")
				if id == 0 {
					continue
				}
				traitReceiver := &tokens.TypeRef{Type: traitName, Module: symbolModule(a.program.SymbolByID(id))}
				candidates, resolvedReceiver := a.methodCandidatesForConcreteReceiver(traitReceiver, name)
				if len(candidates) == 0 {
					continue
				}
				if len(selected) > 0 && selected[0].SymbolID != candidates[0].SymbolID {
					return nil, receiver
				}
				selected = candidates
				selectedReceiver = resolvedReceiver
			}
			return selected, selectedReceiver
		}
	}
	return a.methodCandidatesForConcreteReceiver(receiver, name)
}

func (a *analyzer) methodCandidatesForConcreteReceiver(receiver *tokens.TypeRef, name string) ([]*FunctionSignature, *tokens.TypeRef) {
	current := CloneTypeRef(receiver)
	visited := make(map[string]bool)
	for current != nil && current.Type != "" {
		key := current.Module + "::" + current.Type
		if visited[key] {
			break
		}
		visited[key] = true
		ownerID := a.classSymbolID(current.Type, current.Module)
		traitID := int64(0)
		if ownerID == 0 {
			traitID = a.traitSymbolID(current.Type, current.Module)
			ownerID = traitID
		}
		if ownerID == 0 && (len(a.program.classSymbolIDs[current.Type]) > 1 || len(a.program.traitSymbolIDs[current.Type]) > 1) {
			break
		}
		module := current.Module
		if module == "" {
			module = symbolModule(a.program.SymbolByID(ownerID))
		}
		candidates := a.program.staticMethods[current.Type][name]
		if module != "" {
			filtered := make([]*FunctionSignature, 0, len(candidates))
			for _, candidate := range candidates {
				if candidate.Module == module {
					filtered = append(filtered, candidate)
				}
			}
			candidates = filtered
		}
		if len(candidates) > 0 {
			return candidates, current
		}
		parent := a.program.SymbolByID(a.program.traitParentIDs[traitID])
		if parent == nil {
			break
		}
		current.Type = parent.Name
		current.Module = symbolModule(parent)
	}
	return nil, receiver
}

func symbolModule(symbol *Symbol) string {
	if symbol == nil {
		return ""
	}
	if index := strings.LastIndex(symbol.FullName, "::"); index >= 0 {
		return symbol.FullName[:index]
	}
	return ""
}
