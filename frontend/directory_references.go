package frontend

import "github.com/neutrino2211/gecko/tokens"

type directoryReference struct {
	module string
	name   string
	method bool
}

func directoryReferences(file *tokens.File) []directoryReference {
	if file == nil || len(file.DirectoryImports) == 0 {
		return nil
	}
	localTypes := make(map[string]bool)
	localMethods := make(map[string]bool)
	typeParams := make(map[string]bool)
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		if entry.Class != nil {
			localTypes[entry.Class.Name] = true
		}
		if entry.Trait != nil {
			localTypes[entry.Trait.Name] = true
		}
		if entry.Enum != nil {
			localTypes[entry.Enum.Name] = true
		}
		if entry.Method != nil {
			localMethods[entry.Method.Name] = true
		}
	}
	tokens.WalkSyntaxEntries(file.Entries, func(node any) {
		if param, ok := node.(*tokens.TypeParam); ok {
			typeParams[param.Name] = true
		}
	})
	var references []directoryReference
	seen := make(map[directoryReference]bool)
	add := func(module, name string, method bool) {
		if name == "" || module == "" && !method && (tokens.IsPrimitive(name) || name == "Self" || localTypes[name] || typeParams[name]) {
			return
		}
		if module == "" && method && localMethods[name] {
			return
		}
		ref := directoryReference{module: module, name: name, method: method}
		if seen[ref] {
			return
		}
		seen[ref] = true
		references = append(references, ref)
	}
	tokens.WalkSyntaxEntries(file.Entries, func(node any) {
		switch value := node.(type) {
		case *tokens.TypeRef:
			add(value.Module, value.Type, false)
			add("", value.Trait, false)
		case *tokens.FuncCall:
			if value.StaticType != "" {
				add(value.StaticModule, value.StaticType, false)
			} else {
				add(value.Module, value.Function, true)
			}
		case *tokens.Literal:
			add("", value.StructType, false)
		case *tokens.DestructuringDeclaration:
			add("", value.TypeName, false)
		case *tokens.DestructurePattern:
			add("", value.TypeName, false)
		case *tokens.Implementation:
			add("", value.GetName(), false)
			add("", value.GetFor(), false)
		case *tokens.Trait:
			add("", value.Parent, false)
		case *tokens.TypeParam:
			for _, trait := range value.AllTraits() {
				add("", trait, false)
			}
		case *tokens.WhereConstraint:
			for _, trait := range value.AllTraits() {
				add("", trait, false)
			}
		}
	})
	return references
}
