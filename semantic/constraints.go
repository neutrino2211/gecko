// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"fmt"

	"github.com/neutrino2211/gecko/tokens"
)

// resolveSelfType rewrites a `Self` type to the concrete owner type so call
// sites see e.g. `Point` rather than the context-dependent `Self`.
func resolveSelfType(t *tokens.TypeRef, ownerType string) *tokens.TypeRef {
	if t == nil || ownerType == "" || t.Type != "Self" {
		return t
	}
	resolved := CloneTypeRef(t)
	resolved.Type = ownerType
	return resolved
}

func (a *analyzer) inferArgumentType(arg *tokens.Argument, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	if arg == nil {
		return nil
	}
	if arg.Value != nil {
		return a.inferExpression(arg.Value, env, expected)
	}
	if arg.SubCall != nil {
		return a.inferFuncCall(arg.SubCall, env, expected)
	}
	return nil
}

func unifyType(formal, actual *tokens.TypeRef, subst map[string]*tokens.TypeRef, typeParams map[string]*tokens.TypeParam) error {
	if formal == nil || actual == nil {
		return nil
	}

	if _, isTypeParam := typeParams[formal.Type]; isTypeParam && formal.Array == nil && formal.Size == nil && formal.FuncType == nil && len(formal.TypeArgs) == 0 {
		existing := subst[formal.Type]
		if existing == nil {
			subst[formal.Type] = CloneTypeRef(actual)
			return nil
		}
		if !TypesCompatible(existing, actual) || !TypesCompatible(actual, existing) {
			return fmt.Errorf("conflicting inference for type parameter '%s'", formal.Type)
		}
		return nil
	}

	if formal.Array != nil || actual.Array != nil {
		if formal.Array == nil || actual.Array == nil {
			return fmt.Errorf("array mismatch")
		}
		return unifyType(formal.Array, actual.Array, subst, typeParams)
	}

	if formal.Size != nil || actual.Size != nil {
		if formal.Size == nil || actual.Size == nil {
			return fmt.Errorf("sized array mismatch")
		}
		return unifyType(formal.Size.Type, actual.Size.Type, subst, typeParams)
	}

	if formal.FuncType != nil || actual.FuncType != nil {
		if formal.FuncType == nil || actual.FuncType == nil {
			return fmt.Errorf("function type mismatch")
		}
		if len(formal.FuncType.ParamTypes) != len(actual.FuncType.ParamTypes) {
			return fmt.Errorf("function arity mismatch")
		}
		for i := range formal.FuncType.ParamTypes {
			if err := unifyType(formal.FuncType.ParamTypes[i], actual.FuncType.ParamTypes[i], subst, typeParams); err != nil {
				return err
			}
		}
		return unifyType(formal.FuncType.ReturnType, actual.FuncType.ReturnType, subst, typeParams)
	}

	if formal.Pointer != actual.Pointer {
		return fmt.Errorf("pointer mismatch")
	}

	if NormalizeTypeName(formal.Type) != NormalizeTypeName(actual.Type) {
		if !(IsNumericType(formal) && IsNumericType(actual)) {
			return fmt.Errorf("type mismatch")
		}
	}

	if len(formal.TypeArgs) > 0 {
		if len(formal.TypeArgs) != len(actual.TypeArgs) {
			return fmt.Errorf("type arg arity mismatch")
		}
		for i := range formal.TypeArgs {
			if err := unifyType(formal.TypeArgs[i], actual.TypeArgs[i], subst, typeParams); err != nil {
				return err
			}
		}
	}

	return nil
}

func isUnresolvedTypeParamRef(t *tokens.TypeRef, typeParams map[string]*tokens.TypeParam) bool {
	if t == nil {
		return false
	}
	if _, ok := typeParams[t.Type]; ok && t.Array == nil && t.Size == nil && t.FuncType == nil && len(t.TypeArgs) == 0 {
		return true
	}
	return false
}

func (a *analyzer) typeImplementsTrait(t *tokens.TypeRef, required string) bool {
	if t == nil || required == "" {
		return false
	}
	typeName := t.Type
	if typeName == "" {
		return false
	}
	if a.program.typeTraits[typeName][required] {
		return true
	}
	for implemented := range a.program.typeTraits[typeName] {
		if a.traitExtends(implemented, required) {
			return true
		}
	}
	return false
}

func (a *analyzer) traitExtends(child string, parent string) bool {
	if child == parent {
		return true
	}
	seen := make(map[string]bool)
	cur := child
	for cur != "" {
		if seen[cur] {
			break
		}
		seen[cur] = true
		next := a.program.traitParents[cur]
		if next == parent {
			return true
		}
		cur = next
	}
	return false
}

func (a *analyzer) findFunctionCandidates(module, ownerType, name string) []*FunctionSignature {
	if ownerType != "" {
		return append([]*FunctionSignature{}, a.program.staticMethods[ownerType][name]...)
	}
	if module != "" {
		if mod, ok := a.program.moduleFunctions[module]; ok {
			return append([]*FunctionSignature{}, mod[name]...)
		}
	}
	return append([]*FunctionSignature{}, a.program.functionsByName[name]...)
}
