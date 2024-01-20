package llvmbackend

import (
	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/go-option"
)

func loadPrimitives(ast *ast.Ast) {
	info := LLVMGetScopeInformation(ast)

	for _, p := range Primitives {
		ast.Classes[p.Class.Scope] = p.Class

		if info.LocalContext != nil { // In a function, provide LLIR type context
			info.LocalContext.Types[p.Class.FullScopeName()] = &p.Type
		}
	}
}

func findPrimitive(name string) *PrimitiveType {
	for _, p := range Primitives {
		if p.Class.Scope == name {
			return p
		}
	}

	return nil
}

func TypeRefGetLLIRType(t *tokens.TypeRef, scope *ast.Ast) types.Type {
	prim := findPrimitive(t.Type)

	if prim != nil {
		return prim.Type
	}

	// TODO: Make LLIR type from compound types
	if t.Array != nil {
		return &types.PointerType{
			ElemType: TypeRefGetLLIRType(t.Array, scope),
		}
	}

	return nil
}

func LLVMResolveFuncContext(a *ast.Ast, funcName string) *option.Optional[*LocalContext] {
	info := LLVMGetScopeInformation(a)
	fnCtx, ok := info.ChildContexts[funcName]

	for !ok {
		if a.Parent == nil {
			return option.None[*LocalContext]()
		}

		a = a.Parent
		fnCtx, ok = info.ChildContexts[funcName]
	}

	return option.Some(fnCtx)
}

func LLVMResolveLLIRType(a *ast.Ast, typ string) *option.Optional[*types.Type] {
	scope := *a

	info := LLVMGetScopeInformation(&scope)

	t, ok := info.LocalContext.Types[typ]
	repr.Println(t, ok, info.LocalContext.Types)

	for !ok {
		if scope.Parent == nil {
			return option.None[*types.Type]()
		}

		scope = *scope.Parent

		if info.LocalContext == nil {
			return option.None[*types.Type]()
		}

		t, ok = info.LocalContext.Types[typ]
		repr.Println(scope.FullScopeName(), info.LocalContext.Types)
	}

	return option.Some(t)
}
