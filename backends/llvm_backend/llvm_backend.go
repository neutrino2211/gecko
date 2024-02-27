package llvmbackend

import (
	"fmt"

	"github.com/alecthomas/repr"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

var CurrentBackend interfaces.BackendInterface = nil
var FuncCalls map[string]*ir.InstCall
var Methods map[string]*ast.Method
var LLVMExecutionContext *ExecutionContext = nil

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

	info.LocalContext.MainBlock.NewRet(nil)
}

func (impl *LLVMBackendImplementation) NewReturnLiteral(scope *ast.Ast, literal *tokens.Expression) {
	info := LLVMGetScopeInformation(scope)

	val := impl.ExpressionToLLIRValue(literal, scope, &tokens.TypeRef{})

	info.LocalContext.MainBlock.NewRet(val)
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
		Scope:  c.Name,
		Parent: scope,
	}

	classAst.Init(scope.ErrorScope)
	scope.Classes[c.Name] = classAst

	for _, f := range c.Fields {
		if f.Method != nil {
			impls.NewMethod(classAst, f.Method)
		}

		if f.Field != nil {
			impls.NewVariable(classAst, f.Field)
		}
	}
}

func (impls *LLVMBackendImplementation) FuncCall(scope *ast.Ast, f *tokens.FuncCall) {
	mth := scope.ResolveMethod(f.Function)

	if mth.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Function resolution error", fmt.Sprintf("Unable to resolve the function \"%s\"", f.Function), f.Pos)
		return
	}

	mthUnwrapped := mth.Unwrap()

	var info *LLVMScopeInformation = nil
	var fn *ir.Func

	if mthUnwrapped.Scope != nil {
		info = LLVMGetScopeInformation(mthUnwrapped.Scope)
		fn = info.LocalContext.Func
	} else {
		info = LLVMGetScopeInformation(scope)
		fn = ir.NewFunc(f.Function, impls.TypeRefGetLLIRType(&tokens.TypeRef{Type: mthUnwrapped.Type}, scope))
		fn.Linkage = enum.LinkageExternal
	}

	fn.CallingConv = CallingConventions[scope.Config.Arch][scope.Config.Platform]

	args := make([]value.Value, 0)

	for _, a := range f.Arguments {
		tr := &tokens.TypeRef{}

		args = append(args, impls.ExpressionToLLIRValue(a.Value, scope, tr))
	}

	call := info.LocalContext.MainBlock.NewCall(fn, args...)

	FuncCalls[scope.FullScopeName()+"#"+f.Function] = call
}

func (impl *LLVMBackendImplementation) NewImplementation(scope *ast.Ast, i *tokens.Implementation) {
	if i.For != "" {
		impl.LLVMImplementationForClass(scope, i)
	} else {
		impl.LLVMImplementationForArch(scope, i)
	}
}

func (impl *LLVMBackendImplementation) NewTrait(scope *ast.Ast, t *tokens.Trait) {

}

func (impl *LLVMBackendImplementation) NewMethod(scope *ast.Ast, m *tokens.Method) {
	methodScope := ast.Ast{
		Scope:  m.Name,
		Parent: scope,
	}

	info := LLVMGetScopeInformation(scope)

	fnParams := make([]*ir.Param, 0)

	for _, a := range m.Arguments {
		ty := impl.TypeRefGetLLIRType(a.Type, scope)

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

	(*LLVMScopeDataMap)[astMth.GetFullName()] = mthInfo
	repr.Println(*LLVMScopeDataMap, mthInfo)

	info.ChildContexts[astMth.GetFullName()] = mthInfo.LocalContext
	info.ProgramContext.Module.Funcs = append(info.ProgramContext.Module.Funcs, mthInfo.LocalContext.Func)

	// Add arguments as variables

	for _, v := range m.Arguments {
		repr.Println(v.Type, v.Name)
		mVariable := ast.Variable{
			IsPointer:  v.Type.Pointer,
			IsConst:    v.Type.Const,
			IsExternal: false,
			IsArgument: true,
			Name:       v.Name,
			Parent:     &methodScope,
		}

		methodScope.Variables[v.Name] = mVariable

		vIrType := impl.TypeRefGetLLIRType(v.Type, scope)

		(*LLVMProgramValues)[mVariable.GetFullName()] = &LLVMValueInformation{
			Type:      vIrType,
			Value:     ir.NewParam(v.Name, vIrType),
			GeckoType: v.Type,
		}
	}

	impl.Backend.ProcessEntries(m.Value, &methodScope)

	impl.LLVMAssignArgumentsToMethodArguments(m.Arguments, astMth)

	// If no return is specified, inject a void return
	if mthInfo.LocalContext.MainBlock != nil && mthInfo.LocalContext.MainBlock.Term == nil {
		t := true
		m.Value = append(m.Value, &tokens.Entry{VoidReturn: &t})
	}

	// if len(m.Value) > 0 {
	// 	methodScope.LocalContext.MainBlock.NewRet(constant.NewInt(types.I1, 0))
	// }
	Methods[scope.FullScopeName()+"#"+m.Name] = astMth
}

func (impl *LLVMBackendImplementation) NewVariable(scope *ast.Ast, f *tokens.Field) {
	if f.Type == nil {
		scope.ErrorScope.NewCompileTimeError("TODO: Infer variable type", "variable type inference not implemented", f.Pos)
		f.Type = &tokens.TypeRef{}
	}

	if f.Value == nil && f.Type.Const {
		scope.ErrorScope.NewCompileTimeError("Uninitialesed constant", "constant must be initialised with a value", f.Pos)
		f.Value = &tokens.Expression{}
	}

	repr.Println(scope.GetFullName())

	val := impl.ExpressionToLLIRValue(f.Value, scope, f.Type)

	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    f.Type.Const,
		IsPointer:  f.Type.Pointer,
		IsExternal: false,
		Parent:     scope,
	}

	if f.Visibility == "external" {
		fieldVariable.IsExternal = true
	}

	(*LLVMProgramValues)[fieldVariable.GetFullName()] = &LLVMValueInformation{
		Type:      impl.TypeRefGetLLIRType(f.Type, scope),
		Value:     val,
		GeckoType: f.Type,
	}

	scope.Variables[f.Name] = fieldVariable
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
