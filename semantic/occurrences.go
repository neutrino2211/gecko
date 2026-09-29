package semantic

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) addSymbol(kind SymbolKind, name, fullName string, typ *tokens.TypeRef, pos lexer.Position) int64 {
	id := a.program.addSymbol(kind, name, fullName, typ, pos)
	if a.currentFunction != nil {
		a.program.Symbols[id].Local = true
	}
	a.recordOccurrence(id, pos, name, true)
	return id
}

func (a *analyzer) indexTypeParams(params []*tokens.TypeParam, ownerID int64, ownerName string) {
	for _, parameter := range params {
		if parameter == nil {
			continue
		}
		id := a.program.addTypeVar(parameter.Name, parameter.AllTraits(), ownerID)
		a.program.Symbols[id] = &Symbol{
			ID: id, Kind: SymbolTypeParam, Name: parameter.Name,
			FullName: ownerName + "::" + parameter.Name, Pos: parameter.Pos, Local: true,
		}
		a.typeParamIDs[parameter] = id
		a.nameForID[id] = parameter.Name
		a.recordOccurrence(id, parameter.Pos, parameter.Name, true)
	}
}

func (a *analyzer) pushTypeParams(params []*tokens.TypeParam) {
	scope := make(map[string]int64, len(a.typeParamScope)+len(params))
	for name, id := range a.typeParamScope {
		scope[name] = id
	}
	for _, parameter := range params {
		if parameter != nil && a.typeParamIDs[parameter] != 0 {
			scope[parameter.Name] = a.typeParamIDs[parameter]
		}
	}
	a.typeParamScope = scope
}

func (a *analyzer) recordOccurrence(id int64, pos lexer.Position, name string, declaration bool) {
	if id == 0 || a.currentFile == nil || a.currentFile.Path == "" || name == "" {
		return
	}
	content := a.currentFile.Content
	start := pos.Offset
	if start < 0 || start >= len(content) {
		return
	}
	limit := start + 512
	if limit > len(content) {
		limit = len(content)
	}
	for offset := start; offset+len(name) <= limit; offset++ {
		if content[offset] == '/' && offset+1 < limit && content[offset+1] == '/' {
			for offset < limit && content[offset] != '\n' {
				offset++
			}
			continue
		}
		if content[offset] == '"' || content[offset] == '`' {
			quote := content[offset]
			offset++
			for offset < limit && content[offset] != quote {
				if content[offset] == '\\' && quote == '"' {
					offset++
				}
				offset++
			}
			continue
		}
		if content[offset:offset+len(name)] != name {
			continue
		}
		if offset > 0 && identifierByte(content[offset-1]) || offset+len(name) < len(content) && identifierByte(content[offset+len(name)]) {
			continue
		}
		a.program.addOccurrence(Occurrence{
			SymbolID: id, FilePath: filepath.Clean(a.currentFile.Path),
			Start: offset, End: offset + len(name), Declaration: declaration,
		})
		return
	}
}

func (a *analyzer) recordCallOccurrence(id int64, pos lexer.Position, name string) {
	if a.currentFile == nil || pos.Offset < 0 || pos.Offset >= len(a.currentFile.Content) {
		return
	}
	content := a.currentFile.Content
	end := pos.Offset + 512
	if end > len(content) {
		end = len(content)
	}
	if paren := strings.IndexByte(content[pos.Offset:end], '('); paren >= 0 {
		end = pos.Offset + paren
	}
	for offset := end - len(name); offset >= pos.Offset; offset-- {
		if content[offset:offset+len(name)] != name {
			continue
		}
		if offset > 0 && identifierByte(content[offset-1]) || offset+len(name) < len(content) && identifierByte(content[offset+len(name)]) {
			continue
		}
		a.program.addOccurrence(Occurrence{
			SymbolID: id, FilePath: filepath.Clean(a.currentFile.Path),
			Start: offset, End: offset + len(name),
		})
		return
	}
}

func (a *analyzer) recordAssignmentField(assign *tokens.Assignment, env *flowEnv) {
	if assign.Field == "" {
		return
	}
	receiver := env.lookup(assign.Name)
	if receiver == nil || receiver.Type == nil {
		return
	}
	classInfo := a.classInfoForType(receiver.Type)
	if classInfo == nil {
		return
	}
	a.recordOccurrence(classInfo.FieldSymbolIDs[assign.Field], assign.Pos, assign.Field, false)
}

func (a *analyzer) inferStructLiteral(l *tokens.Literal) *tokens.TypeRef {
	for _, argument := range l.StructTypeArgs {
		a.recordTypeRef(argument)
	}
	if classInfo := a.classInfoForType(&tokens.TypeRef{Type: l.StructType}); classInfo != nil {
		a.recordOccurrence(classInfo.SymbolID, l.Pos, l.StructType, false)
		for _, field := range l.StructFields {
			if field == nil {
				continue
			}
			if _, ok := classInfo.Fields[field.Key]; !ok {
				a.program.addDiagnostic(Diagnostic{
					Severity: SeverityError,
					Title:    "Unknown Field",
					Message:  "Struct '" + l.StructType + "' has no field named '" + field.Key + "'",
					Pos:      field.Pos,
				})
				continue
			}
			a.recordOccurrence(classInfo.FieldSymbolIDs[field.Key], field.Pos, field.Key, false)
		}
	}
	return &tokens.TypeRef{Type: l.StructType, TypeArgs: cloneTypeRefSlice(l.StructTypeArgs)}
}

