package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

func (impl *CBackendImplementation) intrinsicBorrow(i *tokens.Intrinsic, scope *ast.Ast) string {
	fail := func(message string) string {
		scope.ErrorScope.NewCompileTimeError("Borrow Error", message, i.Pos)
		return "0"
	}
	if len(i.Args) != 1 || len(i.TypeArgs) != 0 {
		return fail("@" + i.Name + " requires one owner binding")
	}
	symbol, ok := extractPlainSymbolFromExpression(i.Args[0])
	if !ok {
		return fail("@" + i.Name + " requires an owner binding, not a temporary")
	}
	variable := scope.ResolveSymbolAsVariable(symbol)
	if variable.IsNil() {
		return fail("Cannot resolve borrow owner '" + symbol + "'")
	}
	t := impl.GetTypeOfExpression(i.Args[0], scope)
	if t == nil || t.Pointer {
		return fail("Borrow hooks require an owner value")
	}
	kind := hooks.HookBorrow
	if i.Name == "borrow_mut" {
		if variable.Unwrap().IsConst || t.Const {
			return fail("Cannot mutably borrow a readonly owner")
		}
		kind = hooks.HookBorrowMut
	}
	hook, found := impl.resolveVisibleOperatorHook(scope, kind, i.Pos)
	if !found {
		return fail("No visible trait registers @" + string(kind))
	}
	method := lifecycleMethodForType(t, hook, scope)
	if method == "" {
		return fail("Type '" + t.Type + "' does not implement " + hook.TraitName)
	}
	owner := impl.ExpressionToCString(i.Args[0], scope)
	return method + "(&(" + owner + "))"
}
