// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md, spec/unsafe.md

package semantic

import (
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) fileKey(file *tokens.File) string {
	if file == nil {
		return ""
	}
	key := ""
	if file.Path != "" {
		key = file.Path
	} else if file.Name != "" {
		key = file.Name
	} else {
		key = fmt.Sprintf("%p", file)
	}
	// Distinguish the same module imported under different aliases so each
	// alias gets its own symbol table entries.
	if file.Alias != "" {
		key += "|as:" + file.Alias
	}
	return key
}

func moduleNameForFile(file *tokens.File) string {
	if file == nil {
		return ""
	}
	if file.Name != "" {
		return file.Name
	}
	return file.PackageName
}

func (a *analyzer) indexFileRecursive(file *tokens.File) {
	if file == nil {
		return
	}
	key := a.fileKey(file)
	if key != "" && a.visitedFiles[key] {
		return
	}
	if key != "" {
		a.visitedFiles[key] = true
	}

	module := moduleNameForFile(file)
	a.indexFileEntries(file, module)

	for _, imported := range file.Imports {
		a.indexFileRecursive(imported)
	}
}

func (a *analyzer) indexFileEntries(file *tokens.File, module string) {
	a.currentFile = file
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		if entry.Import != nil {
			a.indexImportModule(entry.Import)
		}

		if entry.Trait != nil {
			trait := entry.Trait
			a.indexBorrowHooks(trait)
			full := trait.Name
			if module != "" {
				full = module + "::" + trait.Name
			}
			id := a.addSymbol(SymbolTrait, trait.Name, full, &tokens.TypeRef{Type: trait.Name}, trait.Pos)
			a.program.traitSymbolIDs[trait.Name] = append(a.program.traitSymbolIDs[trait.Name], id)
			a.indexTypeParams(trait.TypeParams, id, full)
			for _, member := range trait.Fields {
				if member != nil {
					a.registerTraitMethodSignature(module, trait.Name, member)
				}
			}
			if trait.Parent != "" {
				a.program.traitParents[trait.Name] = trait.Parent
				a.pendingTraitParents = append(a.pendingTraitParents, pendingTraitParent{childID: id, name: trait.Parent, file: file})
			}
		}
		if entry.Enum != nil {
			a.indexEnum(module, entry.Enum)
		}

		if entry.Field != nil {
			full := entry.Field.Name
			if module != "" {
				full = module + "::" + entry.Field.Name
			}
			id := a.addSymbol(SymbolVariable, entry.Field.Name, full, entry.Field.Type, entry.Field.Pos)
			a.program.Symbols[id].Mutability = entry.Field.Mutability
			if entry.Field.IsConstBinding() {
				a.program.Symbols[id].Mutability = "const"
			}
			a.program.globalsByName[entry.Field.Name] = CloneTypeRef(entry.Field.Type)
			a.program.globalSymbolIDs[entry.Field.Name] = id
			a.nameForID[id] = entry.Field.Name
		}

		if entry.Method != nil {
			a.registerFunctionSignature(module, "", entry.Method.Name, entry.Method.Type, entry.Method.Arguments, entry.Method.TypeParams, entry.Method.IsVariadic(), entry.Method.Pos)
		}

		if entry.Declaration != nil && entry.Declaration.Method != nil {
			m := entry.Declaration.Method
			a.registerFunctionSignature(module, "", m.Name, m.Type, m.Arguments, m.TypeParams, m.IsVariadic(), m.Pos, m.Visibility)
		}

		if entry.Foreign != nil {
			for _, member := range entry.Foreign.Members {
				if member == nil || member.Method == nil {
					continue
				}
				m := member.Method
				a.registerFunctionSignature(entry.Foreign.Module, "", m.Name, m.Type, m.Arguments, nil, m.IsVariadic(), m.Pos, "public")
			}
		}

		if entry.Class != nil {
			cls := entry.Class
			full := cls.Name
			if module != "" {
				full = module + "::" + cls.Name
			}
			id := a.addSymbol(SymbolClass, cls.Name, full, &tokens.TypeRef{Type: cls.Name}, cls.Pos)
			a.program.classSymbolIDs[cls.Name] = append(a.program.classSymbolIDs[cls.Name], id)
			a.nameForID[id] = cls.Name
			a.indexTypeParams(cls.TypeParams, id, full)

			classInfo := &ClassInfo{
				Name:           cls.Name,
				SymbolID:       id,
				TypeParams:     cls.TypeParams,
				Fields:         make(map[string]*tokens.TypeRef),
				FieldSymbolIDs: make(map[string]int64),
			}
			for _, field := range cls.Fields {
				if field == nil {
					continue
				}
				if field.Field != nil {
					classInfo.Fields[field.Field.Name] = CloneTypeRef(field.Field.Type)
					fieldFullName := full + "::" + field.Field.Name
					fieldID := a.addSymbol(SymbolField, field.Field.Name, fieldFullName, field.Field.Type, field.Field.Pos)
					classInfo.FieldSymbolIDs[field.Field.Name] = fieldID
					a.program.Symbols[fieldID].Visibility = field.Field.Visibility
				}
				if field.Method != nil {
					a.registerFunctionSignature(module, cls.Name, field.Method.Name, field.Method.Type, field.Method.Arguments, field.Method.TypeParams, field.Method.IsVariadic(), field.Method.Pos, field.Method.Visibility)
				}
			}
			a.program.classes[cls.Name] = classInfo
			a.program.classInfosByID[id] = classInfo
		}

		if entry.Implementation != nil {
			impl := entry.Implementation
			a.indexTypeParams(impl.GetTypeParams(), 0, fmt.Sprintf("%s::impl@%d", module, impl.Pos.Offset))
			if impl.GetFor() != "" && impl.GetName() != "" {
				a.addTraitImpl(impl.GetFor(), impl.GetName())
			}
			ownerType := impl.GetFor()
			if ownerType == "" {
				ownerType = impl.GetName()
			}
			if ownerType != "" {
				for _, field := range impl.GetFields() {
					if field == nil {
						continue
					}
					m := field.ToMethodToken()
					a.registerFunctionSignature(module, ownerType, m.Name, m.Type, m.Arguments, m.TypeParams, m.IsVariadic(), m.Pos, m.Visibility)
				}
			}
		}
	}
}

