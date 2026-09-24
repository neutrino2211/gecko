// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

var unaryOperatorToHook = map[string]hooks.HookType{
	"-": hooks.HookNeg,
	"!": hooks.HookNot,
}

func typeArgsToTraitMangleParts(typeArgs []*tokens.TypeRef, scope *ast.Ast) []string {
	if len(typeArgs) == 0 {
		return nil
	}

	parts := make([]string, len(typeArgs))
	for i, arg := range typeArgs {
		if arg == nil {
			continue
		}
		if cType, ok := GeckoToCType[arg.Type]; ok {
			parts[i] = cType
		} else {
			parts[i] = TypeRefToCType(arg, scope)
		}
	}

	return parts
}

func findTraitMethodReturnType(class *ast.Ast, traitName string, methodName string, operandType *tokens.TypeRef) *tokens.TypeRef {
	if class == nil {
		return nil
	}

	matchedTrait := ""
	for candidate := range class.Traits {
		if candidate == traitName {
			matchedTrait = candidate
			break
		}
	}
	if matchedTrait == "" {
		for candidate := range class.Traits {
			if TraitMatchesOrExtends(candidate, traitName) {
				matchedTrait = candidate
				break
			}
		}
	}
	if matchedTrait == "" {
		return nil
	}

	traitMethods := class.Traits[matchedTrait]
	if traitMethods == nil {
		return nil
	}

	expectedSuffix := "__" + methodName
	for _, m := range *traitMethods {
		if m == nil || len(m.Name) < len(expectedSuffix) || !strings.HasSuffix(m.Name, expectedSuffix) {
			continue
		}
		if m.Type == "T" && len(operandType.TypeArgs) > 0 {
			return cloneTypeRefForInference(operandType.TypeArgs[0])
		}
		return &tokens.TypeRef{Type: m.Type}
	}

	return nil
}

func (impl *CBackendImplementation) resolveHookValueType(operandType *tokens.TypeRef, scope *ast.Ast, hookType hooks.HookType, hookMethod string) *tokens.TypeRef {
	if operandType == nil || operandType.Type == "" || hookMethod == "" {
		return nil
	}

	hook := hooks.GetHookRegistry().GetHookFromAnyModule(hookType)
	if hook == nil {
		return nil
	}

	baseTypeName := operandType.Type
	concreteTypeName := baseTypeName
	if len(operandType.TypeArgs) > 0 {
		concreteTypeName = mangleName(baseTypeName, typeArgsToTraitMangleParts(operandType.TypeArgs, scope))
	}

	traitName, found := impl.GetOperatorTraitName(concreteTypeName, hook.TraitName, scope)
	if !found {
		return nil
	}

	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(concreteTypeName)
	if classOpt.IsNil() && baseTypeName != concreteTypeName {
		classOpt = rootScope.ResolveClass(baseTypeName)
	}
	if classOpt.IsNil() {
		return nil
	}

	if returnType := findTraitMethodReturnType(classOpt.Unwrap(), traitName, hookMethod, operandType); returnType != nil {
		return returnType
	}

	// Generic fallback: many hook-based wrappers are Tryable<T>/Orable<T>.
	if len(operandType.TypeArgs) > 0 {
		return cloneTypeRefForInference(operandType.TypeArgs[0])
	}

	return nil
}

// HasOperatorTrait checks if a type has an operator trait implemented
func (impl *CBackendImplementation) HasOperatorTrait(typeName string, traitName string, scope *ast.Ast) bool {
	rootScope := scope.GetRoot()
	classOpt := rootScope.ResolveClass(typeName)
	if classOpt.IsNil() {
		return false
	}

	class := classOpt.Unwrap()

	// Check if the class has this trait implemented
	// Trait names are mangled as "TraitName__TypeArg" (e.g., "Add__Point")
	for tName := range class.Traits {
		if TraitMatchesOrExtends(tName, traitName) {
			return true
		}
	}

	return false
}

func resolveClassFromImportClosure(typeName string, scope *ast.Ast) (*ast.Ast, bool) {
	if scope == nil {
		return nil, false
	}

	rootScope := scope.GetRoot()
	visited := make(map[*ast.Ast]bool)
	queue := []*ast.Ast{rootScope}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == nil || visited[current] {
			continue
		}
		visited[current] = true

		if classDef, ok := current.Classes[typeName]; ok && classDef != nil {
			if classDef.CheckVisibility(scope, typeName) == "" {
				return classDef, true
			}
		}

		for _, child := range current.Children {
			if child != nil && !visited[child] {
				queue = append(queue, child)
			}
		}
	}

	return nil, false
}

