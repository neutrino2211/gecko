// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// GenerateMethodDef generates a monomorphized method with type substitution
func (impl *CBackendImplementation) GenerateMethodDef(scope *ast.Ast, m *tokens.Method, name string, typeArgs []string) {
	if reportOutParamsOnlyForExternal(scope, m) {
		return
	}

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
		SourceFile:   scope.GetSourceFile(),
	}
	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Register arguments in scope
	for _, arg := range m.Arguments {
		paramType := TypeRefToCType(arg.Type, scope)
		argVariable := ast.Variable{
			Name:       arg.Name,
			IsPointer:  arg.Type != nil && arg.Type.Pointer,
			IsConst:    argBindingIsConst(arg.Type),
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
	mthInfo.FunctionBoundary = true
	mthInfo.CurrentFunc = name
	mthInfo.CurrentFuncReturnType = m.Type // Track return type for validation
	markMethodScopeUnsafe(m, mthInfo)
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo
	if len(m.Value) > 0 {
		registerOwnedParameters(&methodScope, mthInfo, m.Arguments)
	}

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		prevTypeState := CurrentTypeState
		CurrentTypeState = mthInfo.TypeState
		impl.Backend.ProcessEntries(m.Value, &methodScope)
		CurrentTypeState = prevTypeState

		// Add implicit void return if needed
		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			impl.emitDefers(&methodScope, mthInfo)
			impl.generateDropCalls(&methodScope, mthInfo)
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function with attributes
	attrStr := tokens.ToCAttributes(m.Attributes)
	isStatic := tokens.HasAttribute(m.Attributes, "static")
	var funcDecl string
	if attrStr != "" {
		funcDecl = fmt.Sprintf("%s %s %s(%s)", attrStr, returnType, name, paramStr)
	} else {
		funcDecl = fmt.Sprintf("%s %s(%s)", returnType, name, paramStr)
	}
	if isStatic {
		funcDecl = "static " + funcDecl
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
	if reportOutParamsOnlyForExternal(scope, m) {
		return
	}

	info := CGetScopeInformation(scope.GetRoot())
	RegisterMethodSignature(methodName, m)
	RegisterMethodSignature(className+"__"+m.Name, m)

	// Set up monomorph context FIRST so type substitution works for return type and params
	oldContext := CurrentMonomorphContext
	var typeParams []*tokens.TypeParam
	if classToken != nil {
		typeParams = classToken.TypeParams
	}
	CurrentMonomorphContext = BuildMonomorphContext(typeParams, typeArgs)
	oldIsTypeParameter := tokens.IsTypeParameter
	classTypeParams := map[string]bool{}
	if classToken != nil {
		classTypeParams["Self"] = true          // Allow Self as a type alias for the current class
		classTypeParams[classToken.Name] = true // Allow self-references like Option<T> inside generic class methods
		for _, tp := range classToken.TypeParams {
			classTypeParams[tp.Name] = true
		}
	}
	tokens.IsTypeParameter = func(name string) bool {
		if classTypeParams[name] {
			return true
		}
		return oldIsTypeParameter(name)
	}
	defer func() {
		tokens.IsTypeParameter = oldIsTypeParameter
	}()

	// Set Self type for resolving Self in method signatures
	oldSelfType := CurrentSelfType
	if className != "" {
		CurrentSelfType = className
	}
	defer func() {
		CurrentSelfType = oldSelfType
	}()

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
		SourceFile:   scope.GetSourceFile(),
	}
	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config
	if classToken != nil {
		if definition := Generics.GenericClassScopes[classToken.Name]; definition != nil {
			for name, class := range definition.Classes {
				methodScope.Classes[name] = class
			}
			for name, method := range definition.Methods {
				methodScope.Methods[name] = method
			}
		}
	}

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
			IsReceiver: paramName == "self",
			IsPointer:  paramName == "self" || (arg.Type != nil && arg.Type.Pointer),
			IsConst:    argBindingIsConst(arg.Type),
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
		Unsafe: tokens.HasAttribute(m.Attributes, "unsafe"),
	}
	// Register under the instantiated class
	classAst, ok := scope.Classes[className]
	if ok {
		classAst.Methods[m.Name] = astMth
	}

	// Initialize method scope info
	mthInfo := &CScopeInformation{}
	mthInfo.Init()
	mthInfo.FunctionBoundary = true
	mthInfo.CurrentFunc = methodName
	mthInfo.CurrentFuncReturnType = m.Type // Track return type for validation
	markMethodScopeUnsafe(m, mthInfo)
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo
	if len(m.Value) > 0 {
		registerOwnedParameters(&methodScope, mthInfo, m.Arguments)
	}

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		prevTypeState := CurrentTypeState
		CurrentTypeState = mthInfo.TypeState
		impl.Backend.ProcessEntries(m.Value, &methodScope)
		CurrentTypeState = prevTypeState

		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			impl.emitDefers(&methodScope, mthInfo)
			impl.generateDropCalls(&methodScope, mthInfo)
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function
	attrStr := tokens.ToCAttributes(m.Attributes)
	isStatic := tokens.HasAttribute(m.Attributes, "static")
	var funcDecl string
	if attrStr != "" {
		funcDecl = fmt.Sprintf("%s %s %s(%s)", attrStr, returnType, methodName, paramStr)
	} else {
		funcDecl = fmt.Sprintf("%s %s(%s)", returnType, methodName, paramStr)
	}
	if isStatic {
		funcDecl = "static " + funcDecl
	}

	info.Declarations = append(info.Declarations, funcDecl+";")

	if len(m.Value) > 0 {
		funcDef := funcDecl + " {\n" + mthInfo.Code + "}\n"
		info.Functions = append(info.Functions, funcDef)
	}

	// Restore previous monomorph context
	CurrentMonomorphContext = oldContext
}