func (a *analyzer) analyzeFileRecursive(file *tokens.File) {
	if file == nil {
		return
	}

	visited := make(map[string]bool)
	var walk func(*tokens.File)
	walk = func(f *tokens.File) {
		if f == nil {
			return
		}
		key := a.fileKey(f) + "#analyze"
		if visited[key] {
			return
		}
		visited[key] = true

		module := moduleNameForFile(f)
		a.analyzeFileEntries(f, module)
		for _, imported := range f.Imports {
			walk(imported)
		}
	}

	walk(file)
}

func (a *analyzer) analyzeFileEntries(file *tokens.File, module string) {
	a.currentFile = file
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		a.recordEntryTypeRefs(entry)
		if entry.Field != nil && entry.Field.Value == nil && entry.Field.IsConstBinding() {
			a.program.addDiagnostic(Diagnostic{
				Severity: SeverityError,
				Title:    "Uninitialized Constant",
				Message:  "Constant must be initialized with a value",
				Pos:      entry.Field.Pos,
			})
		}
		if entry.Field != nil && entry.Field.Value != nil {
			env := newFlowEnv()
			for name, typ := range a.program.globalsByName {
				env.bind(name, typ, a.program.globalSymbolIDs[name])
			}
			a.inferExpression(entry.Field.Value, env, entry.Field.Type)
		}
		if entry.Method != nil {
			a.analyzeMethod(module, "", entry.Method)
		}
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field != nil && field.Method != nil {
					a.analyzeMethod(module, entry.Class.Name, field.Method)
				}
			}
		}
		if entry.Implementation != nil {
			a.validateImplementationCoherence(entry.Implementation)
			previousTypeParams := a.typeParamScope
			a.pushTypeParams(entry.Implementation.GetTypeParams())
			ownerType := entry.Implementation.GetFor()
			if ownerType == "" {
				ownerType = entry.Implementation.GetName()
			}
			if ownerType == "" {
				a.typeParamScope = previousTypeParams
				continue
			}
			for _, field := range entry.Implementation.GetFields() {
				if field == nil {
					continue
				}
				m := field.ToMethodToken()
				a.analyzeMethod(module, ownerType, m)
			}
			a.typeParamScope = previousTypeParams
		}
	}
}

