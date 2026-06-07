// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"fmt"
	"strconv"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

var CurrentBackend interfaces.BackendInteface = nil
var FuncCalls map[string]*ir.InstCall
var Methods map[string]*ast.Method
var LLVMExecutionContext *ExecutionContext = nil

func llvmFuncCallCacheKey(scope *ast.Ast, f *tokens.FuncCall) string {
	if scope == nil || f == nil {
		return ""
	}

	name := f.Function
	if f.Module != "" {
		name = f.Module + "." + f.Function
	}
	if f.StaticType != "" {
		if f.StaticModule != "" {
			name = f.StaticModule + "." + f.StaticType + "::" + f.Function
		} else {
			name = f.StaticType + "::" + f.Function
		}
	}

	return scope.FullScopeName() + "#" + name
}

func (info *LLVMScopeInformation) Init(a *ast.Ast) {
	var executionContext = LLVMExecutionContext
	if LLVMExecutionContext == nil {
		executionContext = NewExecutionContext()
		LLVMExecutionContext = executionContext
	}

	info.ExecutionContext = executionContext
	info.ProgramContext = executionContext.Context
	info.LocalContext = nil
	info.ChildContexts = make(map[string]*LocalContext)

	loadPrimitives(a, info.LocalContext)
}

func (impl *LLVMBackendImplementation) NewReturn(scope *ast.Ast) {
	info := LLVMGetScopeInformation(scope)

	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Return Error", "return statement must be inside a function body", lexer.Position{})
		return
	}

	info.LocalContext.MainBlock.NewRet(nil)
}

func (impl *LLVMBackendImplementation) NewReturnLiteral(scope *ast.Ast, literal *tokens.Expression) {
	info := LLVMGetScopeInformation(scope)
	if literal == nil {
		scope.ErrorScope.NewCompileTimeError("Return Error", "return statement is missing an expression", lexer.Position{})
		return
	}
	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Return Error", "return statement must be inside a function body", literal.Pos)
		return
	}

	if info.LocalContext.Func == nil || info.LocalContext.Func.Sig == nil || info.LocalContext.Func.Sig.RetType == nil {
		scope.ErrorScope.NewCompileTimeError("Return Error", "unable to resolve function return type", literal.Pos)
		return
	}
	if _, isVoid := info.LocalContext.Func.Sig.RetType.(*types.VoidType); isVoid {
		scope.ErrorScope.NewCompileTimeError("Return Error", "cannot return a value from a void function", literal.Pos)
		return
	}

	expectedType := llirTypeToGeckoTypeRef(info.LocalContext.Func.Sig.RetType)
	val := impl.ExpressionToLLIRValue(literal, scope, expectedType)
	if val == nil {
		scope.ErrorScope.NewCompileTimeError("Return Error", "unable to evaluate return expression", literal.Pos)
		return
	}

	val = impl.coerceValueToType(val, info.LocalContext.Func.Sig.RetType, scope, literal.Pos)
	if val == nil {
		return
	}

	info.LocalContext.MainBlock.NewRet(val)
}

func llirTypeToGeckoTypeRef(t types.Type) *tokens.TypeRef {
	switch tt := t.(type) {
	case *types.IntType:
		switch tt.BitSize {
		case 1:
			return &tokens.TypeRef{Type: "bool"}
		case 8:
			return &tokens.TypeRef{Type: "int8"}
		case 16:
			return &tokens.TypeRef{Type: "int16"}
		case 32:
			return &tokens.TypeRef{Type: "int32"}
		case 64:
			return &tokens.TypeRef{Type: "int64"}
		default:
			return &tokens.TypeRef{Type: "int64"}
		}
	case *types.PointerType:
		base := llirTypeToGeckoTypeRef(tt.ElemType)
		if base == nil {
			base = &tokens.TypeRef{Type: "int8"}
		}
		cloned := *base
		cloned.Pointer = true
		return &cloned
	case *types.VoidType:
		return &tokens.TypeRef{Type: "void"}
	default:
		return &tokens.TypeRef{Type: "int64"}
	}
}

