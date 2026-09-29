package semantic

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

func (p *Program) ClassForType(typ *tokens.TypeRef) *ClassInfo {
	if typ == nil {
		return nil
	}
	ids := p.classSymbolIDs[typ.Type]
	if typ.Module == "" && len(ids) == 1 {
		return p.classInfosByID[ids[0]]
	}
	for _, id := range ids {
		if symbol := p.SymbolByID(id); symbol != nil && typ.Module != "" && symbol.FullName == typ.Module+"::"+typ.Type {
			return p.classInfosByID[id]
		}
	}
	if len(ids) == 0 && typ.Module == "" {
		return p.classes[typ.Type]
	}
	return nil
}

func (p *Program) FieldsForType(typ *tokens.TypeRef) map[string]*tokens.TypeRef {
	class := p.ClassForType(typ)
	if class == nil {
		return nil
	}
	substitution := classTypeSubst(typ, class.TypeParams)
	fields := make(map[string]*tokens.TypeRef, len(class.Fields))
	for name, field := range class.Fields {
		fields[name] = SubstituteTypeParams(field, substitution)
	}
	return fields
}

func (p *Program) MethodsForType(typ *tokens.TypeRef) []*FunctionSignature {
	if typ == nil {
		return nil
	}
	var substitution map[string]*tokens.TypeRef
	module := typ.Module
	if class := p.ClassForType(typ); class != nil {
		substitution = classTypeSubst(typ, class.TypeParams)
		if symbol := p.SymbolByID(class.SymbolID); symbol != nil {
			module = strings.TrimSuffix(symbol.FullName, "::"+symbol.Name)
		}
	}
	var result []*FunctionSignature
	for _, methods := range p.staticMethods[typ.Type] {
		for _, method := range methods {
			if module != "" && method.Module != module {
				continue
			}
			copy := *method
			copy.ReturnType = resolveSelfType(SubstituteTypeParams(method.ReturnType, substitution), typ.Type)
			copy.Params = make([]*tokens.Value, len(method.Params))
			for i, parameter := range method.Params {
				if parameter == nil {
					continue
				}
				param := *parameter
				param.Type = resolveSelfType(SubstituteTypeParams(parameter.Type, substitution), typ.Type)
				if param.Name == "self" && param.Type == nil {
					param.Type = CloneTypeRef(typ)
				}
				copy.Params[i] = &param
			}
			result = append(result, &copy)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FullName < result[j].FullName })
	return result
}

func (p *Program) SymbolVisibleFrom(id int64, path string) bool {
	symbol := p.SymbolByID(id)
	return symbol != nil && (filepath.Clean(symbol.Pos.Filename) == filepath.Clean(path) || symbol.Visibility == "public" || symbol.Visibility == "external")
}

func (p *Program) FieldsForTypeFrom(typ *tokens.TypeRef, path string) map[string]*tokens.TypeRef {
	fields := p.FieldsForType(typ)
	if class := p.ClassForType(typ); class != nil {
		for name := range fields {
			if !p.SymbolVisibleFrom(class.FieldSymbolIDs[name], path) {
				delete(fields, name)
			}
		}
	}
	return fields
}

func (p *Program) MethodsForTypeFrom(typ *tokens.TypeRef, path string) []*FunctionSignature {
	methods := p.MethodsForType(typ)
	visible := methods[:0]
	for _, method := range methods {
		if p.SymbolVisibleFrom(method.SymbolID, path) {
			visible = append(visible, method)
		}
	}
	return visible
}