func (a *analyzer) registerFunctionSignature(module, ownerType, name string, ret *tokens.TypeRef, params []*tokens.Value, typeParams []*tokens.TypeParam, variadic bool, pos lexer.Position, visibility ...string) {
	full := name
	if ownerType != "" {
		if module != "" {
			full = module + "::" + ownerType + "::" + name
		} else {
			full = ownerType + "::" + name
		}
	} else if module != "" {
		full = module + "::" + name
	}

	symbolKind := SymbolFunction
	if ownerType != "" {
		symbolKind = SymbolMethod
	}
	symID := a.addSymbol(symbolKind, name, full, ret, pos)
	if len(visibility) > 0 {
		a.program.Symbols[symID].Visibility = visibility[0]
	}

	sig := &FunctionSignature{
		SymbolID:   symID,
		Name:       name,
		FullName:   full,
		Module:     module,
		OwnerType:  ownerType,
		ReturnType: CloneTypeRef(ret),
		Params:     cloneValues(params),
		TypeParams: cloneTypeParams(typeParams),
		Variadic:   variadic,
	}

	a.program.signatureBySymbolID[symID] = sig

	a.program.functionsByName[name] = append(a.program.functionsByName[name], sig)
	if module != "" {
		if _, ok := a.program.moduleFunctions[module]; !ok {
			a.program.moduleFunctions[module] = make(map[string][]*FunctionSignature)
		}
		a.program.moduleFunctions[module][name] = append(a.program.moduleFunctions[module][name], sig)
	}
	if ownerType != "" {
		if _, ok := a.program.staticMethods[ownerType]; !ok {
			a.program.staticMethods[ownerType] = make(map[string][]*FunctionSignature)
		}
		a.program.staticMethods[ownerType][name] = append(a.program.staticMethods[ownerType][name], sig)
	}

	a.indexTypeParams(typeParams, symID, full)
}

func cloneValues(in []*tokens.Value) []*tokens.Value {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tokens.Value, 0, len(in))
	for _, v := range in {
		if v == nil {
			continue
		}
		out = append(out, &tokens.Value{
			Variadic: v.Variadic,
			Name:     v.Name,
			Out:      v.Out,
			Type:     CloneTypeRef(v.Type),
			Default:  v.Default,
		})
	}
	return out
}

func cloneTypeParams(in []*tokens.TypeParam) []*tokens.TypeParam {
	if len(in) == 0 {
		return nil
	}
	out := make([]*tokens.TypeParam, 0, len(in))
	for _, tp := range in {
		if tp == nil {
			continue
		}
		cloned := &tokens.TypeParam{Name: tp.Name, Trait: tp.Trait}
		if len(tp.Traits) > 0 {
			cloned.Traits = append([]string{}, tp.Traits...)
		}
		out = append(out, cloned)
	}
	return out
}

func (a *analyzer) addTraitImpl(typeName string, traitName string) {
	if typeName == "" || traitName == "" {
		return
	}
	if _, ok := a.program.typeTraits[typeName]; !ok {
		a.program.typeTraits[typeName] = make(map[string]bool)
	}
	a.program.typeTraits[typeName][traitName] = true
}