func (impl *LLVMBackendImplementation) coerceValueToType(val value.Value, target types.Type, scope *ast.Ast, pos lexer.Position) value.Value {
	if val == nil || target == nil {
		return nil
	}

	if val.Type().Equal(target) {
		return val
	}

	info := LLVMGetScopeInformation(scope)
	if info.LocalContext == nil || info.LocalContext.MainBlock == nil {
		return val
	}

	switch dst := target.(type) {
	case *types.IntType:
		switch src := val.Type().(type) {
		case *types.IntType:
			if src.BitSize > dst.BitSize {
				return info.LocalContext.MainBlock.NewTrunc(val, dst)
			}
			if src.BitSize < dst.BitSize {
				return info.LocalContext.MainBlock.NewSExt(val, dst)
			}
			return val
		case *types.PointerType:
			return info.LocalContext.MainBlock.NewPtrToInt(val, dst)
		}
	case *types.PointerType:
		switch val.Type().(type) {
		case *types.IntType:
			return info.LocalContext.MainBlock.NewIntToPtr(val, dst)
		case *types.PointerType:
			return info.LocalContext.MainBlock.NewBitCast(val, dst)
		}
	}

	scope.ErrorScope.NewCompileTimeError(
		"Return Error",
		fmt.Sprintf("cannot return value of type '%s' from function returning '%s'", val.Type().String(), target.String()),
		pos,
	)
	return nil
}

func (*LLVMBackendImplementation) Declaration(scope *ast.Ast, decl *tokens.Declaration) {

}

func (*LLVMBackendImplementation) ParseExpression(scope *ast.Ast, exp *tokens.Expression) {

}

func (impls *LLVMBackendImplementation) ProcessEntries(scope *ast.Ast, entries []*tokens.Entry) {
	impls.Backend.ProcessEntries(entries, scope)
}

func (impls *LLVMBackendImplementation) NewDeclaration(scope *ast.Ast, decl *tokens.Declaration) {
	if decl.Field != nil {
		impls.NewVariable(scope, decl.Field)
	} else if decl.Method != nil {
		impls.NewMethod(scope, decl.Method)
	}
}

func (impls *LLVMBackendImplementation) NewClass(scope *ast.Ast, c *tokens.Class) {
	classAst := &ast.Ast{
		Scope:        c.Name,
		Parent:       scope,
		Visibility:   c.Visibility,
		OriginModule: scope.GetRoot().Scope,
		SourceFile:   scope.GetSourceFile(),
	}

	classAst.Init(scope.ErrorScope)
	scope.Classes[c.Name] = classAst

	// Check for @packed attribute and store in AST
	isPacked := tokens.IsPacked(c.Attributes)
	classAst.IsPacked = isPacked

	// Collect field types and names for struct definition
	fieldTypes := make([]types.Type, 0)
	fieldNames := make([]string, 0)
	geckoFieldTypes := make([]*tokens.TypeRef, 0)
	classMethods := make([]*tokens.Method, 0)

	for _, f := range c.Fields {
		if f.Field != nil {
			// Note: Don't validate field types here - circular dependency detection
			// needs forward references to work. Field types are validated at usage time.
			fieldType := impls.TypeRefGetLLIRType(f.Field.Type, scope)
			if fieldType != nil {
				fieldTypes = append(fieldTypes, fieldType)
				fieldNames = append(fieldNames, f.Field.Name)
				geckoFieldTypes = append(geckoFieldTypes, f.Field.Type)
			}

			// Register field as class member
			impls.NewClassField(classAst, f.Field)
		}
		if f.Method != nil {
			classMethods = append(classMethods, f.Method)
		}
	}

	// Create LLVM struct type
	info := LLVMGetScopeInformation(scope)
	structType := types.NewStruct(fieldTypes...)
	structType.SetName(c.Name)
	structType.Packed = isPacked

	// Register the struct type in the module
	info.ProgramContext.Module.TypeDefs = append(info.ProgramContext.Module.TypeDefs, structType)

	// Store struct info in the global map for later use in struct literals
	LLVMStructMap[c.Name] = &LLVMStructInfo{
		Type:       structType,
		FieldNames: fieldNames,
		FieldTypes: geckoFieldTypes,
		IsPacked:   isPacked,
	}

	// Store type information for the class
	if info.LocalContext != nil {
		var t types.Type = structType
		info.LocalContext.Types[c.Name] = &t
	}

	// Lower class methods after struct registration so `self` type resolution can
	// see the class in LLVMStructMap.
	for _, m := range classMethods {
		impls.NewMethod(classAst, m)
	}
}

// NewClassField registers a field in the class AST without generating local variable code
func (impls *LLVMBackendImplementation) NewClassField(scope *ast.Ast, f *tokens.Field) {
	if f.Type == nil {
		f.Type = impls.inferFieldType(scope, f)
	}
	if f.Type == nil {
		f.Type = &tokens.TypeRef{Type: "int32"}
	}

	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    f.Type.Const,
		IsVolatile: f.Type.Volatile,
		IsPointer:  f.Type.Pointer,
		IsExternal: f.Visibility == "external",
		Parent:     scope,
	}

	(*LLVMProgramValues)[fieldVariable.GetFullName()] = &LLVMValueInformation{
		Type:       impls.TypeRefGetLLIRType(f.Type, scope),
		Value:      nil,
		GeckoType:  f.Type,
		IsVolatile: f.Type.Volatile,
	}

	scope.Variables[f.Name] = fieldVariable
}

