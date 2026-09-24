package cbackend

import (
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

func dropMethodForType(t *tokens.TypeRef, scope *ast.Ast) string {
	return lifecycleMethodForType(t, hooks.GetHookRegistry().GetHookFromAnyModule(hooks.HookDrop), scope)
}

func lifecycleMethodForType(t *tokens.TypeRef, hook *hooks.RegisteredHook, scope *ast.Ast) string {
	if t == nil || t.Pointer || t.Array != nil || t.Size != nil {
		return ""
	}
	if hook == nil || len(hook.Methods) == 0 {
		return ""
	}
	concrete := TypeRefToCType(t, scope)
	for _, inst := range Generics.ClassInstantiations {
		if inst.FullName != concrete {
			continue
		}
		class := Generics.GenericClasses[inst.Name]
		if class == nil {
			continue
		}
		for _, implementation := range class.Implementations {
			if implementation.GetFor() != "" && TraitMatchesOrExtends(implementation.GetName(), hook.TraitName) {
				prefix := concrete
				if inst.OriginModule != "" {
					prefix = inst.OriginModule + "__" + prefix
				}
				traitName := implementation.GetName()
				for _, arg := range implementation.GetTypeArgs() {
					name := arg.Type
					for idx, param := range class.TypeParams {
						if param.Name == name && idx < len(inst.TypeArgs) {
							name = inst.TypeArgs[idx]
						}
					}
					traitName += "__" + MangleTypeArgForIdentifier(name)
				}
				return prefix + "__" + traitName + "__" + hook.Methods[0]
			}
		}
	}
	class := scope.ResolveClass(concrete)
	if class.IsNil() {
		class = scope.ResolveClass(t.Type)
	}
	if class.IsNil() {
		return ""
	}
	for name, methods := range class.Unwrap().Traits {
		if !TraitMatchesOrExtends(name, hook.TraitName) || methods == nil {
			continue
		}
		for _, method := range *methods {
			if method.Name == hook.Methods[0] || strings.HasSuffix(method.Name, "__"+hook.Methods[0]) {
				return method.CIdentifier()
			}
		}
	}
	return ""
}

func (impl *CBackendImplementation) intrinsicDropInPlace(i *tokens.Intrinsic, scope *ast.Ast) string {
	if len(i.Args) != 1 {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@drop_in_place requires exactly 1 pointer argument", i.Pos)
		return "0"
	}
	t := impl.GetTypeOfExpression(i.Args[0], scope)
	if t == nil || !t.Pointer || t.Type == "void" {
		scope.ErrorScope.NewCompileTimeError("Intrinsic Error", "@drop_in_place requires a typed pointer", i.Pos)
		return "0"
	}
	CheckReadonlyStore(t, scope, i.Pos)
	ptr := impl.ExpressionToCString(i.Args[0], scope)
	return dropInPlaceCode(t, ptr, scope)
}

func dropInPlaceCode(pointerType *tokens.TypeRef, ptr string, scope *ast.Ast) string {
	t := *pointerType
	t.Pointer = false
	t.Const = false
	if method := dropMethodForType(&t, scope); method != "" {
		return method + "(" + ptr + ")"
	}
	return "(void)(" + ptr + ")"
}