func resolveClassForOperatorTrait(typeName string, scope *ast.Ast) (*ast.Ast, bool) {
	if scope == nil {
		return nil, false
	}

	classOpt := scope.ResolveClass(typeName)
	if !classOpt.IsNil() {
		classDef := classOpt.Unwrap()
		if classDef.CheckVisibility(scope, typeName) == "" {
			return classDef, true
		}
	}

	return resolveClassFromImportClosure(typeName, scope)
}

// GetOperatorTraitName returns the full mangled trait name for an operator if the type implements it
func (impl *CBackendImplementation) GetOperatorTraitName(typeName string, traitName string, scope *ast.Ast) (string, bool) {
	classDef, found := resolveClassForOperatorTrait(typeName, scope)

	// If not found and the type name looks like a monomorphized generic (contains "__"),
	// try looking up the base generic class instead
	if !found {
		if idx := strings.Index(typeName, "__"); idx > 0 {
			baseTypeName := typeName[:idx]
			classDef, found = resolveClassForOperatorTrait(baseTypeName, scope)
		}
	}

	if !found || classDef == nil {
		return "", false
	}

	// Check if the class has this trait implemented
	for tName := range classDef.Traits {
		if TraitMatchesOrExtends(tName, traitName) {
			return tName, true
		}
	}

	return "", false
}

func formatHookCandidates(candidates []*hooks.RegisteredHook) string {
	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		label := candidate.TraitName
		if candidate.ModulePath != "" {
			label += " from " + candidate.ModulePath
		}
		names = append(names, label)
	}
	return strings.Join(names, ", ")
}

func (impl *CBackendImplementation) resolveVisibleOperatorHook(scope *ast.Ast, hookType hooks.HookType, pos lexer.Position) (*hooks.RegisteredHook, bool) {
	if scope == nil {
		return nil, false
	}

	root := scope.GetRoot()
	candidates := hooks.GetHookRegistry().GetVisibleHooks(root.Scope, hookType, func(traitName string) bool {
		return !scope.ResolveTrait(traitName).IsNil()
	})
	if len(candidates) == 0 {
		return nil, false
	}
	if len(candidates) > 1 {
		if scope.ErrorScope != nil {
			scope.ErrorScope.NewCompileTimeError(
				"Hook Resolution Error",
				"Multiple visible traits register '"+string(hookType)+"': "+formatHookCandidates(candidates)+"\nhelp: keep only one hook trait for each operator capability visible in this module",
				pos,
			)
		}
		return nil, false
	}
	return candidates[0], true
}

// GetOperatorTraitMethodCall generates a trait method call for an operator
func (impl *CBackendImplementation) GetOperatorTraitMethodCall(
	leftCode string,
	leftType *tokens.TypeRef,
	rightCode string,
	op string,
	scope *ast.Ast,
	pos lexer.Position,
) (string, bool) {
	if leftType == nil {
		return "", false
	}

	typeName := leftType.Type
	if isPrimitiveType(typeName) {
		return "", false
	}

	hookType, ok := hooks.OperatorToHook[op]
	if !ok {
		return "", false
	}

	hook, ok := impl.resolveVisibleOperatorHook(scope, hookType, pos)
	if !ok || hook == nil || len(hook.Methods) == 0 {
		return "", false
	}

	mangledTraitName, found := impl.GetOperatorTraitName(typeName, hook.TraitName, scope)
	if !found {
		return "", false
	}

	// Generate: TypeName__MangledTraitName__methodName(&left, right)
	methodName := typeName + "__" + mangledTraitName + "__" + hook.Methods[0]
	return methodName + "(&(" + leftCode + "), " + rightCode + ")", true
}

// GetUnaryOperatorTraitMethodCall generates a trait method call for a unary operator
func (impl *CBackendImplementation) GetUnaryOperatorTraitMethodCall(
	operandCode string,
	operandType *tokens.TypeRef,
	op string,
	scope *ast.Ast,
	pos lexer.Position,
) (string, bool) {
	if operandType == nil {
		return "", false
	}

	typeName := operandType.Type
	if isPrimitiveType(typeName) {
		return "", false
	}

	hookType, ok := unaryOperatorToHook[op]
	if !ok {
		return "", false
	}

	hook, ok := impl.resolveVisibleOperatorHook(scope, hookType, pos)
	if !ok || hook == nil || len(hook.Methods) == 0 {
		return "", false
	}

	mangledTraitName, found := impl.GetOperatorTraitName(typeName, hook.TraitName, scope)
	if !found {
		return "", false
	}

	// Generate: TypeName__MangledTraitName__methodName(&operand)
	methodName := typeName + "__" + mangledTraitName + "__" + hook.Methods[0]
	return methodName + "(&(" + operandCode + "))", true
}
