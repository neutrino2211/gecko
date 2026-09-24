package semantic

import "github.com/neutrino2211/gecko/tokens"

func (a *analyzer) indexBorrowHooks(trait *tokens.Trait) {
	for _, attr := range trait.Attributes {
		if attr.Name != "borrow_hook" && attr.Name != "borrow_mut_hook" {
			continue
		}
		methods := attr.GetHookMethods()
		if len(methods) != 1 {
			continue
		}
		if a.program.borrowHooks[attr.Name] == nil {
			a.program.borrowHooks[attr.Name] = make(map[string]string)
		}
		a.program.borrowHooks[attr.Name][trait.Name] = methods[0]
	}
}

func (a *analyzer) inferBorrow(intr *tokens.Intrinsic, env *flowEnv) *tokens.TypeRef {
	if len(intr.Args) != 1 {
		return nil
	}
	owner := a.inferExpression(intr.Args[0], env, nil)
	if owner == nil {
		return nil
	}
	var result *tokens.TypeRef
	for trait, method := range a.program.borrowHooks[intr.Name+"_hook"] {
		if !a.program.typeTraits[owner.Type][trait] {
			continue
		}
		if result != nil {
			return nil
		}
		result = a.inferMethodOnType(owner, &tokens.ChainAccess{Name: method}, env)
	}
	return result
}
