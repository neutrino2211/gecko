// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

type llvmChainState struct {
	Value     value.Value
	GeckoType *tokens.TypeRef
}

type llvmResolvedMethod struct {
	Method    *ast.Method
	TraitName string
}

func cloneTypeRef(t *tokens.TypeRef) *tokens.TypeRef {
	if t == nil {
		return nil
	}
	cloned := *t
	return &cloned
}

func appendUniqueNonEmpty(values []string, candidate string) []string {
	if candidate == "" {
		return values
	}
	for _, existing := range values {
		if existing == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func llvmTraitBaseName(traitName string) string {
	if idx := strings.Index(traitName, "__"); idx >= 0 {
		return traitName[:idx]
	}
	return traitName
}

func llvmMethodBaseName(methodName string) string {
	if idx := strings.LastIndex(methodName, "__"); idx >= 0 {
		return methodName[idx+2:]
	}
	return methodName
}

func (impl *LLVMBackendImplementation) resolveClassByName(scope *ast.Ast, moduleName string, className string) *ast.Ast {
	if className == "" {
		return nil
	}

	if moduleName != "" {
		root := scope.GetRoot()
		moduleScope, ok := root.Children[moduleName]
		if !ok {
			return nil
		}
		classOpt := moduleScope.ResolveClass(className)
		if classOpt.IsNil() {
			return nil
		}
		return classOpt.Unwrap()
	}

	classOpt := scope.ResolveClass(className)
	if classOpt.IsNil() {
		return nil
	}
	return classOpt.Unwrap()
}

func (impl *LLVMBackendImplementation) resolveClassFromTypeRef(scope *ast.Ast, typeRef *tokens.TypeRef) *ast.Ast {
	if typeRef == nil {
		return nil
	}
	return impl.resolveClassByName(scope, typeRef.Module, typeRef.Type)
}

func (impl *LLVMBackendImplementation) resolveEnumInfo(scope *ast.Ast, moduleName string, enumName string) (*ast.Ast, *LLVMEnumInfo, bool) {
	enumScope := impl.resolveClassByName(scope, moduleName, enumName)
	if enumScope == nil {
		return nil, nil, false
	}

	info, ok := LLVMEnumMap[enumScope.FullScopeName()]
	if !ok || info == nil {
		return nil, nil, false
	}

	return enumScope, info, true
}

func (impl *LLVMBackendImplementation) resolveEnumCaseConstant(scope *ast.Ast, enumName string, chain []*tokens.ChainAccess, pos lexer.Position) (value.Value, bool) {
	if len(chain) == 0 {
		return nil, false
	}

	_, enumInfo, ok := impl.resolveEnumInfo(scope, "", enumName)
	if !ok {
		return nil, false
	}

	first := chain[0]
	if first.IsMethodCall() {
		scope.ErrorScope.NewCompileTimeError(
			"Variable Resolution Error",
			fmt.Sprintf("Unable to resolve enum case '%s.%s'", enumName, first.Name),
			pos,
		)
		return nil, true
	}

	caseValue, hasCase := enumInfo.Cases[first.Name]
	if !hasCase {
		scope.ErrorScope.NewCompileTimeError(
			"Variable Resolution Error",
			fmt.Sprintf("Unable to resolve enum case '%s.%s'", enumName, first.Name),
			pos,
		)
		return nil, true
	}

	if len(chain) > 1 {
		scope.ErrorScope.NewCompileTimeError(
			"Variable Resolution Error",
			fmt.Sprintf("Enum case access does not support chained access beyond '%s.%s'", enumName, first.Name),
			pos,
		)
		return nil, true
	}

	return constant.NewInt(enumInfo.LLVMType, caseValue), true
}

func (impl *LLVMBackendImplementation) resolveModuleChain(scope *ast.Ast, moduleScope *ast.Ast, moduleName string, chain []*tokens.ChainAccess, pos lexer.Position) value.Value {
	if len(chain) == 0 {
		scope.ErrorScope.NewCompileTimeError("Variable Resolution Error", "Unable to resolve the variable '"+moduleName+"'", pos)
		return nil
	}

	first := chain[0]
	var base value.Value

	if first.IsMethodCall() {
		moduleCall := &tokens.FuncCall{
			Module:    moduleName,
			Function:  first.Name,
			Arguments: first.GetArgs(),
		}
		impl.FuncCall(scope, moduleCall)
		call := FuncCalls[llvmFuncCallCacheKey(scope, moduleCall)]
		if call == nil {
			scope.ErrorScope.NewCompileTimeError(
				"Function resolution error",
				fmt.Sprintf("Unable to resolve the function \"%s.%s\"", moduleName, first.Name),
				pos,
			)
			return nil
		}
		base = call
	} else {
		moduleVariable := moduleScope.ResolveSymbolAsVariable(first.Name)
		if moduleVariable.IsNil() {
			scope.ErrorScope.NewCompileTimeError(
				"Variable Resolution Error",
				"Unable to resolve module symbol '"+moduleName+"."+first.Name+"'",
				pos,
			)
			return nil
		}
		base = LLVMGetValueInformation(moduleVariable.Unwrap()).Value
	}

	if len(chain) > 1 {
		scope.ErrorScope.NewCompileTimeError(
			"Not Implemented",
			"LLVM backend does not yet support nested module chain access beyond one hop",
			pos,
		)
	}

	return base
}

func findIRFuncByName(module *ir.Module, names ...string) *ir.Func {
	if module == nil {
		return nil
	}

	for _, fn := range module.Funcs {
		for _, name := range names {
			if name == "" {
				continue
			}
			if fn.Name() == name {
				return fn
			}
		}
	}

	return nil
}

func (impl *LLVMBackendImplementation) methodReturnTypeFromString(scope *ast.Ast, raw string) types.Type {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "void" {
		return VoidType.Type
	}

	typeRef := &tokens.TypeRef{Type: trimmed}
	if strings.HasSuffix(trimmed, "*") {
		typeRef.Type = strings.TrimSpace(strings.TrimSuffix(trimmed, "*"))
		typeRef.Pointer = true
	}

	irType := impl.TypeRefGetLLIRType(typeRef, scope)
	if irType != nil {
		return irType
	}

	return VoidType.Type
}

func (impl *LLVMBackendImplementation) resolveMethodIRFunction(scope *ast.Ast, method *ast.Method, fallbackNames ...string) *ir.Func {
	info := LLVMGetScopeInformation(scope)
	module := info.ProgramContext.Module

	candidates := make([]string, 0, len(fallbackNames)+2)
	if method != nil {
		candidates = appendUniqueNonEmpty(candidates, method.Name)
		candidates = appendUniqueNonEmpty(candidates, method.GetFullName())
	}
	for _, name := range fallbackNames {
		candidates = appendUniqueNonEmpty(candidates, name)
	}

	if method != nil {
		if methodInfo, ok := (*LLVMScopeDataMap)[method.GetFullName()]; ok && methodInfo != nil && methodInfo.LocalContext != nil && methodInfo.LocalContext.Func != nil {
			return methodInfo.LocalContext.Func
		}

		if fn := findIRFuncByName(module, candidates...); fn != nil {
			return fn
		}

		retType := impl.methodReturnTypeFromString(scope, method.Type)
		symbolName := method.Name
		if len(candidates) > 0 {
			symbolName = candidates[0]
		}
		fn := ir.NewFunc(symbolName, retType)
		fn.Linkage = enum.LinkageExternal
		fn.CallingConv = CallingConventions[scope.Config.Arch][scope.Config.Platform]
		return fn
	}

	if fn := findIRFuncByName(module, candidates...); fn != nil {
		return fn
	}

	symbolName := "method_call"
	if len(candidates) > 0 {
		symbolName = candidates[0]
	}

	fn := ir.NewFunc(symbolName, VoidType.Type)
	fn.Linkage = enum.LinkageExternal
	fn.CallingConv = CallingConventions[scope.Config.Arch][scope.Config.Platform]
	return fn
}

func (impl *LLVMBackendImplementation) ensureReceiverPointer(scope *ast.Ast, receiver value.Value) value.Value {
	if receiver == nil {
		return nil
	}

	info := LLVMGetScopeInformation(scope)
	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		return nil
	}
	block := info.LocalContext.MainBlock

	if ptrType, isPtr := receiver.Type().(*types.PointerType); isPtr {
		if nestedPtr, nested := ptrType.ElemType.(*types.PointerType); nested {
			return impl.NewVolatileLoad(block, nestedPtr, receiver, false)
		}
		return receiver
	}

	tmp := block.NewAlloca(receiver.Type())
	if impl.NewVolatileStore(block, receiver, tmp, false) == nil {
		return nil
	}
	return tmp
}

func (impl *LLVMBackendImplementation) ensureStructPointer(scope *ast.Ast, receiver value.Value, pos lexer.Position) value.Value {
	if receiver == nil {
		scope.ErrorScope.NewCompileTimeError("Field Access Error", "Unable to evaluate receiver for field access", pos)
		return nil
	}

	info := LLVMGetScopeInformation(scope)
	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Field Access Error", "field access must be inside a function body", pos)
		return nil
	}
	block := info.LocalContext.MainBlock

	if ptrType, isPtr := receiver.Type().(*types.PointerType); isPtr {
		if _, ok := ptrType.ElemType.(*types.StructType); ok {
			return receiver
		}
		if nestedPtr, ok := ptrType.ElemType.(*types.PointerType); ok {
			if _, nestedStruct := nestedPtr.ElemType.(*types.StructType); nestedStruct {
				return impl.NewVolatileLoad(block, nestedPtr, receiver, false)
			}
		}
	}

	if _, isStructValue := receiver.Type().(*types.StructType); isStructValue {
		tmp := block.NewAlloca(receiver.Type())
		if impl.NewVolatileStore(block, receiver, tmp, false) == nil {
			scope.ErrorScope.NewCompileTimeError("Field Access Error", "unable to materialize temporary storage for struct field access", pos)
			return nil
		}
		return tmp
	}

	scope.ErrorScope.NewCompileTimeError("Field Access Error", "field access requires a struct receiver", pos)
	return nil
}