func (impls *LLVMBackendImplementation) MethodCall(scope *ast.Ast, m *tokens.MethodCall) {
	if m == nil || len(m.Chain) == 0 {
		return
	}

	if !m.IsValid() {
		scope.ErrorScope.NewCompileTimeError("Call Error", "invalid method call chain: terminal chain element must be a method call", m.Pos)
		return
	}

	impls.ResolveSymbolChainValue(scope, m.Base, m.Chain, m.Pos, false)
}

func (impls *LLVMBackendImplementation) FuncCall(scope *ast.Ast, f *tokens.FuncCall) {
	if f == nil {
		return
	}

	// Handle variable method call expressions parsed as FuncCall:
	// `x.method(...)` where `x` is in `Module`.
	if f.StaticType == "" && f.Module != "" {
		varOpt := scope.ResolveSymbolAsVariable(f.Module)
		if !varOpt.IsNil() {
			variable := varOpt.Unwrap()
			valueInfo := LLVMGetValueInformation(variable)
			if valueInfo == nil || valueInfo.Value == nil {
				scope.ErrorScope.NewCompileTimeError("Call Error", "unable to resolve receiver for method call '"+f.Module+"."+f.Function+"'", f.Pos)
				return
			}

			chain := &tokens.ChainAccess{
				Name:      f.Function,
				HasParens: true,
				Args:      f.Arguments,
			}
			chain.Pos = f.Pos

			state := llvmChainState{
				Value:     valueInfo.Value,
				GeckoType: cloneTypeRef(valueInfo.GeckoType),
			}
			if state.GeckoType == nil && variable.IsPointer {
				state.GeckoType = &tokens.TypeRef{Pointer: true}
			}

			callValue := impls.lowerMethodCall(scope, state, chain)
			if callValue == nil {
				return
			}

			callInst, ok := callValue.(*ir.InstCall)
			if !ok || callInst == nil {
				scope.ErrorScope.NewCompileTimeError("Call Error", "internal LLVM method-call lowering did not produce a call instruction", f.Pos)
				return
			}

			FuncCalls[llvmFuncCallCacheKey(scope, f)] = callInst
			return
		}
	}

	lookupScope := scope
	if f.Module != "" {
		root := scope.GetRoot()
		if importedModule, ok := root.Children[f.Module]; ok {
			lookupScope = importedModule
		}
	}

	mth := lookupScope.ResolveMethod(f.Function)

	if mth.IsNil() {
		symbol := f.Function
		if f.Module != "" {
			symbol = f.Module + "." + f.Function
		}
		scope.ErrorScope.NewCompileTimeError("Function resolution error", fmt.Sprintf("Unable to resolve the function \"%s\"", symbol), f.Pos)
		return
	}

	mthUnwrapped := mth.Unwrap()
	callerInfo := LLVMGetScopeInformation(scope)
	if callerInfo.LocalContext == nil || callerInfo.LocalContext.MainBlock == nil {
		scope.ErrorScope.NewCompileTimeError("Call Error", "function call must be inside a function body", f.Pos)
		return
	}

	var fn *ir.Func

	if mthUnwrapped.Scope != nil {
		calleeInfo := LLVMGetScopeInformation(mthUnwrapped.Scope)
		if calleeInfo.LocalContext != nil && calleeInfo.LocalContext.Func != nil {
			fn = calleeInfo.LocalContext.Func
		}
	}

	if fn == nil {
		retType := impls.TypeRefGetLLIRType(&tokens.TypeRef{Type: mthUnwrapped.Type}, scope)
		if retType == nil {
			retType = VoidType.Type
		}
		symbolName := mthUnwrapped.GetFullName()
		if symbolName == "" {
			symbolName = f.Function
			if f.Module != "" {
				symbolName = f.Module + "__" + f.Function
			}
		}

		if callerInfo.ProgramContext != nil && callerInfo.ProgramContext.Module != nil {
			if declared := findIRFuncByName(callerInfo.ProgramContext.Module, symbolName, f.Function); declared != nil {
				fn = declared
			}
		}

		if fn == nil {
			fn = ir.NewFunc(symbolName, retType)
			fn.Linkage = enum.LinkageExternal
		}
	}

	fn.CallingConv = CallingConventions[scope.Config.Arch][scope.Config.Platform]

	args := make([]value.Value, 0)

	for idx, a := range f.Arguments {
		tr := &tokens.TypeRef{}

		if a.Out {
			scope.ErrorScope.NewCompileTimeError(
				"Unsupported Out Argument",
				"out arguments are not supported at LLVM call sites; out parameters are only allowed on declared external functions where they are represented as pointers",
				f.Pos,
			)
			return
		}

		if idx < len(mthUnwrapped.Arguments) {
			argVar := mthUnwrapped.Arguments[idx]
			argInfo := LLVMGetValueInformation(&argVar)
			if argInfo.GeckoType != nil {
				tr = argInfo.GeckoType
			} else if argInfo.Type != nil {
				if inferred := llirTypeToGeckoTypeRef(argInfo.Type); inferred != nil {
					tr = inferred
				}
			} else if argVar.IsPointer {
				tr = &tokens.TypeRef{Type: "int8", Pointer: true}
			}
		}

		argValue := impls.ExpressionToLLIRValue(a.Value, scope, tr)
		if argValue == nil {
			scope.ErrorScope.NewCompileTimeError("Call Error", "unable to evaluate function call argument", f.Pos)
			return
		}

		args = append(args, argValue)
	}

	call := callerInfo.LocalContext.MainBlock.NewCall(fn, args...)

	FuncCalls[llvmFuncCallCacheKey(scope, f)] = call
}

