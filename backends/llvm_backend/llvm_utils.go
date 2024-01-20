package llvmbackend

import (
	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/go-option"
)

func loadPrimitives(ast *ast.Ast, localCtx *LocalContext) {
	for _, p := range Primitives {
		ast.Classes[p.Class.Scope] = p.Class

		if localCtx != nil { // In a function, provide LLIR type context
			localCtx.Types[p.Class.FullScopeName()] = &p.Type
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

func (impl *LLVMBackendImplementation) LLVMGetAstMethod(scope *ast.Ast, m *tokens.Method) *ast.Method {
	mth, ok := Methods[scope.FullScopeName()+"#"+m.Name]

	if !ok {
		impl.NewMethod(scope, m)
		mth = Methods[scope.FullScopeName()+"#"+m.Name]
	}

	return mth
}

func (impl *LLVMBackendImplementation) TypeRefGetLLIRType(t *tokens.TypeRef, scope *ast.Ast) types.Type {
	prim := findPrimitive(t.Type)

	if prim != nil {
		return prim.Type
	}

	// TODO: Make LLIR type from compound types
	if t.Array != nil {
		return &types.PointerType{
			ElemType: impl.TypeRefGetLLIRType(t.Array, scope),
		}
	}

	return nil
}

func (impl *LLVMBackendImplementation) LLVMResolveFuncContext(a *ast.Ast, funcName string) *option.Optional[*LocalContext] {
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

func (impl *LLVMBackendImplementation) LLVMResolveLLIRType(a *ast.Ast, typ string) *option.Optional[*types.Type] {
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

func (impl *LLVMBackendImplementation) LLVMAssignArgumentsToMethodArguments(args []*tokens.Value, mth *ast.Method) {
	for _, v := range args {
		var def value.Value = nil

		if v.Default != nil {
			def = impl.ExpressionToLLIRValue(v.Default, mth.Parent, v.Type)
		}

		if mth.Scope != nil {
			v.Type.Check(mth.Scope)
		}

		vIrType := impl.TypeRefGetLLIRType(v.Type, mth.Scope)

		argVariable := ast.Variable{
			Name:      v.Name,
			IsPointer: v.Type.Pointer,
			Parent:    mth.Scope,
		}

		(*LLVMProgramValues)[argVariable.GetFullName()] = &LLVMValueInformation{
			Type:      vIrType,
			Value:     def,
			GeckoType: v.Type,
		}

		mth.Arguments = append(mth.Arguments, argVariable)
	}
}

func (impl *LLVMBackendImplementation) LLVMImplementationToMethodTokens(scope *ast.Ast, i *tokens.Implementation) []*tokens.Method {
	mTokens := make([]*tokens.Method, 0)

	for _, m := range i.Fields {
		mTokens = append(mTokens, m.ToMethodToken())
	}

	return mTokens
}

func (impl *LLVMBackendImplementation) LLVMImplementationForClass(scope *ast.Ast, i *tokens.Implementation) {
	classOpt := scope.ResolveClass(i.For)
	traitOpt := scope.ResolveTrait(i.Name)

	class := classOpt.UnwrapOrElse(func(err error) *ast.Ast {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the class '"+i.For+"'", i.Pos)

		return &ast.Ast{}
	})
	traitMthds := traitOpt.UnwrapOrElse(func(err error) *[]*ast.Method {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the trait '"+i.Name+"'", i.Pos)

		return &[]*ast.Method{}
	})

	if i.Fields != nil && i.Default {
		scope.ErrorScope.NewCompileTimeError(
			"Implementation Error",
			"A default trait implementation must not have a body",
			i.Pos,
		)
	}

	if classOpt.IsNil() || traitOpt.IsNil() {
		return
	}

	var mthdList []*ast.Method

	if i.Default {
		mthdList = *traitMthds
	} else {
		for _, m := range impl.LLVMImplementationToMethodTokens(class, i) {
			mthdList = append(mthdList, impl.LLVMGetAstMethod(scope, m))
		}
	}

	class.Traits[i.Name] = &mthdList
}

func (impl *LLVMBackendImplementation) LLVMImplementationForArch(scope *ast.Ast, i *tokens.Implementation) {
	if i.Default {
		scope.ErrorScope.NewCompileTimeError(
			"Implementation Error",
			"An architecture implementation must not be default",
			i.Pos,
		)

		return
	}

	if scope.Config.Arch == i.Name {
		for _, m := range impl.LLVMImplementationToMethodTokens(scope, i) {
			scope.Methods[m.Name] = impl.LLVMGetAstMethod(scope, m)
		}
	} else {
		scope.ErrorScope.NewCompileTimeWarning(
			"Arch Implementation",
			"Implementation for the arch '"+i.Name+"' was skipped due to target being '"+scope.Config.Arch+"'",
			i.Pos,
		)
	}
}

func (impl *LLVMBackendImplementation) LLVMTraitGetMethods(t *tokens.Trait) []*tokens.Method {
	mthds := make([]*tokens.Method, 0)
	for _, f := range t.Fields {
		mthds = append(mthds, f.ToMethodToken())
	}

	return mthds
}

func (impl *LLVMBackendImplementation) TraitAssignToScope(scope *ast.Ast, t *tokens.Trait) {
	mthds := []*ast.Method{}

	for _, m := range impl.LLVMTraitGetMethods(t) {
		mthds = append(mthds, impl.LLVMGetAstMethod(scope, m))
	}
	scope.Traits[t.Name] = &mthds
}
