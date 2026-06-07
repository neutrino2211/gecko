// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"strconv"

	"github.com/alecthomas/participle/v2/lexer"
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
	if t == nil {
		return nil
	}

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
			// Try to resolve user-defined types (structs/enums).
			if classScope := impl.resolveClassFromTypeRef(scope, t); classScope != nil {
				if enumInfo, ok := LLVMEnumMap[classScope.FullScopeName()]; ok && enumInfo != nil && enumInfo.LLVMType != nil {
					baseType = enumInfo.LLVMType
				}
			}

			if baseType == nil {
				// Struct map is keyed by type name as emitted during class lowering.
				structInfo, ok := LLVMStructMap[t.Type]
				if ok && structInfo.Type != nil {
					baseType = structInfo.Type
				}
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

	for !ok {
		if scope.Parent == nil {
			return option.None[*types.Type]()
		}

		scope = *scope.Parent

		if info.LocalContext == nil {
			return option.None[*types.Type]()
		}

		t, ok = info.LocalContext.Types[typ]
	}

	return option.Some(t)
}

func (impl *LLVMBackendImplementation) LLVMAssignArgumentsToMethodArguments(args []*tokens.Value, mth *ast.Method) {
	for _, v := range args {
		if v == nil {
			continue
		}
		if v.Type == nil {
			continue
		}
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

func (impl *LLVMBackendImplementation) resolveTraitOrigin(scope *ast.Ast, traitName string) string {
	if origin, ok := TraitDefinitionOrigins[traitName]; ok && origin != "" {
		return origin
	}

	traitOpt := scope.ResolveTrait(traitName)
	if !traitOpt.IsNil() {
		traitMethods := traitOpt.Unwrap()
		if traitMethods != nil && len(*traitMethods) > 0 {
			first := (*traitMethods)[0]
			if first != nil {
				return first.GetOriginModule()
			}
		}
	}

	return ""
}

func (impl *LLVMBackendImplementation) validateInherentImplCoherence(scope *ast.Ast, className string, classOrigin string, pos lexer.Position) bool {
	currentPackage := scope.GetRoot().Scope
	if classOrigin == "" || classOrigin == currentPackage {
		return true
	}

	typeName := className
	if classOrigin != "" {
		typeName = classOrigin + "." + className
	}

	scope.ErrorScope.NewCompileTimeError(
		"Coherence Error",
		"cannot add inherent impl for foreign type '"+typeName+"'\nhelp: inherent impls are only allowed in the defining package '"+classOrigin+"'",
		pos,
	)
	return false
}

func (impl *LLVMBackendImplementation) validateTraitImplCoherence(scope *ast.Ast, class *ast.Ast, className string, traitName string, pos lexer.Position) bool {
	currentPackage := scope.GetRoot().Scope
	classOrigin := class.GetOriginModule()
	traitOrigin := impl.resolveTraitOrigin(scope, traitName)

	classLocal := classOrigin == "" || classOrigin == currentPackage
	traitLocal := traitOrigin == "" || traitOrigin == currentPackage
	if classLocal || traitLocal {
		return true
	}

	typeName := className
	if classOrigin != "" {
		typeName = classOrigin + "." + className
	}

	qualifiedTraitName := traitName
	if traitOrigin != "" {
		qualifiedTraitName = traitOrigin + "." + traitName
	}

	scope.ErrorScope.NewCompileTimeError(
		"Coherence Error",
		"orphan impl is not allowed: both trait '"+qualifiedTraitName+"' and type '"+typeName+"' are foreign\nhelp: define a local trait or wrap the foreign type in a local newtype",
		pos,
	)
	return false
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

	if !impl.validateTraitImplCoherence(scope, class, i.GetFor(), i.GetName(), i.Pos) {
		return
	}

	var mthdList []*ast.Method

	if i.Default {
		mthdList = *traitMthds
	} else {
		for _, m := range impl.LLVMImplementationToMethodTokens(class, i) {
			mangledName := class.Scope + "__" + i.GetName()
			if len(i.GetTypeArgs()) > 0 {
				for _, typeArg := range i.GetTypeArgs() {
					mangledName += "__" + typeRefToMangledName(typeArg)
				}
			}
			mangledName += "__" + m.Name

			lowered := *m
			lowered.Name = mangledName

			impl.NewMethod(class, &lowered)

			traitMethod, ok := class.Methods[mangledName]
			if !ok || traitMethod == nil {
				scope.ErrorScope.NewCompileTimeError(
					"Implementation Error",
					"unable to register trait method '"+m.Name+"' for trait '"+i.GetName()+"' on type '"+class.Scope+"'",
					i.Pos,
				)
				continue
			}
			mthdList = append(mthdList, traitMethod)
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

func (impl *LLVMBackendImplementation) LLVMInherentImplementation(scope *ast.Ast, i *tokens.Implementation, class *ast.Ast) {
	className := i.GetName()
	if !impl.validateInherentImplCoherence(scope, className, class.GetOriginModule(), i.Pos) {
		return
	}

	for _, f := range i.GetFields() {
		m := f.ToMethodToken()
		if _, exists := class.Methods[m.Name]; exists {
			scope.ErrorScope.NewCompileTimeError(
				"Duplicate Method",
				"Method '"+m.Name+"' already exists on class '"+className+"'. Extensions can only add new methods, not override existing ones.",
				m.Pos,
			)
			continue
		}
		impl.NewMethod(class, m)
	}
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

func (impl *LLVMBackendImplementation) newTraitMethodMetadata(scope *ast.Ast, m *tokens.Method) *ast.Method {
	if m == nil {
		return nil
	}

	returnType := "void"
	if m.Type != nil {
		returnType = m.Type.ToCString(scope)
	}

	return &ast.Method{
		Name:       m.Name,
		Scope:      nil,
		Arguments:  make([]ast.Variable, 0),
		Visibility: m.Visibility,
		Parent:     scope,
		Type:       returnType,
	}
}

func (impl *LLVMBackendImplementation) TraitAssignToScope(scope *ast.Ast, t *tokens.Trait) {
	// Register parent link first so cycle detection can see the current edge.
	TraitParents[t.Name] = t.Parent

	// Validate parent existence and detect simple inheritance cycles.
	if t.Parent != "" {
		if scope.ResolveTrait(t.Parent).IsNil() {
			scope.ErrorScope.NewCompileTimeError(
				"Resolution Error",
				"Could not resolve parent trait '"+t.Parent+"' for trait '"+t.Name+"'",
				t.Pos,
			)
			delete(TraitParents, t.Name)
			return
		}

		seen := map[string]bool{t.Name: true}
		current := t.Parent
		for current != "" {
			if seen[current] {
				scope.ErrorScope.NewCompileTimeError(
					"Trait Inheritance Error",
					"Circular trait inheritance detected involving '"+t.Name+"'",
					t.Pos,
				)
				delete(TraitParents, t.Name)
				return
			}
			seen[current] = true
			current = TraitParents[current]
		}
	}

	mthds := []*ast.Method{}
	indexByName := make(map[string]int)

	// Inherit parent methods first.
	if t.Parent != "" {
		parentTraitOpt := scope.ResolveTrait(t.Parent)
		if !parentTraitOpt.IsNil() {
			parentMethods := parentTraitOpt.Unwrap()
			for _, method := range *parentMethods {
				indexByName[method.Name] = len(mthds)
				mthds = append(mthds, method)
			}
		}
	}

	// Add/override with child methods.
	for _, m := range impl.LLVMTraitGetMethods(t) {
		method := impl.newTraitMethodMetadata(scope, m)
		if method == nil {
			continue
		}
		if idx, exists := indexByName[method.Name]; exists {
			mthds[idx] = method
			continue
		}
		indexByName[method.Name] = len(mthds)
		mthds = append(mthds, method)
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
	if block == nil || src == nil || dst == nil {
		return nil
	}

	dstPtrType, isPointer := dst.Type().(*types.PointerType)
	if !isPointer || dstPtrType.ElemType == nil || src.Type() == nil {
		return nil
	}

	if !types.Equal(src.Type(), dstPtrType.ElemType) {
		return nil
	}

	store := block.NewStore(src, dst)
	if isVolatile {
		store.Volatile = true
	}
	return store
}