func (impl *LLVMBackendImplementation) NewImplementation(scope *ast.Ast, i *tokens.Implementation) {
	if i.GetFor() != "" {
		impl.LLVMImplementationForClass(scope, i)
	} else {
		classOpt := scope.ResolveClass(i.GetName())
		if !classOpt.IsNil() {
			class := classOpt.Unwrap()
			impl.LLVMInherentImplementation(scope, i, class)
			return
		}
		impl.LLVMImplementationForArch(scope, i)
	}
}

func (impl *LLVMBackendImplementation) NewTrait(scope *ast.Ast, t *tokens.Trait) {
	TraitDefinitionOrigins[t.Name] = scope.GetRoot().Scope
	impl.TraitAssignToScope(scope, t)
}

func (impl *LLVMBackendImplementation) NewEnum(scope *ast.Ast, e *tokens.Enum) {
	if e == nil {
		return
	}

	enumAst := &ast.Ast{
		Scope:        e.Name,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
		SourceFile:   scope.GetSourceFile(),
	}
	enumAst.Init(scope.ErrorScope)

	enumInfo := &LLVMEnumInfo{
		LLVMType: types.I32,
		Cases:    make(map[string]int64),
	}

	for idx, caseName := range e.Cases {
		enumAst.Variables[caseName] = ast.Variable{
			Name:    caseName,
			IsConst: true,
			Parent:  enumAst,
		}

		caseVar := enumAst.Variables[caseName]
		caseValue := int64(idx)
		enumInfo.Cases[caseName] = caseValue

		(*LLVMProgramValues)[caseVar.GetFullName()] = &LLVMValueInformation{
			Type:       enumInfo.LLVMType,
			Value:      constant.NewInt(enumInfo.LLVMType, caseValue),
			GeckoType:  &tokens.TypeRef{Type: e.Name},
			IsVolatile: false,
		}
	}

	scope.Classes[e.Name] = enumAst
	LLVMEnumMap[enumAst.FullScopeName()] = enumInfo
}

