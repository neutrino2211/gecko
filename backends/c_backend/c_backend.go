package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

var CurrentBackend interfaces.BackendInteface = nil
var Methods map[string]*ast.Method

// Init initializes scope info for an AST
func (info *CScopeInformation) InitForAst(a *ast.Ast) {
	info.Init()
	loadPrimitives(a)
}

func loadPrimitives(a *ast.Ast) {
	// Register primitive types as classes in the AST
	for typeName := range GeckoToCType {
		a.Classes[typeName] = &ast.Ast{
			Scope: typeName,
		}
	}
}

// NewReturn generates a void return statement
func (impl *CBackendImplementation) NewReturn(scope *ast.Ast) {
	info := CGetScopeInformation(scope)
	info.Code += "    return;\n"
}

// NewReturnLiteral generates a return statement with a value
func (impl *CBackendImplementation) NewReturnLiteral(scope *ast.Ast, literal *tokens.Expression) {
	info := CGetScopeInformation(scope)
	val := impl.ExpressionToCString(literal, scope)
	info.Code += "    return " + val + ";\n"
}

// Declaration handles declaration tokens
func (*CBackendImplementation) Declaration(scope *ast.Ast, decl *tokens.Declaration) {
	// Handled by NewDeclaration
}

// ParseExpression parses an expression
func (*CBackendImplementation) ParseExpression(scope *ast.Ast, exp *tokens.Expression) {
	// No-op, expressions are processed inline
}

// ProcessEntries processes all entries in a scope
func (impls *CBackendImplementation) ProcessEntries(scope *ast.Ast, entries []*tokens.Entry) {
	impls.Backend.ProcessEntries(entries, scope)
}

// NewDeclaration handles external declarations (declare keyword)
func (impls *CBackendImplementation) NewDeclaration(scope *ast.Ast, decl *tokens.Declaration) {
	if decl.Field != nil {
		impls.NewExternalVariable(scope, decl.Field)
	} else if decl.Method != nil {
		impls.NewExternalMethod(scope, decl.Method)
	} else if decl.ExternalType != nil {
		impls.NewExternalType(scope, decl.ExternalType)
	}
}

// NewExternalType handles external type declarations (opaque C types)
func (impls *CBackendImplementation) NewExternalType(scope *ast.Ast, ext *tokens.ExternalType) {
	rootScope := scope.GetRoot()
	rootScope.Classes[ext.Name] = &ast.Ast{
		Scope: ext.Name,
	}

	// Generate C typedef for opaque type
	info := CGetScopeInformation(scope)
	typedef := fmt.Sprintf("typedef struct %s %s;", ext.Name, ext.Name)
	info.TypeDefs = append(info.TypeDefs, typedef)
}

// NewExternalVariable handles external variable declarations
func (impls *CBackendImplementation) NewExternalVariable(scope *ast.Ast, f *tokens.Field) {
	info := CGetScopeInformation(scope)

	cType := TypeRefToCType(f.Type, scope)
	varName := f.Name

	// Generate extern declaration
	externDecl := fmt.Sprintf("extern %s %s;", cType, varName)
	info.Declarations = append(info.Declarations, externDecl)

	// Register the variable in AST
	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    f.Type != nil && f.Type.Const,
		IsVolatile: f.Type != nil && f.Type.Volatile,
		IsPointer:  f.Type != nil && f.Type.Pointer,
		IsExternal: true,
		Parent:     scope,
	}
	scope.Variables[f.Name] = fieldVariable
}

// NewExternalMethod handles external method declarations (e.g., printf)
func (impls *CBackendImplementation) NewExternalMethod(scope *ast.Ast, m *tokens.Method) {
	info := CGetScopeInformation(scope)

	// Determine return type
	returnType := "void"
	if m.Type != nil {
		returnType = TypeRefToCType(m.Type, scope)
	}

	// Build parameter list
	params := []string{}
	for _, arg := range m.Arguments {
		paramType := TypeRefToCType(arg.Type, scope)
		params = append(params, paramType+" "+arg.Name)
	}

	paramStr := strings.Join(params, ", ")

	// Handle variadic functions
	if m.Variardic {
		if len(params) > 0 {
			paramStr += ", ..."
		} else {
			paramStr = "..."
		}
	}

	// Generate extern declaration
	externDecl := fmt.Sprintf("extern %s %s(%s);", returnType, m.Name, paramStr)
	info.Declarations = append(info.Declarations, externDecl)

	// Register the method in AST
	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      nil, // External, no scope
		Arguments:  make([]ast.Variable, 0),
		Visibility: "external",
		Parent:     scope,
		Type:       returnType,
	}

	for _, arg := range m.Arguments {
		astMth.Arguments = append(astMth.Arguments, ast.Variable{
			Name:      arg.Name,
			IsPointer: arg.Type != nil && arg.Type.Pointer,
			Parent:    nil,
		})
	}

	scope.Methods[m.Name] = astMth
	Methods[scope.FullScopeName()+"#"+m.Name] = astMth
}

