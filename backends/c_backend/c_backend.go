package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
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
	// Generate drop calls for droppable variables before returning
	impl.generateDropCalls(scope, info)
	info.Code += "    return;\n"
}

// NewReturnLiteral generates a return statement with a value
func (impl *CBackendImplementation) NewReturnLiteral(scope *ast.Ast, literal *tokens.Expression) {
	info := CGetScopeInformation(scope)

	// Validate return type matches declared function return type
	impl.CheckReturnType(literal, scope)

	val := impl.ExpressionToCString(literal, scope)

	// Check if we have droppable variables
	droppables := impl.getDroppableVariables(scope)
	if len(droppables) > 0 {
		// Save return value to temp, drop variables, then return
		returnType := impl.inferExpressionType(literal, scope)
		if returnType != "" && returnType != "void" {
			tempVar := "__drop_ret_val"
			info.Code += fmt.Sprintf("    %s %s = %s;\n", returnType, tempVar, val)
			impl.generateDropCalls(scope, info)
			info.Code += fmt.Sprintf("    return %s;\n", tempVar)
			return
		}
	}

	info.Code += "    return " + val + ";\n"
}

// getDroppableVariables returns variables in scope that implement Drop trait
func (impl *CBackendImplementation) getDroppableVariables(scope *ast.Ast) []struct {
	Name       string
	DropMethod string
	ClassName  string
} {
	var droppables []struct {
		Name       string
		DropMethod string
		ClassName  string
	}

	// Check if there's a Drop hook registered
	modulePath := scope.GetRoot().Scope
	dropHook := hooks.GetHookRegistry().GetHook(modulePath, hooks.HookDrop)
	if dropHook == nil {
		return droppables
	}

	dropMethodName := ""
	if len(dropHook.Methods) > 0 {
		dropMethodName = dropHook.Methods[0]
	}

	// Iterate through local variables in this scope (not parent scopes)
	for varName, variable := range scope.Variables {
		// Skip arguments - they're borrowed, not owned
		if variable.IsArgument {
			continue
		}

		// Get the variable's type
		fullName := variable.GetFullName()
		valueInfo, ok := (*CProgramValues)[fullName]
		if !ok || valueInfo.GeckoType == nil {
			continue
		}

		typeName := valueInfo.GeckoType.Type
		// Skip primitives and pointers
		if _, isPrimitive := GeckoToCType[typeName]; isPrimitive {
			continue
		}
		if valueInfo.GeckoType.Pointer {
			continue
		}

		// Check if this type implements Drop trait
		classOpt := scope.ResolveClass(typeName)
		if classOpt.IsNil() {
			continue
		}
		class := classOpt.Unwrap()

		// Check if the class has the Drop trait implemented
		if _, hasDrop := class.Traits[dropHook.TraitName]; hasDrop {
			mangledMethod := typeName + "__" + dropHook.TraitName + "__" + dropMethodName
			droppables = append(droppables, struct {
				Name       string
				DropMethod string
				ClassName  string
			}{
				Name:       varName,
				DropMethod: mangledMethod,
				ClassName:  typeName,
			})
		}
	}

	return droppables
}

// generateDropCalls generates drop method calls for all droppable variables
func (impl *CBackendImplementation) generateDropCalls(scope *ast.Ast, info *CScopeInformation) {
	droppables := impl.getDroppableVariables(scope)

	// Drop in reverse order of declaration (LIFO)
	for i := len(droppables) - 1; i >= 0; i-- {
		d := droppables[i]
		info.Code += fmt.Sprintf("    %s(&%s);\n", d.DropMethod, d.Name)
	}
}

