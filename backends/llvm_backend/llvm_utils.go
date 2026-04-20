package llvmbackend

import (
	"strconv"

	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir"
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
	var baseType types.Type

	// Handle dynamic array types (unsized arrays as pointers)
	if t.Array != nil {
		baseType = &types.PointerType{
			ElemType: impl.TypeRefGetLLIRType(t.Array, scope),
		}
	} else if t.Size != nil {
		// Handle fixed-size array types: [N]T -> [N x T]
		elemType := impl.TypeRefGetLLIRType(t.Size.Type, scope)
		size, err := strconv.ParseUint(t.Size.Size, 10, 64)
		if err != nil {
			// If size parsing fails, default to 0
			size = 0
		}
		baseType = types.NewArray(size, elemType)
	} else {
		// Try to find primitive type
		prim := findPrimitive(t.Type)
		if prim != nil {
			baseType = prim.Type
		} else {
			// Try to find struct type from LLVMStructMap
			structInfo, ok := LLVMStructMap[t.Type]
			if ok && structInfo.Type != nil {
				baseType = structInfo.Type
			}
		}
	}

	// If the type is a pointer, wrap the base type in a PointerType
	if t.Pointer && baseType != nil {
		return &types.PointerType{
			ElemType: baseType,
		}
	}

	return baseType
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
			Name:       v.Name,
			IsPointer:  v.Type.Pointer,
			IsVolatile: v.Type.Volatile,
			Parent:     mth.Scope,
		}

		(*LLVMProgramValues)[argVariable.GetFullName()] = &LLVMValueInformation{
			Type:       vIrType,
			Value:      def,
			GeckoType:  v.Type,
			IsVolatile: v.Type.Volatile,
		}

		mth.Arguments = append(mth.Arguments, argVariable)
	}
}

func (impl *LLVMBackendImplementation) LLVMImplementationToMethodTokens(scope *ast.Ast, i *tokens.Implementation) []*tokens.Method {
	mTokens := make([]*tokens.Method, 0)

	for _, m := range i.GetFields() {
		mTokens = append(mTokens, m.ToMethodToken())
	}

	return mTokens
}

func (impl *LLVMBackendImplementation) LLVMImplementationForClass(scope *ast.Ast, i *tokens.Implementation) {
	classOpt := scope.ResolveClass(i.GetFor())
	traitOpt := scope.ResolveTrait(i.GetName())

	class := classOpt.UnwrapOrElse(func(err error) *ast.Ast {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the class '"+i.GetFor()+"'", i.Pos)

		return &ast.Ast{}
	})
	traitMthds := traitOpt.UnwrapOrElse(func(err error) *[]*ast.Method {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the trait '"+i.GetName()+"'", i.Pos)

		return &[]*ast.Method{}
	})

	if i.GetFields() != nil && i.Default {
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

	// Build mangled trait name with type arguments for generic traits
	// This matches the C backend behavior to avoid collisions when implementing
	// multiple instantiations of the same generic trait (e.g., Iterator<int32> and Iterator<string>)
	mangledTraitName := i.GetName()
	if len(i.GetTypeArgs()) > 0 {
		for _, typeArg := range i.GetTypeArgs() {
			mangledTraitName += "__" + typeRefToMangledName(typeArg)
		}
	}

	class.Traits[mangledTraitName] = &mthdList
}

// typeRefToMangledName converts a TypeRef to a mangled string for trait keys.
// This ensures consistent naming between C and LLVM backends.
func typeRefToMangledName(t *tokens.TypeRef) string {
	if t == nil {
		return "void"
	}

	base := t.Type
	if t.Pointer {
		base += "_ptr"
	}
	return base
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

	if scope.Config.Arch == i.GetName() {
		for _, m := range impl.LLVMImplementationToMethodTokens(scope, i) {
			scope.Methods[m.Name] = impl.LLVMGetAstMethod(scope, m)
		}
	} else {
		scope.ErrorScope.NewCompileTimeWarning(
			"Arch Implementation",
			"Implementation for the arch '"+i.GetName()+"' was skipped due to target being '"+scope.Config.Arch+"'",
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

// NewVolatileLoad creates a load instruction with the volatile flag set if isVolatile is true.
// This is used for memory-mapped I/O where reads must not be optimized away.
func (impl *LLVMBackendImplementation) NewVolatileLoad(block *ir.Block, elemType types.Type, src value.Value, isVolatile bool) *ir.InstLoad {
	load := block.NewLoad(elemType, src)
	if isVolatile {
		load.Volatile = true
	}
	return load
}

// NewVolatileStore creates a store instruction with the volatile flag set if isVolatile is true.
// This is used for memory-mapped I/O where writes must not be optimized away.
func (impl *LLVMBackendImplementation) NewVolatileStore(block *ir.Block, src value.Value, dst value.Value, isVolatile bool) *ir.InstStore {
	store := block.NewStore(src, dst)
	if isVolatile {
		store.Volatile = true
	}
	return store
}