// NewClass handles class definitions
func (impls *CBackendImplementation) NewClass(scope *ast.Ast, c *tokens.Class) {
	classAst := &ast.Ast{
		Scope:  c.Name,
		Parent: scope,
	}

	classAst.Init(scope.ErrorScope)
	scope.Classes[c.Name] = classAst

	// If this is a generic class, register it and skip code generation
	if len(c.TypeParams) > 0 {
		Generics.RegisterGenericClass(c.Name, c)
		return
	}

	// Get parent scope info to add the struct definition
	info := CGetScopeInformation(scope)

	// Generate C struct definition
	impls.GenerateClassDef(scope, c, c.Name, nil)

	// Add to types list (already added by GenerateClassDef)
	_ = info

	// Process methods (they go into the functions list)
	for _, f := range c.Fields {
		if f.Method != nil {
			impls.NewMethod(classAst, f.Method)
		}
	}
}

// GenerateClassDef generates a C struct definition, optionally with type substitution
func (impls *CBackendImplementation) GenerateClassDef(scope *ast.Ast, c *tokens.Class, name string, typeArgs []string) {
	info := CGetScopeInformation(scope)

	// Get or create the class AST
	classAst, ok := scope.Classes[name]
	if !ok {
		classAst = &ast.Ast{
			Scope:  name,
			Parent: scope,
		}
		classAst.Init(scope.ErrorScope)
		scope.Classes[name] = classAst
	}

	var structDef string

	// Check for packed attribute
	isPacked := tokens.IsPacked(c.Attributes)
	if isPacked {
		structDef = "typedef struct __attribute__((packed)) {\n"
	} else {
		structDef = "typedef struct {\n"
	}

	// Add fields with type substitution
	for _, f := range c.Fields {
		if f.Field != nil {
			fieldType := TypeRefToCType(f.Field.Type, scope)
			// Substitute type parameters if this is a generic instantiation
			if len(typeArgs) > 0 && len(c.TypeParams) > 0 {
				fieldType = SubstituteTypeParams(fieldType, c.TypeParams, typeArgs)
			}
			structDef += "    " + fieldType + " " + f.Field.Name + ";\n"

			// Register field in the class AST for type tracking
			fieldVariable := ast.Variable{
				Name:      f.Field.Name,
				IsPointer: f.Field.Type != nil && f.Field.Type.Pointer,
				IsConst:   f.Field.Type != nil && f.Field.Type.Const,
				Parent:    classAst,
			}
			classAst.Variables[f.Field.Name] = fieldVariable

			// Store full type info in CProgramValues
			(*CProgramValues)[fieldVariable.GetFullName()] = &CValueInformation{
				CType:     fieldType,
				GeckoType: f.Field.Type,
			}
		}
	}

	structDef += "} " + name + ";\n"

	info.Types = append(info.Types, structDef)
}

// GenerateMethodDef generates a monomorphized method with type substitution
func (impl *CBackendImplementation) GenerateMethodDef(scope *ast.Ast, m *tokens.Method, name string, typeArgs []string) {
	info := CGetScopeInformation(scope)

	// Determine return type with substitution
	returnType := "void"
	if m.Type != nil {
		returnType = TypeRefToCType(m.Type, scope)
		if len(typeArgs) > 0 && len(m.TypeParams) > 0 {
			returnType = SubstituteTypeParams(returnType, m.TypeParams, typeArgs)
		}
	}

	// Build parameter list with substitution
	params := []string{}
	for _, arg := range m.Arguments {
		paramType := TypeRefToCType(arg.Type, scope)
		if len(typeArgs) > 0 && len(m.TypeParams) > 0 {
			paramType = SubstituteTypeParams(paramType, m.TypeParams, typeArgs)
		}
		paramName := arg.Name
		if IsFuncPointerType(paramType) {
			params = append(params, FormatFuncPointerDecl(paramType, paramName))
		} else {
			params = append(params, paramType+" "+paramName)
		}
	}

	paramStr := strings.Join(params, ", ")

	// Create method scope for body processing
	methodScope := ast.Ast{
		Scope:  name,
		Parent: scope,
	}
	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Register arguments in scope
	for _, arg := range m.Arguments {
		paramType := TypeRefToCType(arg.Type, scope)
		if len(typeArgs) > 0 && len(m.TypeParams) > 0 {
			paramType = SubstituteTypeParams(paramType, m.TypeParams, typeArgs)
		}
		argVariable := ast.Variable{
			Name:       arg.Name,
			IsPointer:  arg.Type != nil && arg.Type.Pointer,
			IsConst:    arg.Type != nil && arg.Type.Const,
			IsVolatile: arg.Type != nil && arg.Type.Volatile,
			IsArgument: true,
			Parent:     &methodScope,
		}
		methodScope.Variables[arg.Name] = argVariable
		(*CProgramValues)[argVariable.GetFullName()] = &CValueInformation{
			CType:     paramType,
			GeckoType: arg.Type,
		}
	}

	// Initialize method scope info
	mthInfo := &CScopeInformation{}
	mthInfo.Init()
	mthInfo.CurrentFunc = name
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		// Set up monomorph context for trait constraint resolution
		oldContext := CurrentMonomorphContext
		CurrentMonomorphContext = BuildMonomorphContext(m.TypeParams, typeArgs)

		loadPrimitives(&methodScope)
		impl.Backend.ProcessEntries(m.Value, &methodScope)

		// Restore previous context
		CurrentMonomorphContext = oldContext

		// Add implicit void return if needed
		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function with attributes
	attrStr := tokens.ToCAttributes(m.Attributes)
	var funcDecl string
	if attrStr != "" {
		funcDecl = fmt.Sprintf("%s %s %s(%s)", attrStr, returnType, name, paramStr)
	} else {
		funcDecl = fmt.Sprintf("%s %s(%s)", returnType, name, paramStr)
	}

	// Always add forward declaration for monomorphized functions
	info.Declarations = append(info.Declarations, funcDecl+";")

	if len(m.Value) > 0 {
		funcDef := funcDecl + " {\n" + mthInfo.Code + "}\n"
		info.Functions = append(info.Functions, funcDef)
	}
}