func (impl *LLVMBackendImplementation) NewMethod(scope *ast.Ast, m *tokens.Method) {
	methodScope := ast.Ast{
		Scope:        m.Name,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
		SourceFile:   scope.GetSourceFile(),
	}

	info := LLVMGetScopeInformation(scope)

	if m.Visibility != "external" {
		for _, a := range m.Arguments {
			if a.Out {
				scope.ErrorScope.NewCompileTimeError(
					"Unsupported Out Parameter",
					"out parameters are only allowed on declared external functions in LLVM (represented as pointer types); they are unsupported for non-external functions and call-site out arguments",
					m.Pos,
				)
			}
		}
	}

	fnParams := make([]*ir.Param, 0)
	resolvedArgTypes := make([]*tokens.TypeRef, len(m.Arguments))

	for idx, a := range m.Arguments {
		resolvedType := a.Type
		if resolvedType == nil && a.Name == "self" {
			if !scope.ResolveClass(scope.Scope).IsNil() {
				resolvedType = &tokens.TypeRef{Type: scope.Scope, Pointer: true}
			}
		}
		if resolvedType == nil {
			scope.ErrorScope.NewCompileTimeError(
				"Type Inference Error",
				"unable to infer parameter type for '"+a.Name+"'; please add an explicit type annotation",
				m.Pos,
			)
			// Keep lowering in a non-panicking state to surface all diagnostics.
			resolvedType = &tokens.TypeRef{Type: "int8", Pointer: true}
		}
		resolvedArgTypes[idx] = resolvedType
		m.Arguments[idx].Type = resolvedType

		// Validate parameter type
		resolvedType.Check(scope)
		ty := impl.TypeRefGetLLIRType(resolvedType, scope)
		if ty == nil {
			scope.ErrorScope.NewCompileTimeError(
				"Type Check Error",
				"unable to lower LLVM type for parameter '"+a.Name+"'",
				m.Pos,
			)
			ty = types.I8Ptr
		}
		if m.Visibility == "external" && a.Out {
			ty = types.NewPointer(ty)
		}

		fnParams = append(fnParams, ir.NewParam(a.Name, ty))
	}

	methodScope.Init(scope.ErrorScope)

	returnType := "void"
	irType := VoidType.Type

	if m.Type != nil {
		m.Type.Check(scope)

		returnType = m.Type.ToCString(scope)
		irType = impl.TypeRefGetLLIRType(m.Type, scope)
	}

	irFunc := ir.NewFunc(m.Name, irType, fnParams...)
	irFunc.CallingConv = CallingConventions[scope.Config.Arch][scope.Config.Platform]
	if m.Variardic {
		irFunc.Sig.Variadic = true
	}

	// Apply function attributes from @naked, @noreturn, @section, etc.
	for _, attr := range m.Attributes {
		switch attr.Name {
		case "naked":
			irFunc.FuncAttrs = append(irFunc.FuncAttrs, enum.FuncAttrNaked)
		case "noreturn":
			irFunc.FuncAttrs = append(irFunc.FuncAttrs, enum.FuncAttrNoReturn)
		case "section":
			irFunc.Section = attr.GetStringValue()
		case "used":
			irFunc.Linkage = enum.LinkageExternal
		}
	}

	mthInfo := &LLVMScopeInformation{}
	mthInfo.Init(&methodScope)

	methodScope.Config = scope.Config
	mthInfo.LocalContext = NewLocalContext(irFunc)

	loadPrimitives(&methodScope, info.LocalContext)

	if len(m.Value) > 0 {
		mthInfo.LocalContext.MainBlock = mthInfo.LocalContext.Func.NewBlock(irFunc.Name() + "$main")
	}

	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      map[bool]*ast.Ast{true: nil, false: &methodScope}[len(m.Value) == 0],
		Arguments:  make([]ast.Variable, 0),
		Visibility: m.Visibility,
		Parent:     scope,
		Type:       returnType,
	}
	scope.Methods[m.Name] = astMth

	methodScopeKey := methodScope.GetFullName()
	(*LLVMScopeDataMap)[methodScopeKey] = mthInfo
	(*LLVMScopeDataMap)[astMth.GetFullName()] = mthInfo

	info.ChildContexts[astMth.GetFullName()] = mthInfo.LocalContext
	info.ChildContexts[methodScopeKey] = mthInfo.LocalContext
	info.ProgramContext.Module.Funcs = append(info.ProgramContext.Module.Funcs, mthInfo.LocalContext.Func)

	// Add arguments as variables

	for idx, v := range m.Arguments {
		argType := resolvedArgTypes[idx]
		if argType == nil {
			argType = &tokens.TypeRef{Type: "int8", Pointer: true}
		}
		mVariable := ast.Variable{
			IsPointer:  argType.Pointer || (m.Visibility == "external" && v.Out),
			IsConst:    argType.Const,
			IsVolatile: argType.Volatile,
			IsExternal: false,
			IsArgument: true,
			Name:       v.Name,
			Parent:     &methodScope,
		}

		methodScope.Variables[v.Name] = mVariable

		vIrType := impl.TypeRefGetLLIRType(argType, scope)
		if vIrType == nil {
			vIrType = types.I8Ptr
		}

		(*LLVMProgramValues)[mVariable.GetFullName()] = &LLVMValueInformation{
			Type:       vIrType,
			Value:      ir.NewParam(v.Name, vIrType),
			GeckoType:  argType,
			IsVolatile: argType.Volatile,
		}
	}

	impl.Backend.ProcessEntries(m.Value, &methodScope)

	impl.LLVMAssignArgumentsToMethodArguments(m.Arguments, astMth)

	// If no return is specified, inject a void return directly to the current block
	if mthInfo.LocalContext.MainBlock != nil && mthInfo.LocalContext.MainBlock.Term == nil {
		mthInfo.LocalContext.MainBlock.NewRet(nil)
	}

	// if len(m.Value) > 0 {
	// 	methodScope.LocalContext.MainBlock.NewRet(constant.NewInt(types.I1, 0))
	// }
	Methods[scope.FullScopeName()+"#"+m.Name] = astMth
}

