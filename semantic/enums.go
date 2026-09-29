package semantic

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) indexEnum(module string, declaration *tokens.Enum) {
	full := declaration.Name
	if module != "" {
		full = module + "::" + full
	}
	id := a.addSymbol(SymbolEnum, declaration.Name, full, &tokens.TypeRef{Type: declaration.Name}, declaration.Pos)
	a.program.enumSymbolIDs[declaration.Name] = append(a.program.enumSymbolIDs[declaration.Name], id)
	a.program.enumCaseIDs[id] = make(map[string]int64)
	if a.currentFile == nil {
		return
	}
	content := a.currentFile.Content
	start := declaration.Pos.Offset
	end := declaration.EndPos.Offset
	if start < 0 || end > len(content) || end <= start {
		return
	}
	open := strings.IndexByte(content[start:end], '{')
	if open < 0 {
		return
	}
	cursor := start + open + 1
	for _, name := range declaration.Cases {
		offset := importObjectOffset(content, cursor, end, name)
		if offset < 0 {
			return
		}
		pos := lexer.Position{Filename: declaration.Pos.Filename, Offset: offset}
		caseID := a.addSymbol(SymbolEnumCase, name, full+"::"+name, &tokens.TypeRef{Type: declaration.Name}, pos)
		a.program.enumCaseIDs[id][name] = caseID
		cursor = offset + len(name)
	}
}

func (a *analyzer) enumSymbolID(name, module string) int64 {
	if name == "" {
		return 0
	}
	preferredModule := module
	if preferredModule == "" {
		preferredModule = moduleNameForFile(a.currentFile)
	}
	var unique int64
	for _, id := range a.program.enumSymbolIDs[name] {
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
		if id := a.explicitlyImportedSymbolID(a.currentFile, name, SymbolEnum); id != 0 {
			return id
		}
	}
	if unique > 0 {
		return unique
	}
	return 0
}

func (a *analyzer) inferEnumLiteral(literal *tokens.Literal) *tokens.TypeRef {
	name := literal.Symbol
	module := ""
	caseIndex := 0
	id := a.enumSymbolID(name, "")
	if id == 0 && len(literal.Chain) > 0 && literal.Chain[0] != nil {
		module = name
		name = literal.Chain[0].Name
		id = a.enumSymbolID(name, module)
		caseIndex = 1
	}
	if id == 0 || len(literal.Chain) != caseIndex+1 {
		return nil
	}
	member := literal.Chain[caseIndex]
	if member == nil || member.IsMethodCall() {
		return nil
	}
	caseID := a.program.enumCaseIDs[id][member.Name]
	if caseID == 0 {
		return nil
	}
	if module != "" {
		a.recordModuleQualifier(module, literal.Pos)
		a.recordOccurrence(id, literal.Chain[0].Pos, name, false)
	} else {
		a.recordOccurrence(id, literal.Pos, name, false)
	}
	a.recordOccurrence(caseID, member.Pos, member.Name, false)
	return &tokens.TypeRef{Type: name, Module: module}
}

func (a *analyzer) inferMatch(match *tokens.Match, env *flowEnv, expected *tokens.TypeRef) *tokens.TypeRef {
	a.inferExpression(match.Scrutinee, env, nil)
	var result *tokens.TypeRef
	for _, matchCase := range match.Cases {
		if matchCase == nil {
			continue
		}
		a.inferMatchPattern(matchCase.Pattern, env)
		a.inferExpression(matchCase.Guard, env, nil)
		if matchCase.Body == nil {
			continue
		}
		if matchCase.Body.Expr != nil {
			inferred := a.inferExpression(matchCase.Body.Expr, env, expected)
			if result == nil {
				result = inferred
			}
		} else {
			a.analyzeEntries(matchCase.Body.Entries, env, matchCase.Body.Pos, matchCase.Body.EndPos)
		}
	}
	return result
}

func (a *analyzer) inferMatchPattern(pattern *tokens.MatchPattern, env *flowEnv) {
	if pattern == nil {
		return
	}
	for _, alternative := range pattern.Alts {
		if alternative == nil || alternative.Inner == nil {
			continue
		}
		inner := alternative.Inner
		if inner.Literal != nil {
			a.inferPrimary(inner.Literal, env, nil)
		}
		if inner.Range != nil {
			a.inferPrimary(inner.Range.Start, env, nil)
			a.inferPrimary(inner.Range.End, env, nil)
		}
		if inner.Destructure != nil {
			a.inferDestructurePattern(inner.Destructure, env)
		}
	}
}

func (a *analyzer) inferDestructurePattern(pattern *tokens.DestructurePattern, env *flowEnv) {
	for _, argument := range pattern.TypeArgs {
		a.recordTypeRef(argument)
	}
	class := a.classInfoForType(&tokens.TypeRef{Type: pattern.TypeName})
	if class != nil {
		a.recordOccurrence(class.SymbolID, pattern.Pos, pattern.TypeName, false)
	}
	for _, field := range pattern.Fields {
		if field == nil {
			continue
		}
		if class != nil {
			a.recordOccurrence(class.FieldSymbolIDs[field.Name], field.Pos, field.Name, false)
		}
		a.inferMatchPattern(field.Pattern, env)
	}
}
