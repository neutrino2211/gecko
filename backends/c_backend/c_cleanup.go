package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

func newLexicalScope(parent *ast.Ast, loop bool) *ast.Ast {
	outer := CGetScopeInformation(parent)
	outer.ChildSequence++
	scope := &ast.Ast{Scope: fmt.Sprintf("block%d", outer.ChildSequence), Parent: parent}
	scope.Init(parent.ErrorScope)
	scope.Config = parent.Config
	info := CGetScopeInformation(scope)
	info.CurrentFunc = outer.CurrentFunc
	info.CurrentFuncReturnType = outer.CurrentFuncReturnType
	info.ClosureCaptures = outer.ClosureCaptures
	info.LexicalBlock = true
	info.LoopBoundary = loop
	return scope
}

func (impl *CBackendImplementation) processScopedEntries(parent *ast.Ast, entries []*tokens.Entry, loop bool) {
	scope := newLexicalScope(parent, loop)
	for _, entry := range entries {
		impl.processEntry(scope, entry)
	}
	info := CGetScopeInformation(scope)
	impl.emitDefers(scope, info)
	impl.generateDropCalls(scope, info)
	CGetScopeInformation(parent).Code += info.Code
}

func (impl *CBackendImplementation) cleanupThrough(scope *ast.Ast, output *CScopeInformation, loop bool) {
	for current := scope; current != nil; current = current.Parent {
		info := CGetScopeInformation(current)
		impl.emitDefers(current, output)
		impl.generateDropCalls(current, output)
		if info.FunctionBoundary || loop && info.LoopBoundary {
			break
		}
	}
}