func (impl *LLVMBackendImplementation) NewVariable(scope *ast.Ast, f *tokens.Field) {
	typeWasExplicit := f.Type != nil
	if f.Type == nil {
		f.Type = impl.inferFieldType(scope, f)
	}
	if f.Type == nil {
		f.Type = &tokens.TypeRef{Type: "int32"}
	}
	if typeWasExplicit {
		f.Type.Check(scope)
	}

	// Check for const - either from Type.Const or from Mutability == "const"
	isConst := (f.Type != nil && f.Type.Const) || f.Mutability == "const"

	if f.Value == nil && isConst {
		scope.ErrorScope.NewCompileTimeError("Uninitialized constant", "constant must be initialized with a value", f.Pos)
		f.Value = &tokens.Expression{}
	}

	info := LLVMGetScopeInformation(scope)

	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    isConst,
		IsVolatile: f.Type.Volatile,
		IsPointer:  f.Type.Pointer,
		IsExternal: false,
		Parent:     scope,
	}

	if f.Visibility == "external" {
		fieldVariable.IsExternal = true
	}

	// Check if this is a global variable (no local context / not in a function)
	isGlobal := info.LocalContext == nil || info.LocalContext.MainBlock == nil

	// Check for @section attribute - if present, always treat as global
	sectionName := tokens.GetSection(f.Attributes)
	if sectionName != "" {
		isGlobal = true
	}

	if isGlobal {
		impl.NewGlobalVariable(scope, f, &fieldVariable, sectionName)
	} else {
		impl.NewLocalVariable(scope, f, &fieldVariable)
	}

	scope.Variables[f.Name] = fieldVariable
}

func (impl *LLVMBackendImplementation) inferFieldType(scope *ast.Ast, f *tokens.Field) *tokens.TypeRef {
	if f == nil {
		return nil
	}
	if f.Type != nil {
		return f.Type
	}
	if f.Value == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Type Inference Error",
			"Unable to infer variable type; please provide an explicit type annotation",
			f.Pos,
		)
		return nil
	}

	resolveSymbol := func(name string) *tokens.TypeRef {
		opt := scope.ResolveSymbolAsVariable(name)
		if opt.IsNil() {
			return nil
		}
		v := opt.Unwrap()
		info := LLVMGetValueInformation(v)
		return info.GeckoType
	}

	inferred := tokens.InferType(f.Value, resolveSymbol)
	if inferred == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Type Inference Error",
			"Unable to infer variable type; please provide an explicit type annotation",
			f.Pos,
		)
	}
	return inferred
}

// NewGlobalVariable creates a global LLVM variable with optional section attribute
func (impl *LLVMBackendImplementation) NewGlobalVariable(scope *ast.Ast, f *tokens.Field, fieldVariable *ast.Variable, sectionName string) {
	info := LLVMGetScopeInformation(scope)
	varType := impl.TypeRefGetLLIRType(f.Type, scope)

	// Handle sized arrays
	if f.Type.Size != nil {
		elemType := impl.TypeRefGetLLIRType(f.Type.Size.Type, scope)
		size, err := parseSize(f.Type.Size.Size)
		if err != nil {
			scope.ErrorScope.NewCompileTimeError("Invalid array size", err.Error(), f.Pos)
			return
		}
		varType = types.NewArray(size, elemType)
	}

	// Get the initializer constant
	var initVal constant.Constant
	if f.Value != nil {
		val := impl.ExpressionToLLIRConstant(f.Value, scope, f.Type)
		if val != nil {
			initVal = val
		} else {
			// Fallback to zero initializer
			initVal = constant.NewZeroInitializer(varType)
		}
	} else {
		// No initializer - use zero initializer
		initVal = constant.NewZeroInitializer(varType)
	}

	// Create global variable
	globalVar := info.ProgramContext.Module.NewGlobalDef(f.Name, initVal)

	// Set section if specified
	if sectionName != "" {
		globalVar.Section = sectionName
	}

	// If const, mark as immutable - either from Type.Const, from Mutability == "const",
	// or from the inner type's Const for sized arrays
	isConst := (f.Type != nil && f.Type.Const) || f.Mutability == "const"
	// For sized arrays, also check the inner type's const flag
	if f.Type != nil && f.Type.Size != nil && f.Type.Size.Type != nil && f.Type.Size.Type.Const {
		isConst = true
	}
	if isConst {
		globalVar.Immutable = true
	}

	// If @used attribute is present, mark with external linkage to prevent removal
	if tokens.HasAttribute(f.Attributes, "used") {
		globalVar.Linkage = enum.LinkageExternal
	}

	(*LLVMProgramValues)[fieldVariable.GetFullName()] = &LLVMValueInformation{
		Type:       varType,
		Value:      globalVar,
		GeckoType:  f.Type,
		IsVolatile: f.Type.Volatile,
	}
}