// GenerateClassMethodDef generates a method for a generic class instantiation
func (impl *CBackendImplementation) GenerateClassMethodDef(scope *ast.Ast, classToken *tokens.Class, m *tokens.Method, methodName string, className string, typeArgs []string) {
	info := CGetScopeInformation(scope.GetRoot())

	// Set up monomorph context FIRST so type substitution works for return type and params
	oldContext := CurrentMonomorphContext
	CurrentMonomorphContext = BuildMonomorphContext(classToken.TypeParams, typeArgs)

	// Determine return type with substitution
	returnType := "void"
	if m.Type != nil {
		returnType = TypeRefToCType(m.Type, scope)
	}

	// Build parameter list - self first, then other args
	params := []string{}
	for _, arg := range m.Arguments {
		paramName := arg.Name
		var paramType string

		if paramName == "self" {
			paramType = className + "*"
		} else {
			paramType = TypeRefToCType(arg.Type, scope)
		}

		if IsFuncPointerType(paramType) {
			params = append(params, FormatFuncPointerDecl(paramType, paramName))
		} else {
			params = append(params, paramType+" "+paramName)
		}
	}

	paramStr := strings.Join(params, ", ")

	// Create method scope for body processing
	methodScope := ast.Ast{
		Scope:  methodName,
		Parent: scope,
	}
	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Register arguments in scope
	for _, arg := range m.Arguments {
		paramName := arg.Name
		var paramType string
		var geckoType *tokens.TypeRef

		if paramName == "self" {
			paramType = className + "*"
			geckoType = &tokens.TypeRef{
				Type:    className,
				Pointer: true,
			}
		} else {
			paramType = TypeRefToCType(arg.Type, scope)
			if len(typeArgs) > 0 && len(classToken.TypeParams) > 0 {
				paramType = SubstituteTypeParams(paramType, classToken.TypeParams, typeArgs)
			}
			geckoType = arg.Type
		}

		argVariable := ast.Variable{
			Name:       paramName,
			IsPointer:  paramName == "self" || (arg.Type != nil && arg.Type.Pointer),
			IsConst:    arg.Type != nil && arg.Type.Const,
			IsVolatile: arg.Type != nil && arg.Type.Volatile,
			IsArgument: true,
			Parent:     &methodScope,
		}
		methodScope.Variables[paramName] = argVariable
		(*CProgramValues)[argVariable.GetFullName()] = &CValueInformation{
			CType:     paramType,
			GeckoType: geckoType,
		}
	}

	// Register method in scope for resolution
	astMth := &ast.Method{
		Name:   m.Name,
		Parent: scope,
	}
	// Register under the instantiated class
	classAst, ok := scope.Classes[className]
	if ok {
		classAst.Methods[m.Name] = astMth
	}

	// Initialize method scope info
	mthInfo := &CScopeInformation{}
	mthInfo.Init()
	mthInfo.CurrentFunc = methodName
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		impl.Backend.ProcessEntries(m.Value, &methodScope)

		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function
	attrStr := tokens.ToCAttributes(m.Attributes)
	var funcDecl string
	if attrStr != "" {
		funcDecl = fmt.Sprintf("%s %s %s(%s)", attrStr, returnType, methodName, paramStr)
	} else {
		funcDecl = fmt.Sprintf("%s %s(%s)", returnType, methodName, paramStr)
	}

	info.Declarations = append(info.Declarations, funcDecl+";")

	if len(m.Value) > 0 {
		funcDef := funcDecl + " {\n" + mthInfo.Code + "}\n"
		info.Functions = append(info.Functions, funcDef)
	}

	// Restore previous monomorph context
	CurrentMonomorphContext = oldContext
}