func (a *analyzer) recordTypeRef(typ *tokens.TypeRef) {
	if typ == nil {
		return
	}
	if typ.Trait != "" {
		a.recordConstraintTraits(typ.Pos, typ.EndPos, []string{typ.Trait})
	}
	if typ.Module != "" {
		a.recordModuleQualifier(typ.Module, typ.Pos)
	}
	if id := a.typeParamScope[typ.Type]; id != 0 && typ.Module == "" {
		a.recordOccurrence(id, typ.Pos, typ.Type, false)
	} else if id := a.classSymbolID(typ.Type, typ.Module); id != 0 {
		a.recordOccurrence(id, typ.Pos, typ.Type, false)
	} else if id := a.traitSymbolID(typ.Type, typ.Module); id != 0 {
		a.recordOccurrence(id, typ.Pos, typ.Type, false)
	} else if id := a.enumSymbolID(typ.Type, typ.Module); id != 0 {
		a.recordOccurrence(id, typ.Pos, typ.Type, false)
	}
	a.recordTypeRef(typ.Array)
	if typ.Size != nil {
		a.recordTypeRef(typ.Size.Type)
	}
	for _, argument := range typ.TypeArgs {
		a.recordTypeRef(argument)
	}
	if typ.FuncType != nil {
		for _, parameter := range typ.FuncType.ParamTypes {
			a.recordTypeRef(parameter)
		}
		a.recordTypeRef(typ.FuncType.ReturnType)
		a.recordTypeRef(typ.FuncType.Throws)
	}
}

func (a *analyzer) classSymbolID(name, module string) int64 {
	if name == "" {
		return 0
	}
	preferredModule := module
	if preferredModule == "" {
		preferredModule = moduleNameForFile(a.currentFile)
	}
	var unique int64
	for _, id := range a.program.classSymbolIDs[name] {
		symbol := a.program.SymbolByID(id)
		if symbol == nil {
			continue
		}
		if preferredModule != "" && symbol.FullName == preferredModule+"::"+name {
			return id
		}
		if module != "" {
			continue
		}
		if unique != 0 {
			unique = -1
		} else {
			unique = id
		}
	}
	if module == "" {
		if id := a.explicitlyImportedSymbolID(a.currentFile, name, SymbolClass); id != 0 {
			return id
		}
	}
	if unique > 0 {
		return unique
	}
	return 0
}

func (a *analyzer) classInfoForType(typ *tokens.TypeRef) *ClassInfo {
	if typ == nil {
		return nil
	}
	if id := a.classSymbolID(typ.Type, typ.Module); id != 0 {
		return a.program.classInfosByID[id]
	}
	if typ.Module == "" && len(a.program.classSymbolIDs[typ.Type]) == 0 {
		return a.program.classes[typ.Type]
	}
	return nil
}

func (a *analyzer) recordConstraintTraits(start, end lexer.Position, names []string) {
	if a.currentFile == nil || len(names) == 0 {
		return
	}
	content := a.currentFile.Content
	if start.Offset < 0 || end.Offset > len(content) || end.Offset <= start.Offset {
		return
	}
	keyword := importObjectOffset(content, start.Offset, end.Offset, "is")
	if keyword < 0 {
		return
	}
	cursor := keyword + len("is")
	for _, name := range names {
		offset := importObjectOffset(content, cursor, end.Offset, name)
		if offset < 0 {
			return
		}
		if id := a.traitSymbolID(name, ""); id != 0 {
			a.program.addOccurrence(Occurrence{
				SymbolID: id, FilePath: filepath.Clean(a.currentFile.Path),
				Start: offset, End: offset + len(name),
			})
		}
		cursor = offset + len(name)
	}
}

func (a *analyzer) recordTypeParamTraits(params []*tokens.TypeParam) {
	for _, param := range params {
		if param != nil {
			a.recordConstraintTraits(param.Pos, param.EndPos, param.AllTraits())
		}
	}
}

func (a *analyzer) recordWhereConstraintTraits(where *tokens.WhereClause) {
	if where == nil {
		return
	}
	for _, constraint := range where.Constraints {
		if constraint != nil {
			if id := a.typeParamScope[constraint.Name]; id != 0 {
				a.recordOccurrence(id, constraint.Pos, constraint.Name, false)
			}
			a.recordConstraintTraits(constraint.Pos, constraint.EndPos, constraint.AllTraits())
		}
	}
}

func (a *analyzer) traitSymbolID(name, module string) int64 {
	ids := a.program.traitSymbolIDs[name]
	if len(ids) == 0 {
		return 0
	}
	explicitModule := module != ""
	if module == "" {
		module = moduleNameForFile(a.currentFile)
	}
	if module != "" {
		full := module + "::" + name
		for _, id := range ids {
			if symbol := a.program.SymbolByID(id); symbol != nil && symbol.FullName == full {
				return id
			}
		}
	}
	if explicitModule {
		return 0
	}
	if id := a.explicitlyImportedSymbolID(a.currentFile, name, SymbolTrait); id != 0 {
		return id
	}
	if len(ids) == 1 {
		return ids[0]
	}
	return 0
}

