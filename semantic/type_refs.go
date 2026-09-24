// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

func CloneTypeRef(t *tokens.TypeRef) *tokens.TypeRef {
	if t == nil {
		return nil
	}
	out := &tokens.TypeRef{
		Module:   t.Module,
		Type:     t.Type,
		Trait:    t.Trait,
		Const:    t.Const,
		Volatile: t.Volatile,
		Pointer:  t.Pointer,
		NonNull:  t.NonNull,
	}
	if t.Array != nil {
		out.Array = CloneTypeRef(t.Array)
	}
	if t.Size != nil {
		out.Size = &tokens.SizeDef{Size: t.Size.Size, Type: CloneTypeRef(t.Size.Type)}
	}
	if t.FuncType != nil {
		params := make([]*tokens.TypeRef, len(t.FuncType.ParamTypes))
		for i, p := range t.FuncType.ParamTypes {
			params[i] = CloneTypeRef(p)
		}
		out.FuncType = &tokens.FuncType{
			ParamTypes: params,
			ReturnType: CloneTypeRef(t.FuncType.ReturnType),
			Throws:     CloneTypeRef(t.FuncType.Throws),
		}
	}
	if len(t.TypeArgs) > 0 {
		out.TypeArgs = make([]*tokens.TypeRef, len(t.TypeArgs))
		for i, arg := range t.TypeArgs {
			out.TypeArgs[i] = CloneTypeRef(arg)
		}
	}
	return out
}

func SubstituteTypeParams(t *tokens.TypeRef, subst map[string]*tokens.TypeRef) *tokens.TypeRef {
	if t == nil {
		return nil
	}
	if subst != nil {
		if concrete, ok := subst[t.Type]; ok && t.Array == nil && t.Size == nil && t.FuncType == nil {
			resolved := CloneTypeRef(concrete)
			if t.Pointer {
				resolved.Pointer = true
			}
			if t.NonNull {
				resolved.NonNull = true
			}
			if t.Const {
				resolved.Const = true
			}
			if t.Volatile {
				resolved.Volatile = true
			}
			return resolved
		}
	}

	out := CloneTypeRef(t)
	if out.Array != nil {
		out.Array = SubstituteTypeParams(out.Array, subst)
	}
	if out.Size != nil {
		out.Size.Type = SubstituteTypeParams(out.Size.Type, subst)
	}
	if out.FuncType != nil {
		for i, param := range out.FuncType.ParamTypes {
			out.FuncType.ParamTypes[i] = SubstituteTypeParams(param, subst)
		}
		out.FuncType.ReturnType = SubstituteTypeParams(out.FuncType.ReturnType, subst)
		out.FuncType.Throws = SubstituteTypeParams(out.FuncType.Throws, subst)
	}
	if len(out.TypeArgs) > 0 {
		for i, arg := range out.TypeArgs {
			out.TypeArgs[i] = SubstituteTypeParams(arg, subst)
		}
	}
	return out
}

func TypeRefString(t *tokens.TypeRef) string {
	if t == nil {
		return "unknown"
	}
	if t.Array != nil {
		return "[]" + TypeRefString(t.Array)
	}
	if t.Size != nil {
		return fmt.Sprintf("[%s]%s", t.Size.Size, TypeRefString(t.Size.Type))
	}
	if t.FuncType != nil {
		parts := make([]string, 0, len(t.FuncType.ParamTypes))
		for _, p := range t.FuncType.ParamTypes {
			parts = append(parts, TypeRefString(p))
		}
		ret := "void"
		if t.FuncType.ReturnType != nil {
			ret = TypeRefString(t.FuncType.ReturnType)
		}
		return "func(" + strings.Join(parts, ", ") + "): " + ret
	}

	name := t.Type
	if t.Module != "" {
		name = t.Module + "." + name
	}
	if len(t.TypeArgs) > 0 {
		args := make([]string, 0, len(t.TypeArgs))
		for _, arg := range t.TypeArgs {
			args = append(args, TypeRefString(arg))
		}
		name += "<" + strings.Join(args, ", ") + ">"
	}
	if t.Volatile {
		name += " volatile"
	}
	if t.Const {
		name += " readonly"
	}
	if t.Pointer {
		name += "*"
	}
	if t.NonNull {
		name += "!"
	}
	return name
}

func IsNumericType(t *tokens.TypeRef) bool {
	if t == nil {
		return false
	}
	switch t.Type {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float", "float32", "float64":
		return true
	default:
		return false
	}
}

func NormalizeTypeName(name string) string {
	if name == "" {
		return ""
	}
	aliases := map[string]string{
		"float": "float32",
	}
	if normalized, ok := aliases[name]; ok {
		return normalized
	}
	return name
}

func TypesEqual(a, b *tokens.TypeRef) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if NormalizeTypeName(a.Type) != NormalizeTypeName(b.Type) {
		return false
	}
	if a.Module != b.Module || a.Pointer != b.Pointer || a.NonNull != b.NonNull || a.Const != b.Const || a.Volatile != b.Volatile {
		return false
	}
	if (a.Array == nil) != (b.Array == nil) {
		return false
	}
	if a.Array != nil && !TypesEqual(a.Array, b.Array) {
		return false
	}
	if (a.Size == nil) != (b.Size == nil) {
		return false
	}
	if a.Size != nil {
		if a.Size.Size != b.Size.Size || !TypesEqual(a.Size.Type, b.Size.Type) {
			return false
		}
	}
	if len(a.TypeArgs) != len(b.TypeArgs) {
		return false
	}
	for i := range a.TypeArgs {
		if !TypesEqual(a.TypeArgs[i], b.TypeArgs[i]) {
			return false
		}
	}
	return true
}

func TypesCompatible(expected, actual *tokens.TypeRef) bool {
	if expected == nil || actual == nil {
		return true
	}
	if TypesEqual(expected, actual) {
		return true
	}

	// Resolve Self to the current class type
	if CurrentSelfType != "" {
		if expected.Type == "Self" {
			resolved := *expected
			resolved.Type = CurrentSelfType
			return TypesCompatible(&resolved, actual)
		}
		if actual.Type == "Self" {
			resolved := *actual
			resolved.Type = CurrentSelfType
			return TypesCompatible(expected, &resolved)
		}
	}

	if IsNumericType(expected) && IsNumericType(actual) {
		return true
	}

	// Nullable pointer cannot flow into non-null pointer.
	if expected.Pointer && expected.NonNull && (!actual.Pointer || !actual.NonNull) {
		return false
	}
	if expected.Pointer && actual.Pointer {
		if NormalizeTypeName(expected.Type) == "void" || NormalizeTypeName(actual.Type) == "void" {
			return true
		}
		if NormalizeTypeName(expected.Type) == NormalizeTypeName(actual.Type) {
			return true
		}
	}

	// C interop commonly uses both `string` and `string*!` at call boundaries.
	if NormalizeTypeName(expected.Type) == "string" && NormalizeTypeName(actual.Type) == "string" {
		return true
	}

	if NormalizeTypeName(expected.Type) == NormalizeTypeName(actual.Type) && expected.Pointer == actual.Pointer {
		return true
	}

	return false
}

func collectUnresolvedTypeParams(typeParams []*tokens.TypeParam, subst map[string]*tokens.TypeRef) []string {
	var unresolved []string
	for _, tp := range typeParams {
		if tp == nil {
			continue
		}
		if _, ok := subst[tp.Name]; !ok {
			unresolved = append(unresolved, tp.Name)
		}
	}
	sort.Strings(unresolved)
	return unresolved
}