// FuncCall handles standalone function calls
func (impls *CBackendImplementation) FuncCall(scope *ast.Ast, f *tokens.FuncCall) {
	info := CGetScopeInformation(scope)
	callStr := impls.FuncCallToCString(f, scope)
	info.Code += "    " + callStr + ";\n"
}

// NewImplementation handles trait implementations
func (impl *CBackendImplementation) NewImplementation(scope *ast.Ast, i *tokens.Implementation) {
	// Similar to LLVM backend
	if i.For != "" {
		impl.CImplementationForClass(scope, i)
	} else {
		impl.CImplementationForArch(scope, i)
	}
}

// NewTrait handles trait definitions
func (impl *CBackendImplementation) NewTrait(scope *ast.Ast, t *tokens.Trait) {
	// Store trait methods for later implementation
	mthds := []*ast.Method{}
	for _, f := range t.Fields {
		m := f.ToMethodToken()
		astMth := &ast.Method{
			Name:       m.Name,
			Scope:      nil,
			Arguments:  make([]ast.Variable, 0),
			Visibility: m.Visibility,
			Parent:     scope,
			Type:       TypeRefToCType(m.Type, scope),
		}
		mthds = append(mthds, astMth)
	}
	scope.Traits[t.Name] = &mthds
}

// NewTraitMethod generates a C function for a trait implementation
// It handles the self parameter and uses the mangled name
func (impl *CBackendImplementation) NewTraitMethod(scope *ast.Ast, classScope *ast.Ast, m *tokens.Method, mangledName string, className string) {
	methodScope := ast.Ast{
		Scope:  mangledName,
		Parent: scope,
	}

	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Determine return type
	returnType := "void"
	if m.Type != nil {
		m.Type.Check(scope)
		returnType = TypeRefToCType(m.Type, scope)
	}

	// Build parameter list, handling self specially
	params := []string{}
	for _, arg := range m.Arguments {
		paramName := arg.Name
		var paramType string

		if paramName == "self" {
			// Self parameter becomes pointer to class type
			paramType = className + "*"
		} else {
			paramType = TypeRefToCType(arg.Type, scope)
		}

		if IsFuncPointerType(paramType) {
			params = append(params, FormatFuncPointerDecl(paramType, paramName))
		} else {
			params = append(params, paramType+" "+paramName)
		}

		// Register argument as variable in method scope
		argVariable := ast.Variable{
			Name:       paramName,
			IsPointer:  paramName == "self" || (arg.Type != nil && arg.Type.Pointer),
			IsConst:    arg.Type != nil && arg.Type.Const,
			IsVolatile: arg.Type != nil && arg.Type.Volatile,
			IsArgument: true,
			Parent:     &methodScope,
		}
		methodScope.Variables[paramName] = argVariable

		(*CProgramValues)[argVariable.GetFullName()] = &CValueInformation{
			CType:     paramType,
			GeckoType: arg.Type,
		}
	}

	paramStr := strings.Join(params, ", ")

	// Create AST method with mangled name
	// Mark as external so GetFullName returns just the mangled name
	astMth := &ast.Method{
		Name:       mangledName,
		Scope:      &methodScope,
		Arguments:  make([]ast.Variable, 0),
		Visibility: "external",
		Parent:     scope,
		Type:       returnType,
	}

	if len(m.Value) == 0 {
		astMth.Scope = nil
	}

	scope.Methods[mangledName] = astMth
	Methods[scope.FullScopeName()+"#"+mangledName] = astMth

	// Initialize method scope info
	mthInfo := &CScopeInformation{}
	mthInfo.Init()
	mthInfo.CurrentFunc = mangledName
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		impl.Backend.ProcessEntries(m.Value, &methodScope)

		if returnType == "void" && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function signature
	funcDecl := fmt.Sprintf("%s %s(%s)", returnType, mangledName, paramStr)

	// Add to parent scope info
	parentInfo := CGetScopeInformation(scope)

	if len(m.Value) > 0 {
		funcDef := funcDecl + " {\n" + mthInfo.Code + "}\n"
		parentInfo.Functions = append(parentInfo.Functions, funcDef)
	} else {
		parentInfo.Declarations = append(parentInfo.Declarations, funcDecl+";")
	}

	// Add forward declaration for trait methods
	parentInfo.Declarations = append(parentInfo.Declarations, funcDecl+";")
}

