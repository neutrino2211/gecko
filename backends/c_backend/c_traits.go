// spec: spec/traits.md

package cbackend

import (
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