func (impl *LLVMBackendImplementation) lowerFieldAccess(scope *ast.Ast, state llvmChainState, chain *tokens.ChainAccess, terminal bool) (llvmChainState, bool) {
	classScope := impl.resolveClassFromTypeRef(scope, state.GeckoType)
	if classScope == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Field Access Error",
			fmt.Sprintf("Unable to resolve receiver type for field '%s'", chain.Name),
			chain.Pos,
		)
		return llvmChainState{}, false
	}

	structInfo, ok := LLVMStructMap[classScope.Scope]
	if !ok || structInfo == nil || structInfo.Type == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Field Access Error",
			fmt.Sprintf("Unable to resolve struct layout for type '%s'", classScope.Scope),
			chain.Pos,
		)
		return llvmChainState{}, false
	}

	fieldIndex := -1
	for idx, fieldName := range structInfo.FieldNames {
		if fieldName == chain.Name {
			fieldIndex = idx
			break
		}
	}
	if fieldIndex < 0 {
		scope.ErrorScope.NewCompileTimeError(
			"Field Access Error",
			fmt.Sprintf("Type '%s' has no field '%s'", classScope.Scope, chain.Name),
			chain.Pos,
		)
		return llvmChainState{}, false
	}

	structPtr := impl.ensureStructPointer(scope, state.Value, chain.Pos)
	if structPtr == nil {
		return llvmChainState{}, false
	}

	info := LLVMGetScopeInformation(scope)
	block := info.LocalContext.MainBlock
	zero := constant.NewInt(types.I32, 0)
	fieldIdxConst := constant.NewInt(types.I32, int64(fieldIndex))
	fieldPtr := block.NewGetElementPtr(structInfo.Type, structPtr, zero, fieldIdxConst)

	var fieldType *tokens.TypeRef
	if fieldIndex < len(structInfo.FieldTypes) && structInfo.FieldTypes[fieldIndex] != nil {
		fieldType = cloneTypeRef(structInfo.FieldTypes[fieldIndex])
	} else if fieldVar, hasField := classScope.Variables[chain.Name]; hasField {
		fieldType = &tokens.TypeRef{
			Pointer:  fieldVar.IsPointer,
			Volatile: fieldVar.IsVolatile,
			Const:    fieldVar.IsConst,
		}
	}

	fieldIRType := types.Type(nil)
	if fieldType != nil {
		fieldIRType = impl.TypeRefGetLLIRType(fieldType, scope)
	}
	if fieldIRType == nil {
		if ptrType, isPtr := fieldPtr.Type().(*types.PointerType); isPtr {
			fieldIRType = ptrType.ElemType
		}
	}
	if fieldIRType == nil {
		scope.ErrorScope.NewCompileTimeError("Field Access Error", "unable to lower LLVM field type", chain.Pos)
		return llvmChainState{}, false
	}

	isVolatile := fieldType != nil && fieldType.IsVolatile()
	if terminal {
		loaded := impl.NewVolatileLoad(block, fieldIRType, fieldPtr, isVolatile)
		return llvmChainState{
			Value:     loaded,
			GeckoType: fieldType,
		}, true
	}

	if fieldType != nil && fieldType.Pointer {
		loadedPtr := impl.NewVolatileLoad(block, fieldIRType, fieldPtr, isVolatile)
		return llvmChainState{
			Value:     loadedPtr,
			GeckoType: fieldType,
		}, true
	}

	nextType := cloneTypeRef(fieldType)
	if nextType != nil {
		nextType.Pointer = true
	}

	return llvmChainState{
		Value:     fieldPtr,
		GeckoType: nextType,
	}, true
}