// NewMethod generates a C function
func (impl *CBackendImplementation) NewMethod(scope *ast.Ast, m *tokens.Method) {
	// If this is a generic method, register it and skip code generation
	if len(m.TypeParams) > 0 {
		fullName := scope.GetFullName() + "__" + m.Name
		Generics.RegisterGenericMethod(fullName, m)
		// Still register in scope for resolution
		astMth := &ast.Method{
			Name:       m.Name,
			Scope:      nil,
			Arguments:  make([]ast.Variable, 0),
			Visibility: m.Visibility,
			Parent:     scope,
			Type:       "generic",
		}
		scope.Methods[m.Name] = astMth
		Methods[scope.FullScopeName()+"#"+m.Name] = astMth
		return
	}

	methodScope := ast.Ast{
		Scope:  m.Name,
		Parent: scope,
	}

	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Determine return type
	returnType := "void"
	if m.Type != nil {
		m.Type.Check(scope)
		returnType = TypeRefToCType(m.Type, scope)
	}

	// Check if this is a class method (scope is a class AST)
	isClassMethod := false
	className := ""
	if scope.Parent != nil {
		if _, ok := scope.Parent.Classes[scope.Scope]; ok {
			isClassMethod = true
			className = scope.Scope
		}
	}

	// Build parameter list
	params := []string{}
	for _, arg := range m.Arguments {
		paramName := arg.Name
		var paramType string
		var geckoType *tokens.TypeRef

		// Handle self parameter for class methods
		if paramName == "self" && isClassMethod {
			paramType = className + "*"
			// Create synthetic TypeRef for self
			geckoType = &tokens.TypeRef{
				Type:    className,
				Pointer: true,
			}
		} else {
			paramType = TypeRefToCType(arg.Type, scope)
			geckoType = arg.Type
		}

		if IsFuncPointerType(paramType) {
			params = append(params, FormatFuncPointerDecl(paramType, paramName))
		} else {
			params = append(params, paramType+" "+paramName)
		}

		// Register argument as variable in method scope
		argVariable := ast.Variable{
			Name:       arg.Name,
			IsPointer:  (paramName == "self" && isClassMethod) || (arg.Type != nil && arg.Type.Pointer),
			IsConst:    arg.Type != nil && arg.Type.Const,
			IsVolatile: arg.Type != nil && arg.Type.Volatile,
			IsArgument: true,
			Parent:     &methodScope,
		}
		methodScope.Variables[arg.Name] = argVariable

		(*CProgramValues)[argVariable.GetFullName()] = &CValueInformation{
			CType:     paramType,
			GeckoType: geckoType,
		}
	}

	paramStr := strings.Join(params, ", ")
	if m.Variardic {
		if len(params) > 0 {
			paramStr += ", ..."
		} else {
			paramStr = "..."
		}
	}

	// Create AST method
	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      &methodScope,
		Arguments:  make([]ast.Variable, 0),
		Visibility: m.Visibility,
		Parent:     scope,
		Type:       returnType,
	}

	if len(m.Value) == 0 {
		// Declaration only, no body
		astMth.Scope = nil
	}

	scope.Methods[m.Name] = astMth
	Methods[scope.FullScopeName()+"#"+m.Name] = astMth

	// Initialize method scope info
	mthInfo := &CScopeInformation{}
	mthInfo.Init()
	mthInfo.CurrentFunc = m.Name
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		impl.Backend.ProcessEntries(m.Value, &methodScope)

		// Check if we need to add implicit void return
		// Don't add return for @naked or @noreturn functions
		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function signature with attributes
	funcName := astMth.GetFullName()
	attrStr := tokens.ToCAttributes(m.Attributes)
	var funcDecl string
	if attrStr != "" {
		funcDecl = fmt.Sprintf("%s %s %s(%s)", attrStr, returnType, funcName, paramStr)
	} else {
		funcDecl = fmt.Sprintf("%s %s(%s)", returnType, funcName, paramStr)
	}

	// Add to root scope info (so class methods end up in the output)
	parentInfo := CGetScopeInformation(scope.GetRoot())

	if len(m.Value) > 0 {
		// Full function definition
		funcDef := funcDecl + " {\n" + mthInfo.Code + "}\n"
		parentInfo.Functions = append(parentInfo.Functions, funcDef)
	} else {
		// Declaration only (for forward declarations)
		parentInfo.Declarations = append(parentInfo.Declarations, funcDecl+";")
	}

	// Copy arguments to AST method
	for _, arg := range m.Arguments {
		astMth.Arguments = append(astMth.Arguments, ast.Variable{
			Name:      arg.Name,
			IsPointer: arg.Type != nil && arg.Type.Pointer,
			Parent:    &methodScope,
		})
	}
}

