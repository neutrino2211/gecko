package semantic

import "github.com/neutrino2211/gecko/tokens"

func (a *analyzer) inferIntrinsic(intr *tokens.Intrinsic, env *flowEnv) *tokens.TypeRef {
	if intr == nil {
		return nil
	}
	for _, arg := range intr.Args {
		a.inferExpression(arg, env, nil)
	}
	switch intr.Name {
	case "move":
		if len(intr.Args) == 1 {
			return a.inferExpression(intr.Args[0], env, nil)
		}
		return nil
	case "borrow", "borrow_mut":
		return a.inferBorrow(intr, env)
	case "size_of", "align_of":
		return &tokens.TypeRef{Type: "uint64"}
	case "alloc":
		return &tokens.TypeRef{Type: "void", Pointer: true}
	case "deref", "read_volatile":
		if len(intr.Args) == 1 {
			t := a.inferExpression(intr.Args[0], env, nil)
			if t != nil && t.Pointer {
				t = CloneTypeRef(t)
				t.Pointer = false
				t.NonNull = false
				t.Const = false
				t.Volatile = false
				return t
			}
		}
		return nil
	case "is_null", "is_not_null":
		return &tokens.TypeRef{Type: "bool"}
	default:
		return nil
	}
}
