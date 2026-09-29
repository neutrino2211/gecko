package semantic

import (
	"path/filepath"
	"sort"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

type scopeSnapshot struct {
	start, end, depth int
	bindings          map[string]*boundVar
}

func thenEnd(end lexer.Position, branch *tokens.ElseIf, otherwise *tokens.Else) lexer.Position {
	if branch != nil {
		return branch.Pos
	}
	if otherwise != nil {
		return otherwise.Pos
	}
	return end
}

func (a *analyzer) recordScope(start, end int, env *flowEnv) {
	if a.currentFile == nil || end <= start {
		return
	}
	bindings := make(map[string]*boundVar, len(env.vars))
	for name, variable := range env.vars {
		copy := *variable
		copy.Type = CloneTypeRef(variable.Type)
		if copy.Type != nil && copy.Type.Pointer && env.nonNull[copy.SymbolID] {
			copy.Type.NonNull = true
		}
		bindings[name] = &copy
	}
	path := filepath.Clean(a.currentFile.Path)
	a.program.scopes[path] = append(a.program.scopes[path], scopeSnapshot{start: start, end: end, depth: a.scopeDepth, bindings: bindings})
}

func (p *Program) scopeAt(path string, offset int) *scopeSnapshot {
	var best *scopeSnapshot
	for i := range p.scopes[filepath.Clean(path)] {
		scope := &p.scopes[filepath.Clean(path)][i]
		if offset < scope.start || offset >= scope.end {
			continue
		}
		if best == nil || scope.depth > best.depth || scope.depth == best.depth && scope.start >= best.start {
			best = scope
		}
	}
	return best
}

func (p *Program) SymbolAt(path string, offset int) *Symbol {
	occurrence, ok := p.OccurrenceAt(path, offset)
	if !ok {
		return nil
	}
	return p.SymbolByID(occurrence.SymbolID)
}

func (p *Program) LookupSymbol(path string, offset int, name string) *Symbol {
	scope := p.scopeAt(path, offset)
	if scope == nil {
		return nil
	}
	return p.boundSymbol(scope.bindings[name])
}

func (p *Program) boundSymbol(variable *boundVar) *Symbol {
	if variable == nil {
		return nil
	}
	symbol := p.SymbolByID(variable.SymbolID)
	if symbol == nil {
		return nil
	}
	copy := *symbol
	copy.Type = CloneTypeRef(variable.Type)
	return &copy
}

func (p *Program) VisibleSymbols(path string, offset int) []*Symbol {
	scope := p.scopeAt(path, offset)
	if scope == nil {
		return nil
	}
	names := make([]string, 0, len(scope.bindings))
	for name := range scope.bindings {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]*Symbol, 0, len(names))
	for _, name := range names {
		if symbol := p.boundSymbol(scope.bindings[name]); symbol != nil {
			result = append(result, symbol)
		}
	}
	return result
}