// NewVariable generates a C variable declaration
func (impl *CBackendImplementation) NewVariable(scope *ast.Ast, f *tokens.Field) {
	info := CGetScopeInformation(scope)

	// Type inference: if no explicit type, try to infer from the value
	if f.Type == nil {
		if f.Value != nil {
			// Create a symbol resolver that looks up variables in scope
			resolveSymbol := func(name string) *tokens.TypeRef {
				opt := scope.ResolveSymbolAsVariable(name)
				if !opt.IsNil() {
					v := opt.Unwrap()
					fullName := v.GetFullName()
					if valInfo, ok := (*CProgramValues)[fullName]; ok {
						return valInfo.GeckoType
					}
				}
				return nil
			}

			f.Type = tokens.InferType(f.Value, resolveSymbol)
		}

		// If still nil, error
		if f.Type == nil {
			scope.ErrorScope.NewCompileTimeError(
				"Cannot infer type",
				"unable to infer variable type; please provide an explicit type annotation",
				f.Pos,
			)
			f.Type = &tokens.TypeRef{Type: "int"}
		}
	}

	// Check for const - either from Type.Const or from Mutability == "const"
	isConst := (f.Type != nil && f.Type.Const) || f.Mutability == "const"

	if f.Value == nil && isConst {
		scope.ErrorScope.NewCompileTimeError("Uninitialized constant", "constant must be initialized with a value", f.Pos)
		return
	}

	cType := TypeRefToCType(f.Type, scope)

	// Check if this is a global variable (no current function context)
	isGlobal := info.CurrentFunc == ""

	// Register variable in AST
	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    isConst,
		IsVolatile: f.Type.Volatile,
		IsPointer:  f.Type.Pointer,
		IsExternal: f.Visibility == "external",
		Parent:     scope,
	}

	// For globals, use the full qualified name; for locals, use just the name
	var varName string
	if isGlobal {
		varName = fieldVariable.GetFullName()
	} else {
		varName = f.Name
	}

	(*CProgramValues)[fieldVariable.GetFullName()] = &CValueInformation{
		CType:     cType,
		GeckoType: f.Type,
	}

	scope.Variables[f.Name] = fieldVariable

	if isGlobal {
		// Generate global variable declaration with attributes
		impl.NewGlobalVariable(scope, f, cType, varName)
	} else {
		// Generate local variable declaration
		impl.NewLocalVariable(scope, f, cType, varName)
	}
}

// NewGlobalVariable generates a global C variable declaration with optional attributes
func (impl *CBackendImplementation) NewGlobalVariable(scope *ast.Ast, f *tokens.Field, cType string, varName string) {
	info := CGetScopeInformation(scope)

	// Build attributes string
	attrStr := tokens.ToCAttributes(f.Attributes)

	// Handle const modifier - either from Type.Const, from Mutability == "const",
	// or from the inner type's Const for sized arrays
	isConst := (f.Type != nil && f.Type.Const) || f.Mutability == "const"
	// For sized arrays, also check the inner type's const flag
	if f.Type != nil && f.Type.Size != nil && f.Type.Size.Type != nil && f.Type.Size.Type.Const {
		isConst = true
	}
	typeDecl := cType
	if isConst {
		typeDecl = "const " + typeDecl
	}

	// Handle sized arrays: for [N]T, we need to generate "T name[N]" format
	var varDecl string
	if f.Type.Size != nil {
		// Sized array: e.g., [4096]uint8 -> uint8_t name[4096]
		baseType := TypeRefToCType(f.Type.Size.Type, scope)
		if isConst {
			baseType = "const " + baseType
		}
		if attrStr != "" {
			varDecl = fmt.Sprintf("%s %s %s[%s]", attrStr, baseType, varName, f.Type.Size.Size)
		} else {
			varDecl = fmt.Sprintf("%s %s[%s]", baseType, varName, f.Type.Size.Size)
		}
	} else {
		// Regular type
		if attrStr != "" {
			varDecl = fmt.Sprintf("%s %s %s", attrStr, typeDecl, varName)
		} else {
			varDecl = fmt.Sprintf("%s %s", typeDecl, varName)
		}
	}

	// Add initializer if present
	if f.Value != nil {
		val := impl.ExpressionToCString(f.Value, scope)
		varDecl += " = " + val
	}

	varDecl += ";"

	info.Globals = append(info.Globals, varDecl)
}

// NewLocalVariable generates a local C variable declaration
func (impl *CBackendImplementation) NewLocalVariable(scope *ast.Ast, f *tokens.Field, cType string, varName string) {
	info := CGetScopeInformation(scope)

	// Handle const modifier - either from Type.Const or from Mutability == "const"
	isConst := (f.Type != nil && f.Type.Const) || f.Mutability == "const"
	typeDecl := cType
	if isConst {
		typeDecl = "const " + typeDecl
	}

	// Generate variable declaration/definition
	var varDecl string

	// Handle function pointer types
	if IsFuncPointerType(cType) {
		if f.Value != nil {
			val := impl.ExpressionToCString(f.Value, scope)
			varDecl = fmt.Sprintf("    %s = %s;\n", FormatFuncPointerDecl(cType, varName), val)
		} else {
			varDecl = fmt.Sprintf("    %s;\n", FormatFuncPointerDecl(cType, varName))
		}
	} else if f.Type.Size != nil {
		// Handle sized arrays: for [N]T, we need to generate "T name[N]" format
		baseType := TypeRefToCType(f.Type.Size.Type, scope)
		if isConst {
			baseType = "const " + baseType
		}
		if f.Value != nil {
			val := impl.ExpressionToCString(f.Value, scope)
			varDecl = fmt.Sprintf("    %s %s[%s] = %s;\n", baseType, varName, f.Type.Size.Size, val)
		} else {
			varDecl = fmt.Sprintf("    %s %s[%s];\n", baseType, varName, f.Type.Size.Size)
		}
	} else {
		if f.Value != nil {
			val := impl.ExpressionToCString(f.Value, scope)
			varDecl = fmt.Sprintf("    %s %s = %s;\n", typeDecl, varName, val)
		} else {
			varDecl = fmt.Sprintf("    %s %s;\n", typeDecl, varName)
		}
	}

	info.Code += varDecl
}