// NewLocalVariable creates a local LLVM variable (stack allocation)
func (impl *LLVMBackendImplementation) NewLocalVariable(scope *ast.Ast, f *tokens.Field, fieldVariable *ast.Variable) {
	info := LLVMGetScopeInformation(scope)
	varType := impl.TypeRefGetLLIRType(f.Type, scope)

	// Handle sized arrays for local variables
	if f.Type.Size != nil {
		elemType := impl.TypeRefGetLLIRType(f.Type.Size.Type, scope)
		size, err := parseSize(f.Type.Size.Size)
		if err != nil {
			scope.ErrorScope.NewCompileTimeError("Invalid array size", err.Error(), f.Pos)
			return
		}
		varType = types.NewArray(size, elemType)
	}

	if varType == nil {
		scope.ErrorScope.NewCompileTimeError("Type Error", "unable to resolve local variable type '"+f.Type.ToCString(scope)+"' for LLVM backend", f.Pos)
		return
	}

	var val value.Value
	if f.Value != nil {
		val = impl.ExpressionToLLIRValue(f.Value, scope, f.Type)
	}

	// Allocate space on the stack for local variables (including fixed-size arrays)
	var storedValue value.Value = val
	if info.LocalContext != nil && info.LocalContext.MainBlock != nil {
		// Check if the value is already an alloca (e.g., from a struct literal)
		// In that case, just rename it and use it directly
		if valAlloca, isAlloca := val.(*ir.InstAlloca); isAlloca {
			// Reuse the existing alloca, just rename it
			valAlloca.LocalIdent.SetName(f.Name)
			storedValue = valAlloca
		} else {
			// Create a new alloca for this variable
			allocaInst := info.LocalContext.MainBlock.NewAlloca(varType)
			allocaInst.LocalIdent.SetName(f.Name)

			// If there's an initializer value and it's not a fixed-size array, store it
			// Note: For fixed-size arrays without explicit initializers, they are zero-initialized
			// by the alloca. For arrays with initializers, we would need memcpy or element-by-element copy.
			// For now, skip storing for arrays as the original code did.
			if val != nil && f.Type.Size == nil {
				// Only store for non-array types when the value is not a pointer to a global
				// This is a simplified check - proper handling would need to load from globals first
				if _, isGlobal := val.(*ir.Global); !isGlobal {
					store := impl.NewVolatileStore(info.LocalContext.MainBlock, val, allocaInst, f.Type.Volatile)
					if store == nil {
						scope.ErrorScope.NewCompileTimeError("Type Error", "cannot initialize local variable '"+f.Name+"' because initializer type is incompatible", f.Pos)
						return
					}
				}
			}
			storedValue = allocaInst
		}
	}

	(*LLVMProgramValues)[fieldVariable.GetFullName()] = &LLVMValueInformation{
		Type:       varType,
		Value:      storedValue,
		GeckoType:  f.Type,
		IsVolatile: f.Type.Volatile,
	}
}

// ExpressionToLLIRConstant converts an expression to an LLVM constant (for global initializers)
func (impl *LLVMBackendImplementation) ExpressionToLLIRConstant(e *tokens.Expression, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if e == nil || e.GetLogicalOr() == nil {
		return nil
	}

	// For simple cases, we can evaluate constants
	return impl.LogicalOrToLLIRConstant(e.GetLogicalOr(), scope, expectedType)
}

// LogicalOrToLLIRConstant converts logical OR expressions to constants
func (impl *LLVMBackendImplementation) LogicalOrToLLIRConstant(lo *tokens.LogicalOr, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if lo == nil {
		return nil
	}

	if lo.Next == nil {
		return impl.LogicalAndToLLIRConstant(lo.LogicalAnd, scope, expectedType)
	}

	return nil
}

// LogicalAndToLLIRConstant converts logical AND expressions to constants
func (impl *LLVMBackendImplementation) LogicalAndToLLIRConstant(la *tokens.LogicalAnd, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if la == nil {
		return nil
	}

	if la.Next == nil {
		return impl.EqualityToLLIRConstant(la.Equality, scope, expectedType)
	}

	return nil
}

