// spec: spec/traits.md

package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

func traitBaseName(traitName string) string {
	if idx := strings.Index(traitName, "__"); idx >= 0 {
		return traitName[:idx]
	}
	return traitName
}

func traitExtendsTraitRecursive(traitName string, ancestor string, visiting map[string]bool) bool {
	if traitName == "" || ancestor == "" {
		return false
	}
	if visiting[traitName] {
		return false
	}

	traitDef, ok := TraitDefinitions[traitName]
	if !ok || traitDef == nil {
		return false
	}

	visiting[traitName] = true
	defer delete(visiting, traitName)

	for _, parent := range traitDef.AllParents() {
		if parent == ancestor || traitExtendsTraitRecursive(parent, ancestor, visiting) {
			return true
		}
	}

	return false
}

// TraitExtendsTrait reports whether traitName inherits (directly or transitively) from ancestor.
func TraitExtendsTrait(traitName string, ancestor string) bool {
	return traitExtendsTraitRecursive(traitName, ancestor, map[string]bool{})
}

// TraitMatchesOrExtends reports whether an implemented trait key (possibly mangled with type args)
// satisfies a required trait name.
func TraitMatchesOrExtends(implementedTraitName string, requiredTraitName string) bool {
	implementedBase := traitBaseName(implementedTraitName)
	requiredBase := traitBaseName(requiredTraitName)

	if implementedBase == requiredBase {
		return true
	}

	return TraitExtendsTrait(implementedBase, requiredBase)
}

func traitDefinesMethodRecursive(traitName string, methodName string, visiting map[string]bool) bool {
	if traitName == "" || methodName == "" {
		return false
	}
	if visiting[traitName] {
		return false
	}

	traitDef, ok := TraitDefinitions[traitName]
	if !ok || traitDef == nil {
		return false
	}

	visiting[traitName] = true
	defer delete(visiting, traitName)

	for _, field := range traitDef.Fields {
		if field.Name == methodName {
			return true
		}
	}

	for _, parent := range traitDef.AllParents() {
		if traitDefinesMethodRecursive(parent, methodName, visiting) {
			return true
		}
	}

	return false
}

// TraitDefinesMethod reports whether a trait or one of its parent traits defines methodName.
func TraitDefinesMethod(traitName string, methodName string) bool {
	return traitDefinesMethodRecursive(traitName, methodName, map[string]bool{})
}

func collectTraitFieldsRecursive(traitName string, visiting map[string]bool) ([]*tokens.ImplementationField, bool) {
	if traitName == "" {
		return nil, false
	}
	if visiting[traitName] {
		return nil, false
	}

	traitDef, ok := TraitDefinitions[traitName]
	if !ok || traitDef == nil {
		return nil, false
	}

	visiting[traitName] = true
	defer delete(visiting, traitName)

	fields := make([]*tokens.ImplementationField, 0, len(traitDef.Fields))
	indexByMethod := make(map[string]int)

	for _, parent := range traitDef.AllParents() {
		parentFields, ok := collectTraitFieldsRecursive(parent, visiting)
		if !ok {
			return nil, false
		}
		for _, field := range parentFields {
			if idx, exists := indexByMethod[field.Name]; exists {
				fields[idx] = field
				continue
			}
			indexByMethod[field.Name] = len(fields)
			fields = append(fields, field)
		}
	}

	for _, field := range traitDef.Fields {
		if idx, exists := indexByMethod[field.Name]; exists {
			fields[idx] = field
			continue
		}
		indexByMethod[field.Name] = len(fields)
		fields = append(fields, field)
	}

	return fields, true
}

// CollectTraitFields resolves a trait's full method set, including inherited methods.
// Child trait methods override parent methods with the same name.
func CollectTraitFields(traitName string) ([]*tokens.ImplementationField, bool) {
	return collectTraitFieldsRecursive(traitName, map[string]bool{})
}

// FindTraitMethodOwner returns the trait where methodName is declared within traitName's hierarchy.
// Returns empty string if no declaration is found.
func FindTraitMethodOwner(traitName string, methodName string) string {
	return findTraitMethodOwnerRecursive(traitName, methodName, map[string]bool{})
}