// Helper functions for implementations

func (impl *CBackendImplementation) CImplementationForClass(scope *ast.Ast, i *tokens.Implementation) {
	classOpt := scope.ResolveClass(i.For)
	traitOpt := scope.ResolveTrait(i.Name)

	if classOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the class '"+i.For+"'", i.Pos)
		return
	}

	if traitOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the trait '"+i.Name+"'", i.Pos)
		return
	}

	class := classOpt.Unwrap()
	traitMthds := traitOpt.Unwrap()
	className := i.For
	traitName := i.Name

	// Build mangled trait name with type arguments (e.g., "Add__Point" for Add<Point>)
	mangledTraitName := traitName
	if len(i.TypeArgs) > 0 {
		for _, typeArg := range i.TypeArgs {
			mangledTraitName += "__" + TypeRefToCType(typeArg, scope)
		}
	}

	if i.Fields != nil && i.Default {
		scope.ErrorScope.NewCompileTimeError(
			"Implementation Error",
			"A default trait implementation must not have a body",
			i.Pos,
		)
		return
	}

	var mthdList []*ast.Method

	if i.Default {
		mthdList = *traitMthds
	} else {
		for _, f := range i.Fields {
			m := f.ToMethodToken()
			mangledName := className + "__" + mangledTraitName + "__" + m.Name
			impl.NewTraitMethod(scope, class, m, mangledName, className)
			if astMth, ok := scope.Methods[mangledName]; ok {
				mthdList = append(mthdList, astMth)
			}
		}
	}

	class.Traits[mangledTraitName] = &mthdList
}

func (impl *CBackendImplementation) CImplementationForArch(scope *ast.Ast, i *tokens.Implementation) {
	if i.Default {
		scope.ErrorScope.NewCompileTimeError(
			"Implementation Error",
			"An architecture implementation must not be default",
			i.Pos,
		)
		return
	}

	if scope.Config.Arch == i.Name {
		for _, f := range i.Fields {
			m := f.ToMethodToken()
			impl.NewMethod(scope, m)
		}
	} else {
		scope.ErrorScope.NewCompileTimeWarning(
			"Arch Implementation",
			"Implementation for the arch '"+i.Name+"' was skipped due to target being '"+scope.Config.Arch+"'",
			i.Pos,
		)
	}
}

// NewIf handles if statements (basic implementation for now)
func (impl *CBackendImplementation) NewIf(scope *ast.Ast, i *tokens.If) {
	info := CGetScopeInformation(scope)

	condition := impl.ExpressionToCString(i.Expression, scope)
	info.Code += fmt.Sprintf("    if (%s) {\n", condition)

	// Process if body
	for _, entry := range i.Value {
		impl.processEntry(scope, entry)
	}

	info.Code += "    }\n"

	// Handle else if
	if i.ElseIf != nil {
		impl.NewElseIf(scope, i.ElseIf)
	}

	// Handle else
	if i.Else != nil {
		impl.NewElse(scope, i.Else)
	}
}

// NewElseIf handles else if statements
func (impl *CBackendImplementation) NewElseIf(scope *ast.Ast, ei *tokens.ElseIf) {
	info := CGetScopeInformation(scope)

	condition := impl.ExpressionToCString(ei.Expression, scope)
	info.Code += fmt.Sprintf("    else if (%s) {\n", condition)

	// Process else if body
	for _, entry := range ei.Value {
		impl.processEntry(scope, entry)
	}

	info.Code += "    }\n"

	// Handle chained else if
	if ei.ElseIf != nil {
		impl.NewElseIf(scope, ei.ElseIf)
	}

	// Handle else
	if ei.Else != nil {
		impl.NewElse(scope, ei.Else)
	}
}

// NewElse handles else statements
func (impl *CBackendImplementation) NewElse(scope *ast.Ast, e *tokens.Else) {
	info := CGetScopeInformation(scope)

	info.Code += "    else {\n"

	// Process else body
	for _, entry := range e.Value {
		impl.processEntry(scope, entry)
	}

	info.Code += "    }\n"
}