func (impl *LLVMBackendImplementation) traitDeclaresMethod(classScope *ast.Ast, traitName string, methodName string) bool {
	if classScope == nil || methodName == "" {
		return false
	}

	traitOpt := classScope.ResolveTrait(llvmTraitBaseName(traitName))
	if traitOpt.IsNil() {
		return false
	}

	traitMethods := traitOpt.Unwrap()
	if traitMethods == nil {
		return false
	}

	for _, method := range *traitMethods {
		if method == nil {
			continue
		}
		if llvmMethodBaseName(method.Name) == methodName {
			return true
		}
	}

	return false
}

func (impl *LLVMBackendImplementation) resolveClassFromTypeRefForMethod(scope *ast.Ast, typeRef *tokens.TypeRef, methodName string, pos lexer.Position) *ast.Ast {
	if typeRef == nil || typeRef.Type == "" {
		scope.ErrorScope.NewCompileTimeError(
			"Function resolution error",
			fmt.Sprintf("Unable to resolve receiver type for method '%s'", methodName),
			pos,
		)
		return nil
	}

	if typeRef.Module != "" {
		rootScope := scope.GetRoot()
		moduleScope, ok := rootScope.Children[typeRef.Module]
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Resolution Error",
				fmt.Sprintf("Could not resolve module '%s' while resolving receiver type '%s.%s' for method '%s'", typeRef.Module, typeRef.Module, typeRef.Type, methodName),
				pos,
			)
			return nil
		}

		classOpt := moduleScope.ResolveClass(typeRef.Type)
		if classOpt.IsNil() {
			scope.ErrorScope.NewCompileTimeError(
				"Resolution Error",
				fmt.Sprintf("Could not resolve type '%s.%s' while resolving method '%s'", typeRef.Module, typeRef.Type, methodName),
				pos,
			)
			return nil
		}

		return classOpt.Unwrap()
	}

	classOpt := scope.ResolveClass(typeRef.Type)
	if classOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError(
			"Function resolution error",
			fmt.Sprintf("Unable to resolve receiver type '%s' for method '%s'", typeRef.Type, methodName),
			pos,
		)
		return nil
	}

	return classOpt.Unwrap()
}

