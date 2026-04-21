package llvmbackend

import (
	"fmt"
	"strconv"

	"github.com/alecthomas/repr"
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
		Scope:        c.Name,
		Parent:       scope,
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
			impls.NewMethod(classAst, f.Method)
		}
	}

	// Create LLVM struct type
	info := LLVMGetScopeInformation(scope)
	structType := types.NewStruct(fieldTypes...)
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
}

// NewClassField registers a field in the class AST without generating local variable code
func (impls *LLVMBackendImplementation) NewClassField(scope *ast.Ast, f *tokens.Field) {
	if f.Type == nil {
		scope.ErrorScope.NewCompileTimeError("TODO: Infer variable type", "variable type inference not implemented", f.Pos)
		f.Type = &tokens.TypeRef{Type: "int"}
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
	// TODO: Implement chained method calls for LLVM backend
	scope.ErrorScope.NewCompileTimeError("Not Implemented", "Chained method calls are not yet supported in the LLVM backend", m.Pos)
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
	if i.GetFor() != "" {
		impl.LLVMImplementationForClass(scope, i)
	} else {
		impl.LLVMImplementationForArch(scope, i)
	}
}

func (impl *LLVMBackendImplementation) NewTrait(scope *ast.Ast, t *tokens.Trait) {

}

func (impl *LLVMBackendImplementation) NewEnum(scope *ast.Ast, e *tokens.Enum) {
	// TODO: Implement LLVM enum generation
}

func (impl *LLVMBackendImplementation) NewMethod(scope *ast.Ast, m *tokens.Method) {
	methodScope := ast.Ast{
		Scope:        m.Name,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
		SourceFile:   scope.GetSourceFile(),
	}

	info := LLVMGetScopeInformation(scope)

	fnParams := make([]*ir.Param, 0)

	for _, a := range m.Arguments {
		// Validate parameter type
		if a.Type != nil {
			a.Type.Check(scope)
		}
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
			IsVolatile: v.Type.Volatile,
			IsExternal: false,
			IsArgument: true,
			Name:       v.Name,
			Parent:     &methodScope,
		}

		methodScope.Variables[v.Name] = mVariable

		vIrType := impl.TypeRefGetLLIRType(v.Type, scope)

		(*LLVMProgramValues)[mVariable.GetFullName()] = &LLVMValueInformation{
			Type:       vIrType,
			Value:      ir.NewParam(v.Name, vIrType),
			GeckoType:  v.Type,
			IsVolatile: v.Type.Volatile,
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
	if f.Type == nil {
		scope.ErrorScope.NewCompileTimeError("TODO: Infer variable type", "variable type inference not implemented", f.Pos)
		f.Type = &tokens.TypeRef{}
	}

	// Check for const - either from Type.Const or from Mutability == "const"
	isConst := (f.Type != nil && f.Type.Const) || f.Mutability == "const"

	if f.Value == nil && isConst {
		scope.ErrorScope.NewCompileTimeError("Uninitialized constant", "constant must be initialized with a value", f.Pos)
		f.Value = &tokens.Expression{}
	}

	repr.Println(scope.GetFullName())

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
					impl.NewVolatileStore(info.LocalContext.MainBlock, val, allocaInst, f.Type.Volatile)
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