// inferExpressionType infers the C type of an expression for temp variable declaration
func (impl *CBackendImplementation) inferExpressionType(expr *tokens.Expression, scope *ast.Ast) string {
	exprType := impl.GetTypeOfExpression(expr, scope)
	if exprType != nil {
		return TypeRefToCType(exprType, scope)
	}
	return ""
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
	// Store Gecko type for type checking
	geckoReturnType := "void"
	if m.Type != nil {
		geckoReturnType = m.Type.Type
	}
	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      nil, // External, no scope
		Arguments:  make([]ast.Variable, 0),
		Visibility: "external",
		Parent:     scope,
		Type:       geckoReturnType,
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
		Scope:        c.Name,
		Parent:       scope,
		Visibility:   c.Visibility,
		OriginModule: scope.GetRoot().Scope,
	}

	classAst.Init(scope.ErrorScope)
	scope.Classes[c.Name] = classAst

	// Register fields in the class AST for type checking (needed for both generic and non-generic)
	for _, f := range c.Fields {
		if f.Field != nil {
			fieldVariable := ast.Variable{
				Name:      f.Field.Name,
				IsPointer: f.Field.Type != nil && f.Field.Type.Pointer,
				IsConst:   f.Field.Type != nil && f.Field.Type.Const,
				Parent:    classAst,
			}
			classAst.Variables[f.Field.Name] = fieldVariable

			// Store type info for type checking (use generic param name as-is for generics)
			(*CProgramValues)[fieldVariable.GetFullName()] = &CValueInformation{
				CType:     "", // Not resolved yet for generics
				GeckoType: f.Field.Type,
			}
		}
	}

	// Register methods in class AST for type checking (needed for both generic and non-generic)
	for _, f := range c.Fields {
		if f.Method != nil {
			returnType := "void"
			if f.Method.Type != nil {
				returnType = f.Method.Type.Type
			}
			classAst.Methods[f.Method.Name] = &ast.Method{
				Name:       f.Method.Name,
				Type:       returnType,
				Visibility: f.Method.Visibility,
				Parent:     classAst,
			}
		}
	}

	// If this is a generic class, register it and skip C code generation
	if len(c.TypeParams) > 0 {
		originModule := scope.GetRoot().Scope
		Generics.RegisterGenericClass(c.Name, c, originModule)
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
			Scope:        name,
			Parent:       scope,
			OriginModule: scope.GetRoot().Scope,
		}
		classAst.Init(scope.ErrorScope)
		scope.Classes[name] = classAst
	}

	var structDef string
	dependencies := []string{}
	valueDependencies := []string{}

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
			var fieldDecl string
			var cTypeForTracking string

			// Handle fixed-size arrays specially: [N]T -> T name[N]
			if f.Field.Type != nil && f.Field.Type.Size != nil {
				elemType := TypeRefToCType(f.Field.Type.Size.Type, scope)
				if len(typeArgs) > 0 && len(c.TypeParams) > 0 {
					elemType = SubstituteTypeParams(elemType, c.TypeParams, typeArgs)
				}
				fieldDecl = "    " + elemType + " " + f.Field.Name + "[" + f.Field.Type.Size.Size + "];\n"
				cTypeForTracking = elemType + "[" + f.Field.Type.Size.Size + "]"
				// Track dependency on array element type
				if depType := extractDependencyType(elemType); depType != "" {
					dependencies = append(dependencies, depType)
				}
				// Arrays of values create value dependencies
				if valDepType := extractValueDependencyType(elemType); valDepType != "" {
					valueDependencies = append(valueDependencies, valDepType)
				}
			} else {
				fieldType := TypeRefToCType(f.Field.Type, scope)
				// Substitute type parameters if this is a generic instantiation
				if len(typeArgs) > 0 && len(c.TypeParams) > 0 {
					fieldType = SubstituteTypeParams(fieldType, c.TypeParams, typeArgs)
				}
				fieldDecl = "    " + fieldType + " " + f.Field.Name + ";\n"
				cTypeForTracking = fieldType
				// Track dependency on field type (including pointer types, since we use typedefs)
				if depType := extractDependencyType(fieldType); depType != "" {
					dependencies = append(dependencies, depType)
				}
				// Track value dependencies (non-pointer) for cycle detection
				if valDepType := extractValueDependencyType(fieldType); valDepType != "" {
					valueDependencies = append(valueDependencies, valDepType)
				}
			}
			structDef += fieldDecl

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
				CType:     cTypeForTracking,
				GeckoType: f.Field.Type,
			}
		}
	}

	structDef += "} " + name + ";\n"

	// Add to both Types (for backward compat) and StructDefs (for sorting)
	info.Types = append(info.Types, structDef)
	info.StructDefs = append(info.StructDefs, &StructDefinition{
		Name:              name,
		Code:              structDef,
		Dependencies:      dependencies,
		ValueDependencies: valueDependencies,
		Pos:               c.Pos,
	})
}

// extractDependencyType extracts a type name from a C type string that we depend on.
// Returns empty string for primitives. Strips pointer suffixes since typedefs need to exist.
func extractDependencyType(cType string) string {
	// Skip primitive C types
	primitives := map[string]bool{
		"void": true, "int": true, "int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
		"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
		"float": true, "double": true, "const char*": true, "char": true,
	}

	// Remove volatile qualifier
	cleanType := strings.TrimPrefix(cType, "volatile ")

	// For pointers, extract the base type - typedefs require the typedef to exist
	// even when used as a pointer (e.g., Rectangle* requires Rectangle typedef)
	for strings.HasSuffix(cleanType, "*") {
		cleanType = strings.TrimSuffix(cleanType, "*")
		cleanType = strings.TrimSpace(cleanType)
	}

	if primitives[cleanType] {
		return ""
	}

	return cleanType
}