func (impl *LLVMBackendImplementation) resolveMethodOnClass(scope *ast.Ast, classScope *ast.Ast, methodName string, pos lexer.Position) *llvmResolvedMethod {
	if classScope == nil {
		return nil
	}

	if method, ok := classScope.Methods[methodName]; ok && method != nil {
		return &llvmResolvedMethod{Method: method}
	}

	matches := make([]llvmResolvedMethod, 0, 1)
	declaredButMissing := make([]string, 0)
	for traitName, traitMethods := range classScope.Traits {
		foundInTrait := false
		if traitMethods != nil {
			for _, method := range *traitMethods {
				if method == nil {
					continue
				}
				if method.Name == methodName || llvmMethodBaseName(method.Name) == methodName {
					matches = append(matches, llvmResolvedMethod{
						Method:    method,
						TraitName: traitName,
					})
					foundInTrait = true
					break
				}
			}
		}

		if !foundInTrait && impl.traitDeclaresMethod(classScope, traitName, methodName) {
			declaredButMissing = appendUniqueNonEmpty(declaredButMissing, traitName)
		}
	}

	if len(matches) > 1 {
		traitNames := make([]string, 0, len(matches))
		for _, match := range matches {
			traitNames = appendUniqueNonEmpty(traitNames, match.TraitName)
		}
		scope.ErrorScope.NewCompileTimeError(
			"Function resolution error",
			fmt.Sprintf("Method \"%s\" is ambiguous on type \"%s\" across traits [%s]", methodName, classScope.Scope, strings.Join(traitNames, ", ")),
			pos,
		)
		return nil
	}

	if len(matches) == 1 {
		match := matches[0]
		return &match
	}

	if len(declaredButMissing) > 0 {
		scope.ErrorScope.NewCompileTimeError(
			"Function resolution error",
			fmt.Sprintf("Trait method \"%s\" is declared by implemented trait \"%s\" on type \"%s\", but no lowered implementation was found", methodName, declaredButMissing[0], classScope.Scope),
			pos,
		)
		return nil
	}

	scope.ErrorScope.NewCompileTimeError(
		"Function resolution error",
		fmt.Sprintf("Unable to resolve the method \"%s\" on type \"%s\"", methodName, classScope.Scope),
		pos,
	)
	return nil
}