// NewLoop handles loop statements (basic implementation)
func (impl *CBackendImplementation) NewLoop(scope *ast.Ast, l *tokens.Loop) {
	info := CGetScopeInformation(scope)

	if l.ForExpression != nil {
		// for condition style: for condition { ... }
		condition := impl.ExpressionToCString(l.ForExpression, scope)
		info.Code += fmt.Sprintf("    while (%s) {\n", condition)
	} else if l.WhileExpr != nil {
		// while condition style: while condition { ... }
		condition := impl.ExpressionToCString(l.WhileExpr, scope)
		info.Code += fmt.Sprintf("    while (%s) {\n", condition)
	} else if l.ForOf != nil {
		// for-of loop: iterate over array
		// This is a simplified implementation
		info.Code += "    /* for-of loop not fully implemented */\n"
		info.Code += "    {\n"
	} else if l.ForIn != nil {
		// for-in loop: iterate over range
		info.Code += "    /* for-in loop not fully implemented */\n"
		info.Code += "    {\n"
	} else {
		// Infinite loop
		info.Code += "    while (1) {\n"
	}

	// Process loop body
	for _, entry := range l.Value {
		impl.processEntry(scope, entry)
	}

	info.Code += "    }\n"
}

// NewAssignment handles variable assignment
func (impl *CBackendImplementation) NewAssignment(scope *ast.Ast, a *tokens.Assignment) {
	info := CGetScopeInformation(scope)

	// Resolve the variable
	varOpt := scope.ResolveSymbolAsVariable(a.Name)
	varName := a.Name
	isPointer := false
	if !varOpt.IsNil() {
		variable := varOpt.Unwrap()
		isPointer = variable.IsPointer
		// For local variables and arguments, use just the name (not the full qualified name)
		if variable.IsArgument || variable.Parent == scope {
			varName = variable.Name
		} else {
			varName = variable.GetFullName()
		}
	}

	value := impl.ExpressionToCString(a.Value, scope)

	// Handle field assignment (e.g., p.x = value)
	if a.Field != "" {
		accessor := "."
		if isPointer {
			accessor = "->"
		}
		if a.Index != nil {
			indexStr := impl.ExpressionToCString(a.Index, scope)
			info.Code += fmt.Sprintf("    %s%s%s[%s] = %s;\n", varName, accessor, a.Field, indexStr, value)
		} else {
			info.Code += fmt.Sprintf("    %s%s%s = %s;\n", varName, accessor, a.Field, value)
		}
		return
	}

	// Handle indexed assignment (e.g., arr[0] = value)
	if a.Index != nil {
		indexStr := impl.ExpressionToCString(a.Index, scope)
		info.Code += fmt.Sprintf("    %s[%s] = %s;\n", varName, indexStr, value)
		return
	}

	info.Code += fmt.Sprintf("    %s = %s;\n", varName, value)
}

// NewAsm handles inline assembly statements
func (impl *CBackendImplementation) NewAsm(scope *ast.Ast, asm *tokens.Asm) {
	info := CGetScopeInformation(scope)

	// Strip quotes from the assembly code string (participle captures them)
	asmCode := asm.Code
	if len(asmCode) >= 2 && asmCode[0] == '"' && asmCode[len(asmCode)-1] == '"' {
		asmCode = asmCode[1 : len(asmCode)-1]
	}

	// Generate GNU-style inline assembly with volatile qualifier
	info.Code += fmt.Sprintf("    __asm__ volatile (\"%s\");\n", asmCode)
}

// NewBreak generates a break statement
func (impl *CBackendImplementation) NewBreak(scope *ast.Ast) {
	info := CGetScopeInformation(scope)
	info.Code += "    break;\n"
}

// NewContinue generates a continue statement
func (impl *CBackendImplementation) NewContinue(scope *ast.Ast) {
	info := CGetScopeInformation(scope)
	info.Code += "    continue;\n"
}

// processEntry handles a single entry in a block (for control flow bodies)
func (impl *CBackendImplementation) processEntry(scope *ast.Ast, entry *tokens.Entry) {
	if entry.Return != nil {
		impl.NewReturnLiteral(scope, entry.Return)
	} else if entry.VoidReturn != nil {
		impl.NewReturn(scope)
	} else if entry.Break != nil {
		impl.NewBreak(scope)
	} else if entry.Continue != nil {
		impl.NewContinue(scope)
	} else if entry.Intrinsic != nil {
		impl.IntrinsicStatement(scope, entry.Intrinsic)
	} else if entry.FuncCall != nil {
		impl.FuncCall(scope, entry.FuncCall)
	} else if entry.Field != nil {
		impl.NewVariable(scope, entry.Field)
	} else if entry.Assignment != nil {
		impl.NewAssignment(scope, entry.Assignment)
	} else if entry.If != nil {
		impl.NewIf(scope, entry.If)
	} else if entry.Loop != nil {
		impl.NewLoop(scope, entry.Loop)
	} else if entry.Asm != nil {
		impl.NewAsm(scope, entry.Asm)
	}
}