func (a *analyzer) recordMethodTypeRefs(method *tokens.Method) {
	if method == nil {
		return
	}
	previousTypeParams := a.typeParamScope
	a.pushTypeParams(method.TypeParams)
	defer func() { a.typeParamScope = previousTypeParams }()
	a.recordTypeParamTraits(method.TypeParams)
	a.recordWhereConstraintTraits(method.Where)
	for _, argument := range method.Arguments {
		if argument != nil {
			a.recordTypeRef(argument.Type)
		}
	}
	a.recordTypeRef(method.Type)
	a.recordTypeRef(method.Throws)
}

func (a *analyzer) recordEntryTypeRefs(entry *tokens.Entry) {
	if entry.Import != nil {
		a.recordImportObjectUses(entry.Import)
	}
	if entry.Field != nil {
		a.recordTypeRef(entry.Field.Type)
	}
	a.recordMethodTypeRefs(entry.Method)
	if entry.Class != nil {
		previousTypeParams := a.typeParamScope
		a.pushTypeParams(entry.Class.TypeParams)
		a.recordTypeParamTraits(entry.Class.TypeParams)
		a.recordWhereConstraintTraits(entry.Class.Where)
		for _, field := range entry.Class.Fields {
			if field == nil {
				continue
			}
			if field.Field != nil {
				a.recordTypeRef(field.Field.Type)
			}
			a.recordMethodTypeRefs(field.Method)
		}
		a.typeParamScope = previousTypeParams
	}
	if entry.Trait != nil {
		previousTypeParams := a.typeParamScope
		a.pushTypeParams(entry.Trait.TypeParams)
		a.recordTypeParamTraits(entry.Trait.TypeParams)
		if entry.Trait.Parent != "" {
			a.recordOccurrence(a.traitSymbolID(entry.Trait.Parent, ""), entry.Trait.Pos, entry.Trait.Parent, false)
		}
		for _, field := range entry.Trait.Fields {
			if field != nil {
				a.recordMethodTypeRefs(field.ToMethodToken())
			}
		}
		a.typeParamScope = previousTypeParams
	}
	if entry.Implementation != nil {
		implementation := entry.Implementation
		previousTypeParams := a.typeParamScope
		a.pushTypeParams(implementation.GetTypeParams())
		a.recordTypeParamTraits(implementation.GetTypeParams())
		for _, argument := range implementation.GetTypeArgs() {
			a.recordTypeRef(argument)
		}
		for _, argument := range implementation.GetForTypeArgs() {
			a.recordTypeRef(argument)
		}
		a.recordOccurrence(a.traitSymbolID(implementation.GetName(), ""), implementation.Pos, implementation.GetName(), false)
		if id := a.classSymbolID(implementation.GetFor(), ""); id != 0 {
			a.recordOccurrence(id, implementation.Pos, implementation.GetFor(), false)
		} else if id := a.classSymbolID(implementation.GetName(), ""); id != 0 {
			a.recordOccurrence(id, implementation.Pos, implementation.GetName(), false)
		}
		for _, field := range entry.Implementation.GetFields() {
			if field != nil {
				a.recordMethodTypeRefs(field.ToMethodToken())
			}
		}
		a.typeParamScope = previousTypeParams
	}
	if entry.Declaration != nil {
		a.recordMethodTypeRefs(entry.Declaration.Method)
		if entry.Declaration.Field != nil {
			a.recordTypeRef(entry.Declaration.Field.Type)
		}
	}
}

func identifierByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func (p *Program) addOccurrence(occurrence Occurrence) {
	if p.occurrenceSet[occurrence] {
		return
	}
	p.occurrenceSet[occurrence] = true
	p.occurrences = append(p.occurrences, occurrence)
	p.occurrencesByFile[occurrence.FilePath] = append(p.occurrencesByFile[occurrence.FilePath], occurrence)
	p.occurrencesBySymbol[occurrence.SymbolID] = append(p.occurrencesBySymbol[occurrence.SymbolID], occurrence)
}

func (p *Program) OccurrenceAt(path string, offset int) (Occurrence, bool) {
	path = filepath.Clean(path)
	for _, occurrence := range p.occurrencesByFile[path] {
		if occurrence.Start <= offset && offset < occurrence.End {
			return occurrence, true
		}
	}
	return Occurrence{}, false
}

func (p *Program) OccurrencesFor(id int64, includeDeclaration bool) []Occurrence {
	result := make([]Occurrence, 0)
	for _, occurrence := range p.occurrencesBySymbol[id] {
		if includeDeclaration || !occurrence.Declaration {
			result = append(result, occurrence)
		}
	}
	return result
}

func (p *Program) OccurrencesInFile(path string) []Occurrence {
	return append([]Occurrence{}, p.occurrencesByFile[filepath.Clean(path)]...)
}