func (impl *LLVMBackendImplementation) methodSymbolCandidates(classScope *ast.Ast, resolution *llvmResolvedMethod, methodName string) []string {
	candidates := make([]string, 0, 8)
	if resolution != nil && resolution.Method != nil {
		candidates = appendUniqueNonEmpty(candidates, resolution.Method.Name)
		candidates = appendUniqueNonEmpty(candidates, resolution.Method.GetFullName())
	}

	if classScope != nil {
		className := classScope.Scope
		classFullName := classScope.GetFullName()
		originModule := classScope.GetOriginModule()

		candidates = appendUniqueNonEmpty(candidates, classFullName+"__"+methodName)
		candidates = appendUniqueNonEmpty(candidates, className+"__"+methodName)
		if originModule != "" {
			candidates = appendUniqueNonEmpty(candidates, originModule+"__"+className+"__"+methodName)
		}

		if resolution != nil && resolution.TraitName != "" {
			traitName := resolution.TraitName
			traitBaseName := llvmTraitBaseName(traitName)

			candidates = appendUniqueNonEmpty(candidates, className+"__"+traitName+"__"+methodName)
			candidates = appendUniqueNonEmpty(candidates, classFullName+"__"+traitName+"__"+methodName)
			if originModule != "" {
				candidates = appendUniqueNonEmpty(candidates, originModule+"__"+className+"__"+traitName+"__"+methodName)
			}

			if traitBaseName != traitName {
				candidates = appendUniqueNonEmpty(candidates, className+"__"+traitBaseName+"__"+methodName)
				candidates = appendUniqueNonEmpty(candidates, classFullName+"__"+traitBaseName+"__"+methodName)
				if originModule != "" {
					candidates = appendUniqueNonEmpty(candidates, originModule+"__"+className+"__"+traitBaseName+"__"+methodName)
				}
			}
		}
	}

	candidates = appendUniqueNonEmpty(candidates, methodName)
	return candidates
}

func (impl *LLVMBackendImplementation) traitMethodLowered(scope *ast.Ast, resolution *llvmResolvedMethod, symbolCandidates []string) bool {
	if resolution == nil || resolution.Method == nil || resolution.TraitName == "" {
		return true
	}

	if methodInfo, ok := (*LLVMScopeDataMap)[resolution.Method.GetFullName()]; ok &&
		methodInfo != nil &&
		methodInfo.LocalContext != nil &&
		methodInfo.LocalContext.Func != nil {
		return true
	}

	info := LLVMGetScopeInformation(scope)
	return findIRFuncByName(info.ProgramContext.Module, symbolCandidates...) != nil
}