// extractValueDependencyType extracts a type name only for VALUE (non-pointer) dependencies.
// Returns empty string for primitives and pointer types.
// Used for circular dependency detection - pointer cycles are allowed, value cycles are not.
func extractValueDependencyType(cType string) string {
	// Skip primitive C types
	primitives := map[string]bool{
		"void": true, "int": true, "int8_t": true, "int16_t": true, "int32_t": true, "int64_t": true,
		"uint8_t": true, "uint16_t": true, "uint32_t": true, "uint64_t": true,
		"float": true, "double": true, "const char*": true, "char": true,
	}

	// Remove volatile qualifier
	cleanType := strings.TrimPrefix(cType, "volatile ")

	// Pointers don't create value dependencies (they have fixed size)
	if strings.HasSuffix(cleanType, "*") {
		return ""
	}

	if primitives[cleanType] {
		return ""
	}

	return cleanType
}

// GenerateMethodDef generates a monomorphized method with type substitution
func (impl *CBackendImplementation) GenerateMethodDef(scope *ast.Ast, m *tokens.Method, name string, typeArgs []string) {
	info := CGetScopeInformation(scope)

	// Set up monomorph context early so type parameters in argument types are resolved
	oldContext := CurrentMonomorphContext
	if len(typeArgs) > 0 && len(m.TypeParams) > 0 {
		CurrentMonomorphContext = BuildMonomorphContext(m.TypeParams, typeArgs)
	}

	// Determine return type with substitution
	returnType := "void"
	if m.Type != nil {
		returnType = TypeRefToCType(m.Type, scope)
	}

	// Build parameter list with substitution
	params := []string{}
	for _, arg := range m.Arguments {
		paramType := TypeRefToCType(arg.Type, scope)
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
		Scope:        name,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
	}
	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Register arguments in scope
	for _, arg := range m.Arguments {
		paramType := TypeRefToCType(arg.Type, scope)
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
	mthInfo.CurrentFuncReturnType = m.Type // Track return type for validation
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		impl.Backend.ProcessEntries(m.Value, &methodScope)

		// Add implicit void return if needed
		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			impl.generateDropCalls(&methodScope, mthInfo)
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

	// Restore previous monomorph context
	CurrentMonomorphContext = oldContext
}

// GenerateClassMethodDef generates a method for a generic class instantiation
func (impl *CBackendImplementation) GenerateClassMethodDef(scope *ast.Ast, classToken *tokens.Class, m *tokens.Method, methodName string, className string, typeArgs []string) {
	info := CGetScopeInformation(scope.GetRoot())

	// Set up monomorph context FIRST so type substitution works for return type and params
	oldContext := CurrentMonomorphContext
	var typeParams []*tokens.TypeParam
	if classToken != nil {
		typeParams = classToken.TypeParams
	}
	CurrentMonomorphContext = BuildMonomorphContext(typeParams, typeArgs)

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
		Scope:        methodName,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
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
	mthInfo.CurrentFuncReturnType = m.Type // Track return type for validation
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		impl.Backend.ProcessEntries(m.Value, &methodScope)

		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			impl.generateDropCalls(&methodScope, mthInfo)
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
	callStr := impls.FuncCallToCString(f, scope) // Type checking happens inside
	info.Code += "    " + callStr + ";\n"
}

// MethodCall handles chained method calls like self.field.method()
func (impl *CBackendImplementation) MethodCall(scope *ast.Ast, m *tokens.MethodCall) {
	if m == nil || len(m.Chain) == 0 {
		return
	}

	// Build a Literal to reuse the chain processing logic
	lit := &tokens.Literal{
		Symbol: m.Base,
		Chain:  m.Chain,
	}

	info := CGetScopeInformation(scope)
	callStr := impl.LiteralToCString(lit, scope)
	info.Code += "    " + callStr + ";\n"
}

// NewImplementation handles implementations (trait impls, inherent impls, arch impls)
func (impl *CBackendImplementation) NewImplementation(scope *ast.Ast, i *tokens.Implementation) {
	if i.GetFor() != "" {
		// `impl Trait for Class` - trait implementation
		impl.CImplementationForClass(scope, i)
	} else {
		// Check if this is a generic impl (impl<T> ClassName<T>)
		typeParams := i.GetTypeParams()
		className := i.GetName()

		// Check if the target class is a registered generic class
		if Generics.IsGenericClass(className) {
			// Store the impl with the generic class for later instantiation
			classToken := Generics.GenericClasses[className]
			if classToken != nil {
				classToken.Implementations = append(classToken.Implementations, i)
			}
			return
		}

		// Check if i.GetName() is a class (inherent impl) or an arch (arch-specific impl)
		classOpt := scope.ResolveClass(className)
		if !classOpt.IsNil() {
			// If this impl has type params but class is not generic, that's an error
			if len(typeParams) > 0 {
				scope.ErrorScope.NewCompileTimeError(
					"Implementation Error",
					"Cannot use type parameters in impl block for non-generic class '"+className+"'",
					i.Pos,
				)
				return
			}
			// `impl ClassName` - inherent implementation (methods on the class itself)
			impl.CInherentImplementation(scope, i, classOpt.Unwrap())
		} else {
			// Assume it's an arch-specific implementation
			impl.CImplementationForArch(scope, i)
		}
	}
}

// NewEnum handles enum definitions - generates C enum for FFI
func (impl *CBackendImplementation) NewEnum(scope *ast.Ast, e *tokens.Enum) {
	info := CGetScopeInformation(scope)

	// Generate C enum typedef
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("typedef enum {\n"))
	for i, caseName := range e.Cases {
		mangledCase := scope.GetRoot().Scope + "__" + e.Name + "_" + caseName
		sb.WriteString(fmt.Sprintf("    %s = %d", mangledCase, i))
		if i < len(e.Cases)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	mangledName := scope.GetRoot().Scope + "__" + e.Name
	sb.WriteString(fmt.Sprintf("} %s;", mangledName))

	info.Types = append(info.Types, sb.String())

	// Register enum as a type in AST (as a pseudo-class for type checking)
	enumAst := &ast.Ast{
		Scope:        e.Name,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
	}
	enumAst.Init(scope.ErrorScope)

	// Store enum cases as constants for potential future use
	for _, caseName := range e.Cases {
		enumAst.Variables[caseName] = ast.Variable{
			Name:    caseName,
			IsConst: true,
			Parent:  enumAst,
		}
	}

	scope.Classes[e.Name] = enumAst

	// Register in EnumToCType for type conversion (not GeckoToCType to avoid
	// loadPrimitives overwriting the enum AST)
	EnumToCType[e.Name] = mangledName
}

// NewTrait handles trait definitions
func (impl *CBackendImplementation) NewTrait(scope *ast.Ast, t *tokens.Trait) {
	// Store trait token for default implementations
	TraitDefinitions[t.Name] = t

	// Store trait methods for later implementation
	mthds := []*ast.Method{}
	for _, f := range t.Fields {
		m := f.ToMethodToken()
		// Store Gecko type for type checking
		geckoReturnType := "void"
		if m.Type != nil {
			geckoReturnType = m.Type.Type
		}
		astMth := &ast.Method{
			Name:       m.Name,
			Scope:      nil,
			Arguments:  make([]ast.Variable, 0),
			Visibility: m.Visibility,
			Parent:     scope,
			Type:       geckoReturnType,
		}
		mthds = append(mthds, astMth)
	}
	scope.Traits[t.Name] = &mthds
}

// NewTraitMethod generates a C function for a trait implementation
// It handles the self parameter and uses the mangled name
func (impl *CBackendImplementation) NewTraitMethod(scope *ast.Ast, classScope *ast.Ast, m *tokens.Method, mangledName string, className string) {
	methodScope := ast.Ast{
		Scope:        mangledName,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
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

		// For self parameter, set the Gecko type to the class type (as pointer)
		geckoType := arg.Type
		if paramName == "self" {
			geckoType = &tokens.TypeRef{Type: className, Pointer: true}
		}

		(*CProgramValues)[argVariable.GetFullName()] = &CValueInformation{
			CType:     paramType,
			GeckoType: geckoType,
		}
	}

	paramStr := strings.Join(params, ", ")

	// Create AST method with mangled name
	// Mark as external so GetFullName returns just the mangled name
	// Store the Gecko type (not C type) for type checking
	geckoReturnType := "void"
	if m.Type != nil {
		geckoReturnType = m.Type.Type
	}
	astMth := &ast.Method{
		Name:       mangledName,
		Scope:      &methodScope,
		Arguments:  make([]ast.Variable, 0),
		Visibility: "external",
		Parent:     scope,
		Type:       geckoReturnType,
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
	mthInfo.CurrentFuncReturnType = m.Type // Track return type for validation
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		impl.Backend.ProcessEntries(m.Value, &methodScope)

		if returnType == "void" && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			impl.generateDropCalls(&methodScope, mthInfo)
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function signature
	funcDecl := fmt.Sprintf("%s %s(%s)", returnType, mangledName, paramStr)

	// Add to parent scope info
	parentInfo := CGetScopeInformation(scope)

	// Always generate function definition for trait methods (empty body is valid for no-op)
	if len(m.Value) > 0 {
		funcDef := funcDecl + " {\n" + mthInfo.Code + "}\n"
		parentInfo.Functions = append(parentInfo.Functions, funcDef)
	} else {
		// Generate empty function body (void return)
		funcDef := funcDecl + " {\n    return;\n}\n"
		parentInfo.Functions = append(parentInfo.Functions, funcDef)
	}

	// Add forward declaration for trait methods
	parentInfo.Declarations = append(parentInfo.Declarations, funcDecl+";")
}

// NewMethod generates a C function
func (impl *CBackendImplementation) NewMethod(scope *ast.Ast, m *tokens.Method) {
	// Register method signature for type checking (with both full and short names)
	fullName := scope.GetFullName() + "__" + m.Name
	RegisterMethodSignature(fullName, m)
	RegisterMethodSignature(m.Name, m) // Also register with short name for local lookups

	// If this is a generic method, register it and skip code generation
	if len(m.TypeParams) > 0 {
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
		Scope:        m.Name,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
	}

	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Determine return type
	returnType := "void"
	geckoReturnType := "void"
	if m.Type != nil {
		m.Type.Check(scope)
		returnType = TypeRefToCType(m.Type, scope)
		geckoReturnType = m.Type.Type
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
		Type:       geckoReturnType,
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
	mthInfo.CurrentFuncReturnType = m.Type // Track return type for validation
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
			impl.generateDropCalls(&methodScope, mthInfo)
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

	// Track whether type was explicit or inferred
	typeWasExplicit := f.Type != nil

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

			// If tokens.InferType returns nil, try backend's type inference
			// This handles function calls, method calls, etc.
			if f.Type == nil {
				f.Type = impl.GetTypeOfExpression(f.Value, scope)
			}
		}

		// If still nil, error
		if f.Type == nil {
			scope.ErrorScope.NewCompileTimeError(
				"Type Inference Error",
				"Unable to infer variable type; please provide an explicit type annotation",
				f.Pos,
			)
			f.Type = &tokens.TypeRef{Type: "int"}
		}
	}

	// Check type visibility for explicit type annotations
	// (Inferred types already passed visibility when they were originally defined)
	if typeWasExplicit && f.Type != nil {
		f.Type.Check(scope)
	}

	// Check for const - either from Type.Const or from Mutability == "const"
	isConst := (f.Type != nil && f.Type.Const) || f.Mutability == "const"

	if f.Value == nil && isConst {
		scope.ErrorScope.NewCompileTimeError("Uninitialized Constant", "Constant must be initialized with a value", f.Pos)
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
		IsGlobal:   isGlobal,
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
	classOpt := scope.ResolveClass(i.GetFor())
	traitOpt := scope.ResolveTrait(i.GetName())

	if classOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the class '"+i.GetFor()+"'", i.Pos)
		return
	}

	if traitOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the trait '"+i.GetName()+"'", i.Pos)
		return
	}

	class := classOpt.Unwrap()
	_ = traitOpt.Unwrap() // Validate trait exists (methods come from TraitDefinitions for default impls)
	className := i.GetFor()
	traitName := i.GetName()

	// Build mangled trait name with type arguments (e.g., "Add__Point" for Add<Point>)
	mangledTraitName := traitName
	if len(i.GetTypeArgs()) > 0 {
		for _, typeArg := range i.GetTypeArgs() {
			mangledTraitName += "__" + TypeRefToCType(typeArg, scope)
		}
	}

	if i.GetFields() != nil && i.Default {
		scope.ErrorScope.NewCompileTimeError(
			"Implementation Error",
			"A default trait implementation must not have a body",
			i.Pos,
		)
		return
	}

	var mthdList []*ast.Method

	if i.Default {
		// Default impl: validate required methods and compile default bodies
		traitToken, ok := TraitDefinitions[i.GetName()]
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Implementation Error",
				"Could not find trait definition for '"+i.GetName()+"'",
				i.Pos,
			)
			return
		}

		// Build type substitution map for generic traits
		var typeSubstitutions map[string]*tokens.TypeRef
		if len(traitToken.TypeParams) > 0 && len(i.GetTypeArgs()) > 0 {
			typeSubstitutions = make(map[string]*tokens.TypeRef)
			for idx, param := range traitToken.TypeParams {
				if idx < len(i.GetTypeArgs()) {
					typeSubstitutions[param.Name] = i.GetTypeArgs()[idx]
				}
			}
		}

		for _, f := range traitToken.Fields {
			methodName := f.Name
			isRequired := f.Value == nil || len(f.Value) == 0

			if isRequired {
				// Required method: check class has it
				if _, exists := class.Methods[methodName]; !exists {
					// Build expected signature for error message
					argTypes := []string{}
					for _, arg := range f.Arguments {
						if arg.Name == "self" {
							argTypes = append(argTypes, "self")
							continue
						}
						argType := "?"
						if arg.Type != nil {
							argType = arg.Type.Type
						}
						argTypes = append(argTypes, arg.Name+": "+argType)
					}
					retType := "void"
					if f.Type != nil {
						retType = f.Type.Type
					}
					signature := methodName + "(" + strings.Join(argTypes, ", ") + "): " + retType

					scope.ErrorScope.NewCompileTimeError(
						"Missing Required Method",
						"Class '"+className+"' cannot implement trait '"+i.GetName()+"': missing required method '"+signature+"'",
						i.Pos,
					)
					return
				}
				// Use the class's existing method
				if astMth, exists := class.Methods[methodName]; exists {
					mthdList = append(mthdList, astMth)
				}
			} else {
				// Default method: compile with class's self type
				m := f.ToMethodToken()

				// Apply type substitutions for generic traits
				if typeSubstitutions != nil {
					m.Type = substituteTypeRef(m.Type, typeSubstitutions)
					for _, arg := range m.Arguments {
						if arg.Type != nil {
							arg.Type = substituteTypeRef(arg.Type, typeSubstitutions)
						}
					}
				}

				mangledName := className + "__" + mangledTraitName + "__" + m.Name
				impl.NewTraitMethod(scope, class, m, mangledName, className)
				if astMth, ok := scope.Methods[mangledName]; ok {
					mthdList = append(mthdList, astMth)
				}
			}
		}
	} else {
		for _, f := range i.GetFields() {
			m := f.ToMethodToken()
			mangledName := className + "__" + mangledTraitName + "__" + m.Name
			impl.NewTraitMethod(scope, class, m, mangledName, className)
			if astMth, ok := scope.Methods[mangledName]; ok {
				mthdList = append(mthdList, astMth)
			}
		}
	}

	// Check for trait method conflicts before registering
	// A class implementing multiple traits with the same method name is an error
	for _, newMethod := range mthdList {
		// Extract just the method name from the mangled name (e.g., "Point__Add__add" -> "add")
		methodBaseName := newMethod.Name
		if idx := strings.LastIndex(newMethod.Name, "__"); idx >= 0 {
			methodBaseName = newMethod.Name[idx+2:]
		}

		for existingTraitName, existingMethods := range class.Traits {
			if existingTraitName == mangledTraitName || existingMethods == nil {
				continue
			}
			for _, existingMethod := range *existingMethods {
				existingBaseName := existingMethod.Name
				if idx := strings.LastIndex(existingMethod.Name, "__"); idx >= 0 {
					existingBaseName = existingMethod.Name[idx+2:]
				}
				if methodBaseName == existingBaseName {
					scope.ErrorScope.NewCompileTimeError(
						"Trait Method Conflict",
						fmt.Sprintf("Method '%s' is defined in both trait '%s' and trait '%s' on class '%s'",
							methodBaseName, existingTraitName, traitName, className),
						i.Pos,
					)
					return
				}
			}
		}
	}

	class.Traits[mangledTraitName] = &mthdList
}