// EqualityToLLIRConstant converts equality expressions to constants
func (impl *LLVMBackendImplementation) EqualityToLLIRConstant(eq *tokens.Equality, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if eq == nil {
		return nil
	}

	// For now, only handle simple cases without operators
	if eq.Next == nil {
		return impl.ComparisonToLLIRConstant(eq.Comparison, scope, expectedType)
	}

	// Complex expressions with operators are not supported for global initializers yet
	return nil
}

// ComparisonToLLIRConstant converts comparison expressions to constants
func (impl *LLVMBackendImplementation) ComparisonToLLIRConstant(c *tokens.Comparison, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if c == nil {
		return nil
	}

	if c.Next == nil {
		return impl.AdditionToLLIRConstant(c.Addition, scope, expectedType)
	}

	return nil
}

// AdditionToLLIRConstant converts addition expressions to constants
func (impl *LLVMBackendImplementation) AdditionToLLIRConstant(a *tokens.Addition, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if a == nil {
		return nil
	}

	if a.Next == nil {
		return impl.MultiplicationToLLIRConstant(a.Multiplication, scope, expectedType)
	}

	// For constant expressions with operators, we need constant folding
	// For now, return nil for complex expressions
	return nil
}

// MultiplicationToLLIRConstant converts multiplication expressions to constants
func (impl *LLVMBackendImplementation) MultiplicationToLLIRConstant(m *tokens.Multiplication, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if m == nil {
		return nil
	}

	if m.Next == nil {
		return impl.UnaryToLLIRConstant(m.Unary, scope, expectedType)
	}

	return nil
}

// UnaryToLLIRConstant converts unary expressions to constants
func (impl *LLVMBackendImplementation) UnaryToLLIRConstant(u *tokens.Unary, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if u == nil {
		return nil
	}

	if u.Primary != nil {
		return impl.PrimaryToLLIRConstant(u.Primary, scope, expectedType)
	}

	return nil
}

// PrimaryToLLIRConstant converts primary expressions to constants
func (impl *LLVMBackendImplementation) PrimaryToLLIRConstant(p *tokens.Primary, scope *ast.Ast, expectedType *tokens.TypeRef) constant.Constant {
	if p == nil || p.Literal == nil {
		return nil
	}

	l := p.Literal

	if l.Number != "" {
		// Parse the number and create appropriate constant based on expected type
		intVal, err := parseNumber(l.Number)
		if err != nil {
			scope.ErrorScope.NewCompileTimeError("Invalid number", err.Error(), l.Pos)
			return nil
		}

		// Determine the integer type based on expectedType
		intType := impl.TypeRefGetLLIRType(expectedType, scope)
		if intType == nil {
			intType = types.I64 // default to i64
		}

		if iType, ok := intType.(*types.IntType); ok {
			if iType.BitSize == 1 && intVal != 0 && intVal != 1 {
				return constant.NewInt(types.I64, intVal)
			}
			return constant.NewInt(iType, intVal)
		}

		return constant.NewInt(types.I64, intVal)
	}

	if l.Bool != "" {
		i := map[string]int64{"true": 1, "false": 0}[l.Bool]
		return constant.NewInt(types.I1, i)
	}

	// For sub-expressions in parentheses
	if p.SubExpression != nil {
		return impl.ExpressionToLLIRConstant(p.SubExpression, scope, expectedType)
	}

	return nil
}

// parseSize parses a size string to uint64
func parseSize(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// parseNumber parses a number string (including hex) to int64
func parseNumber(s string) (int64, error) {
	// Handle hex numbers
	if len(s) > 2 && (s[0:2] == "0x" || s[0:2] == "0X") {
		return strconv.ParseInt(s[2:], 16, 64)
	}
	// Handle decimal
	return strconv.ParseInt(s, 10, 64)
}

func LLVMGetScopeInformation(scope *ast.Ast) *LLVMScopeInformation {
	name := scope.GetFullName()

	info, ok := (*LLVMScopeDataMap)[name]

	if !ok {
		info := &LLVMScopeInformation{}
		info.Init(scope)
		(*LLVMScopeDataMap)[name] = info
		return (*LLVMScopeDataMap)[name]
	}

	return info
}

func LLVMGetValueInformation(variable *ast.Variable) *LLVMValueInformation {
	name := variable.GetFullName()

	info, ok := (*LLVMProgramValues)[name]

	if !ok {
		(*LLVMProgramValues)[name] = &LLVMValueInformation{}
		return (*LLVMProgramValues)[name]
	}

	return info
}

var LLVMScopeDataMap = &LLVMScopeData{}
var LLVMProgramValues = &LLVMValuesMap{}