func (impl *LLVMBackendImplementation) lowerMethodCall(scope *ast.Ast, state llvmChainState, chain *tokens.ChainAccess) value.Value {
	classScope := impl.resolveClassFromTypeRefForMethod(scope, state.GeckoType, chain.Name, chain.Pos)
	if classScope == nil {
		return nil
	}

	resolution := impl.resolveMethodOnClass(scope, classScope, chain.Name, chain.Pos)
	if resolution == nil || resolution.Method == nil {
		return nil
	}

	if visErr := resolution.Method.CheckVisibility(scope); visErr != "" {
		scope.ErrorScope.NewCompileTimeError("Visibility Error", visErr, chain.Pos)
	}

	info := LLVMGetScopeInformation(scope)
	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Call Error", "method call must be inside a function body", chain.Pos)
		return nil
	}

	symbolCandidates := impl.methodSymbolCandidates(classScope, resolution, chain.Name)
	if !impl.traitMethodLowered(scope, resolution, symbolCandidates) {
		scope.ErrorScope.NewCompileTimeError(
			"Function resolution error",
			fmt.Sprintf("Trait method \"%s\" on type \"%s\" has no lowered implementation in LLVM (trait \"%s\")", chain.Name, classScope.Scope, resolution.TraitName),
			chain.Pos,
		)
		return nil
	}

	fn := impl.resolveMethodIRFunction(scope, resolution.Method, symbolCandidates...)
	selfArg := impl.ensureReceiverPointer(scope, state.Value)
	if selfArg == nil {
		scope.ErrorScope.NewCompileTimeError("Call Error", "unable to materialize receiver for method call", chain.Pos)
		return nil
	}

	args := make([]value.Value, 0, len(chain.GetArgs())+1)
	args = append(args, selfArg)
	for _, arg := range chain.GetArgs() {
		argValue := impl.ExpressionToLLIRValue(arg.Value, scope, &tokens.TypeRef{})
		if argValue == nil {
			scope.ErrorScope.NewCompileTimeError("Call Error", "unable to evaluate method call argument", chain.Pos)
			return nil
		}
		args = append(args, argValue)
	}

	return info.LocalContext.MainBlock.NewCall(fn, args...)
}

func (impl *LLVMBackendImplementation) lowerValueChain(scope *ast.Ast, state llvmChainState, chain []*tokens.ChainAccess, pos lexer.Position) value.Value {
	if len(chain) == 0 {
		return state.Value
	}

	for idx, link := range chain {
		isTerminal := idx == len(chain)-1

		if link.IsMethodCall() {
			if !isTerminal {
				scope.ErrorScope.NewCompileTimeError(
					"Not Implemented",
					"LLVM backend currently supports terminal method invocation in chains only",
					pos,
				)
				return nil
			}
			return impl.lowerMethodCall(scope, state, link)
		}

		nextState, ok := impl.lowerFieldAccess(scope, state, link, isTerminal)
		if !ok {
			return nil
		}
		state = nextState
	}

	return state.Value
}

func (impl *LLVMBackendImplementation) readSymbolValue(scope *ast.Ast, variable *ast.Variable, info *LLVMValueInformation, wantsAddress bool) value.Value {
	if info == nil {
		return nil
	}
	if info.Value == nil {
		return nil
	}
	if wantsAddress {
		return info.Value
	}
	if variable != nil && variable.IsArgument {
		return info.Value
	}

	ptrType, isPointer := info.Value.Type().(*types.PointerType)
	if !isPointer || ptrType.ElemType == nil {
		return info.Value
	}

	scopeInfo := LLVMGetScopeInformation(scope)
	if scopeInfo.LocalContext == nil || scopeInfo.LocalContext.MainBlock == nil {
		return info.Value
	}

	return impl.NewVolatileLoad(scopeInfo.LocalContext.MainBlock, ptrType.ElemType, info.Value, info.IsVolatile)
}

func (impl *LLVMBackendImplementation) ResolveSymbolChainValue(scope *ast.Ast, symbolName string, chain []*tokens.ChainAccess, pos lexer.Position, wantsAddress bool) value.Value {
	symbolVariable := scope.ResolveSymbolAsVariable(symbolName)
	if !symbolVariable.IsNil() {
		variable := symbolVariable.Unwrap()
		varInfo := LLVMGetValueInformation(variable)
		if len(chain) == 0 {
			return impl.readSymbolValue(scope, variable, varInfo, wantsAddress)
		}

		state := llvmChainState{
			Value:     varInfo.Value,
			GeckoType: cloneTypeRef(varInfo.GeckoType),
		}
		if state.GeckoType == nil && variable.IsPointer {
			state.GeckoType = &tokens.TypeRef{Pointer: true}
		}
		return impl.lowerValueChain(scope, state, chain, pos)
	}

	if enumValue, handled := impl.resolveEnumCaseConstant(scope, symbolName, chain, pos); handled {
		return enumValue
	}

	if len(chain) > 0 {
		rootScope := scope.GetRoot()
		if importedModule, ok := rootScope.Children[symbolName]; ok {
			return impl.resolveModuleChain(scope, importedModule, symbolName, chain, pos)
		}
	}

	scope.ErrorScope.NewCompileTimeError("Variable Resolution Error", "Unable to resolve the variable '"+symbolName+"'", pos)
	return nil
}