// CInherentImplementation handles `impl ClassName { ... }` - methods directly on a class
// Extensions can only ADD methods, not replace existing ones (Swift-style)
func (impl *CBackendImplementation) CInherentImplementation(scope *ast.Ast, i *tokens.Implementation, class *ast.Ast) {
	className := i.GetName()
	fullClassName := class.GetFullName() // e.g., "main__Point"

	for _, f := range i.GetFields() {
		m := f.ToMethodToken()

		// Check if method already exists - extensions cannot override
		if _, exists := class.Methods[m.Name]; exists {
			scope.ErrorScope.NewCompileTimeError(
				"Duplicate Method",
				"Method '"+m.Name+"' already exists on class '"+className+"'. Extensions can only add new methods, not override existing ones.",
				m.Pos,
			)
			continue
		}

		// Generate the method with full scope: main__Point__new
		mangledName := fullClassName + "__" + m.Name
		impl.GenerateClassMethodDef(scope, nil, m, mangledName, className, nil)

		// Register the method in the class scope for resolution
		geckoReturnType := "void"
		if m.Type != nil {
			geckoReturnType = m.Type.Type
		}
		astMth := &ast.Method{
			Name:       m.Name,
			Scope:      nil,
			Arguments:  make([]ast.Variable, 0),
			Visibility: m.Visibility,
			Parent:     class,
			Type:       geckoReturnType,
		}
		class.Methods[m.Name] = astMth
	}
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

	if scope.Config.Arch == i.GetName() {
		for _, f := range i.GetFields() {
			m := f.ToMethodToken()
			impl.NewMethod(scope, m)
		}
	} else {
		scope.ErrorScope.NewCompileTimeWarning(
			"Arch Implementation",
			"Implementation for the arch '"+i.GetName()+"' was skipped due to target being '"+scope.Config.Arch+"'",
			i.Pos,
		)
	}
}