func findTraitMethodOwnerRecursive(traitName string, methodName string, visiting map[string]bool) string {
	if traitName == "" || methodName == "" {
		return ""
	}
	if visiting[traitName] {
		return ""
	}

	traitDef, ok := TraitDefinitions[traitName]
	if !ok || traitDef == nil {
		return ""
	}

	visiting[traitName] = true
	defer delete(visiting, traitName)

	for _, field := range traitDef.Fields {
		if field.Name == methodName {
			return traitName
		}
	}

	for _, parent := range traitDef.AllParents() {
		if owner := findTraitMethodOwnerRecursive(parent, methodName, visiting); owner != "" {
			return owner
		}
	}

	return ""
}

// TraitMethodSignature formats an implementation field as a signature string.
func TraitMethodSignature(f *tokens.ImplementationField) string {
	if f == nil {
		return "func(): void"
	}

	args := make([]string, 0, len(f.Arguments))
	for _, arg := range f.Arguments {
		typeStr := "unknown"
		if arg.Type != nil {
			typeStr = typeRefSignature(arg.Type)
		}
		args = append(args, fmt.Sprintf("%s: %s", arg.Name, typeStr))
	}

	ret := "void"
	if f.Type != nil {
		ret = typeRefSignature(f.Type)
	}

	return fmt.Sprintf("func %s(%s): %s", f.Name, strings.Join(args, ", "), ret)
}

// TraitMethodSignaturesCompatible reports whether child can safely override parent.
func TraitMethodSignaturesCompatible(parent *tokens.ImplementationField, child *tokens.ImplementationField) (bool, string) {
	if parent == nil || child == nil {
		return false, "missing method metadata"
	}

	if len(parent.Arguments) != len(child.Arguments) {
		return false, fmt.Sprintf("parameter count mismatch (expected %d, got %d)", len(parent.Arguments), len(child.Arguments))
	}

	for i := 0; i < len(parent.Arguments); i++ {
		parentArg := parent.Arguments[i]
		childArg := child.Arguments[i]

		if parentArg.Name != childArg.Name {
			return false, fmt.Sprintf("parameter %d name mismatch (expected '%s', got '%s')", i+1, parentArg.Name, childArg.Name)
		}

		parentType := typeRefSignature(parentArg.Type)
		childType := typeRefSignature(childArg.Type)
		if parentType != childType {
			return false, fmt.Sprintf("parameter '%s' type mismatch (expected '%s', got '%s')", parentArg.Name, parentType, childType)
		}
	}

	parentRet := typeRefSignature(parent.Type)
	childRet := typeRefSignature(child.Type)
	if parentRet != childRet {
		return false, fmt.Sprintf("return type mismatch (expected '%s', got '%s')", parentRet, childRet)
	}

	return true, ""
}

func typeRefSignature(t *tokens.TypeRef) string {
	if t == nil {
		return "void"
	}

	if t.Array != nil {
		return "[" + typeRefSignature(t.Array) + "]"
	}

	if t.Size != nil {
		return "[" + t.Size.Size + "]" + typeRefSignature(t.Size.Type)
	}

	if t.FuncType != nil {
		paramTypes := make([]string, 0, len(t.FuncType.ParamTypes))
		for _, p := range t.FuncType.ParamTypes {
			paramTypes = append(paramTypes, typeRefSignature(p))
		}
		sig := "func(" + strings.Join(paramTypes, ", ") + ")"
		if t.FuncType.ReturnType != nil {
			sig += ": " + typeRefSignature(t.FuncType.ReturnType)
		}
		if t.FuncType.Throws != nil {
			sig += " throws " + typeRefSignature(t.FuncType.Throws)
		}
		return sig
	}

	var b strings.Builder
	if t.Module != "" {
		b.WriteString(t.Module)
		b.WriteString(".")
	}
	b.WriteString(t.Type)

	if len(t.TypeArgs) > 0 {
		typeArgs := make([]string, 0, len(t.TypeArgs))
		for _, arg := range t.TypeArgs {
			typeArgs = append(typeArgs, typeRefSignature(arg))
		}
		b.WriteString("<")
		b.WriteString(strings.Join(typeArgs, ", "))
		b.WriteString(">")
	}

	if t.Trait != "" {
		b.WriteString(" is ")
		b.WriteString(t.Trait)
	}
	if t.Const {
		b.WriteString("!")
	}
	if t.Volatile {
		b.WriteString(" volatile")
	}
	if t.Pointer {
		b.WriteString("*")
	}
	if t.NonNull {
		b.WriteString("!")
	}

	return b.String()
}