// NewIf handles if statements (basic implementation for now)
func (impl *CBackendImplementation) NewIf(scope *ast.Ast, i *tokens.If) {
	info := CGetScopeInformation(scope)

	condition := impl.ExpressionToCString(i.Expression, scope)
	info.Code += fmt.Sprintf("    if (%s) {\n", condition)

	// Detect null check patterns for type narrowing
	nullCheck := DetectNullCheck(i.Expression)
	var savedTypeState *ast.TypeState

	if nullCheck != nil {
		checkType := "== nil"
		if nullCheck.IsNotNull {
			checkType = "!= nil"
		}
		narrowingDebug(fmt.Sprintf("Detected null check in if condition: %s %s", nullCheck.VarName, checkType))

		// Save current type state and create a forked state for the if-body
		savedTypeState = CurrentTypeState
		if savedTypeState != nil {
			CurrentTypeState = savedTypeState.Fork(1) // Path 1 = then branch
		} else {
			CurrentTypeState = ast.NewTypeState()
		}

		// Apply narrowing: if condition is "x != nil", mark x as non-null in the if-body
		ApplyNullNarrowing(CurrentTypeState, nullCheck, scope)
	}

	// Process if body with the narrowed type state
	for _, entry := range i.Value {
		impl.processEntry(scope, entry)
	}

	// Restore the original type state after processing the if-body
	if nullCheck != nil {
		CurrentTypeState = savedTypeState
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
		// for-in loop: for x in iterator { ... }
		// Desugars to: while (iter.has_next()) { let x = iter.next(); ... }
		impl.generateForInLoop(scope, l)
		return
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

// generateForInLoop handles for-in loop iteration using the Iterator trait
// for x in collection { ... } desugars to:
//   {
//       CollType __iter = collection;
//       while (CollType__Iterator__has_next(&__iter)) {
//           ElemType x = CollType__Iterator__next(&__iter);
//           ...
//       }
//   }
func (impl *CBackendImplementation) generateForInLoop(scope *ast.Ast, l *tokens.Loop) {
	info := CGetScopeInformation(scope)
	forIn := l.ForIn

	// Get the loop variable name and optional type
	varName := forIn.Variable.Name
	varType := forIn.Variable.Type

	// Get the iterator expression
	iterExpr := impl.ExpressionToCString(forIn.SourceArray, scope)

	// Infer the iterator type using CProgramValues for type lookup
	resolveSymbol := func(name string) *tokens.TypeRef {
		varOpt := scope.ResolveSymbolAsVariable(name)
		if !varOpt.IsNil() {
			v := varOpt.Unwrap()
			// Look up the type in CProgramValues
			if valInfo, ok := (*CProgramValues)[v.GetFullName()]; ok && valInfo.GeckoType != nil {
				return valInfo.GeckoType
			}
			// Fallback: construct TypeRef from variable info
			return &tokens.TypeRef{Pointer: v.IsPointer}
		}
		return nil
	}
	iterType := tokens.InferType(forIn.SourceArray, resolveSymbol)

	if iterType == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Type Inference Error",
			"Cannot infer type of iterator expression in for-in loop",
			forIn.Pos,
		)
		return
	}

	// Get the iterator type name (strip pointer if present)
	iterTypeName := iterType.Type
	if iterType.Pointer {
		iterTypeName = strings.TrimSuffix(iterTypeName, "*")
	}

	// Look up the class to find Iterator trait implementation
	classOpt := scope.ResolveClass(iterTypeName)
	if classOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError(
			"Iterator Error",
			"Type '"+iterTypeName+"' is not a class that can be iterated",
			forIn.Pos,
		)
		return
	}
	class := classOpt.Unwrap()

	// Find Iterator trait - look for trait names starting with "Iterator"
	var iteratorTraitName string
	var elementType string
	for traitName := range class.Traits {
		if strings.HasPrefix(traitName, "Iterator") {
			iteratorTraitName = traitName
			// Extract element type from Iterator<T>
			if strings.Contains(traitName, "__") {
				// Generic instantiation like Iterator__int32
				parts := strings.SplitN(traitName, "__", 2)
				if len(parts) == 2 {
					elementType = parts[1]
				}
			}
			break
		}
	}

	if iteratorTraitName == "" {
		scope.ErrorScope.NewCompileTimeError(
			"Iterator Error",
			"Type '"+iterTypeName+"' does not implement Iterator trait",
			forIn.Pos,
		)
		return
	}

	// Get the iterator hook to find method names
	iterHook := hooks.GetHookRegistry().GetHook(scope.GetRoot().Scope, hooks.HookIterator)
	hasNextMethod := "has_next"
	nextMethod := "next"
	if iterHook != nil && len(iterHook.Methods) >= 2 {
		// Hook should define [next, has_next] methods
		nextMethod = iterHook.Methods[0]
		hasNextMethod = iterHook.Methods[1]
	}

	// Determine element type for the loop variable
	var elemCType string
	if varType != nil {
		// Explicit type annotation on loop variable
		elemCType = TypeRefToCType(varType, scope)
	} else if elementType != "" {
		// Inferred from Iterator<T>
		if cType, ok := GeckoToCType[elementType]; ok {
			elemCType = cType
		} else {
			elemCType = elementType
		}
	} else {
		// Fallback - try to get from trait methods
		if traitMethods, ok := class.Traits[iteratorTraitName]; ok && traitMethods != nil {
			for _, m := range *traitMethods {
				if m.Name == nextMethod && m.Type != "" {
					if cType, ok := GeckoToCType[m.Type]; ok {
						elemCType = cType
					} else {
						elemCType = m.Type
					}
					break
				}
			}
		}
	}

	if elemCType == "" {
		elemCType = "int" // Fallback
	}

	// Get C type for iterator
	var iterCType string
	if cType, ok := GeckoToCType[iterTypeName]; ok {
		iterCType = cType
	} else {
		iterCType = iterTypeName
	}

	// Use bare class name for method calls (matches trait implementation naming)
	// Trait methods are mangled as: ClassName__TraitName__MethodName
	classBaseName := iterTypeName

	// Generate mangled method names
	hasNextMangled := classBaseName + "__" + iteratorTraitName + "__" + hasNextMethod
	nextMangled := classBaseName + "__" + iteratorTraitName + "__" + nextMethod

	// Generate the loop structure
	info.Code += "    {\n"

	// Copy iterator to local variable (for mutable state)
	info.Code += fmt.Sprintf("        %s __iter = %s;\n", iterCType, iterExpr)

	// Generate while loop with has_next condition
	info.Code += fmt.Sprintf("        while (%s(&__iter)) {\n", hasNextMangled)

	// Declare loop variable with value from next()
	info.Code += fmt.Sprintf("            %s %s = %s(&__iter);\n", elemCType, varName, nextMangled)

	// Create a child scope for the loop body to register the loop variable
	loopScope := &ast.Ast{
		Scope:  scope.Scope + "__for_in",
		Parent: scope,
	}
	loopScope.Init(scope.ErrorScope)
	loopScope.Config = scope.Config

	// Register the loop variable in the loop scope
	loopScope.Variables[varName] = ast.Variable{
		Name:   varName,
		Parent: loopScope,
	}

	// Store element type info in CProgramValues
	(*CProgramValues)[varName] = &CValueInformation{
		CType:     elemCType,
		GeckoType: &tokens.TypeRef{Type: elementType},
	}

	// Initialize scope info for the loop scope using CGetScopeInformation
	CGetScopeInformation(loopScope)

	// Process loop body entries in the loop scope
	for _, entry := range l.Value {
		impl.processEntry(loopScope, entry)
	}

	// Append the loop body code
	loopInfo := CGetScopeInformation(loopScope)
	info.Code += loopInfo.Code

	info.Code += "        }\n"
	info.Code += "    }\n"
}

// NewAssignment handles variable assignment
func (impl *CBackendImplementation) NewAssignment(scope *ast.Ast, a *tokens.Assignment) {
	// Type check the assignment
	impl.CheckAssignmentType(a, scope)

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

// NewCImport handles a C import statement, adding #include directive
func (impl *CBackendImplementation) NewCImport(scope *ast.Ast, cimport *tokens.CImport) {
	info := CGetScopeInformation(scope)

	// Add the include directive
	header := cimport.Header

	// Strip outer quotes from the parser (String token includes quotes)
	if len(header) >= 2 && header[0] == '"' && header[len(header)-1] == '"' {
		header = header[1 : len(header)-1]
	}

	// Check if it's a system header (angle brackets) or local header
	if len(header) > 0 && header[0] == '<' {
		// System header: <stdio.h> -> #include <stdio.h>
		info.Includes = append(info.Includes, "#include "+header)
	} else {
		// Local header: myheader.h -> #include "myheader.h"
		info.Includes = append(info.Includes, "#include \""+header+"\"")
	}

	// Note: WithObject and WithLibrary are handled at link time by the build command
	// They are stored in the token and can be retrieved by the compiler orchestrator
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
	} else if entry.MethodCall != nil {
		impl.MethodCall(scope, entry.MethodCall)
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
