// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// NewMethod generates a C function
func (impl *CBackendImplementation) NewMethod(scope *ast.Ast, m *tokens.Method) {
	if reportOutParamsOnlyForExternal(scope, m) {
		return
	}

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
			Unsafe:     tokens.HasAttribute(m.Attributes, "unsafe"),
		}
		scope.Methods[m.Name] = astMth
		Methods[scope.FullScopeName()+"#"+m.Name] = astMth
		return
	}

	methodScope := ast.Ast{
		Scope:        m.Name,
		Parent:       scope,
		OriginModule: scope.GetRoot().Scope,
		SourceFile:   scope.GetSourceFile(),
	}

	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

	// Check if this is a class method (scope is a class AST)
	isClassMethod := false
	className := ""
	if scope.Parent != nil {
		if _, ok := scope.Parent.Classes[scope.Scope]; ok {
			isClassMethod = true
			className = scope.Scope
		}
	}

	// Set Self type for resolving Self in method signatures
	oldSelfType := CurrentSelfType
	if className != "" {
		CurrentSelfType = className
	}
	defer func() {
		CurrentSelfType = oldSelfType
	}()

	// Determine return type
	returnType := "void"
	geckoReturnType := "void"
	if m.Type != nil {
		m.Type.Check(scope)
		returnType = TypeRefToCType(m.Type, scope)
		geckoReturnType = m.Type.Type
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
			// Validate parameter type before conversion
			if arg.Type != nil {
				arg.Type.Check(scope)
			}
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
			IsConst:    argBindingIsConst(arg.Type),
			IsVolatile: arg.Type != nil && arg.Type.Volatile,
			IsArgument: true,
			IsReceiver: paramName == "self" && isClassMethod,
			Parent:     &methodScope,
		}
		methodScope.Variables[arg.Name] = argVariable

		(*CProgramValues)[argVariable.GetFullName()] = &CValueInformation{
			CType:     paramType,
			GeckoType: geckoType,
		}
	}

	paramStr := strings.Join(params, ", ")
	if m.IsVariadic() {
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
		Unsafe:     tokens.HasAttribute(m.Attributes, "unsafe"),
	}

	if len(m.Value) == 0 {
		// Declaration only, no body
		astMth.Scope = nil
	}

	scope.Methods[m.Name] = astMth
	Methods[scope.FullScopeName()+"#"+m.Name] = astMth

	// Track full return type for generic type support
	if m.Type != nil {
		MethodReturnTypes[scope.FullScopeName()+"#"+m.Name] = m.Type
	}

	// Initialize method scope info
	mthInfo := &CScopeInformation{}
	mthInfo.Init()
	mthInfo.FunctionBoundary = true
	mthInfo.CurrentFunc = m.Name
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

		// If std.errors is imported, install its hidden try diagnostics hook at process startup.
		if m.Visibility == "external" && m.Name == "main" {
			if installer := findStdTryDiagnosticsInstaller(scope.GetRoot()); installer != "" {
				mthInfo.Code = "    " + installer + "();\n" + mthInfo.Code
			}
		}

		// Check if we need to add implicit void return
		// Don't add return for @naked or @noreturn functions
		isNaked := tokens.HasAttribute(m.Attributes, "naked")
		isNoReturn := tokens.HasAttribute(m.Attributes, "noreturn")
		if returnType == "void" && !isNaked && !isNoReturn && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			impl.emitDefers(&methodScope, mthInfo)
			impl.generateDropCalls(&methodScope, mthInfo)
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function signature with attributes
	funcName := astMth.GetFullName()
	if m.Type != nil {
		// Keep emitted C symbol aliases stable after body generation as well.
		MethodReturnTypes[funcName] = m.Type
		MethodReturnTypes[scope.FullScopeName()+"#"+funcName] = m.Type
	}
	attrStr := tokens.ToCAttributes(m.Attributes)
	isStatic := tokens.HasAttribute(m.Attributes, "static")
	var funcDecl string
	// Handle function pointer return types: generate correct C syntax
	// For func(int32): func(int32): int32, the C signature is:
	// int32_t (*funcname(int32_t))(int32_t)
	if IsFuncPointerType(returnType) {
		// Extract the inner return type and params from the function pointer type
		// returnType is like "int32_t (*__FUNCPTR__)(int32_t)"
		// We need: "int32_t (*funcname(params))(int32_t_inner)"
		// Parse: retType (*__FUNCPTR__)(params)
		innerRetType := returnType
		innerParams := ""
		if idx := strings.Index(returnType, "(*"); idx >= 0 {
			innerRetType = returnType[:idx]
			rest := returnType[idx:]
			if endIdx := strings.Index(rest, ")("); endIdx >= 0 {
				innerParams = rest[endIdx+2:]
				innerParams = strings.TrimSuffix(innerParams, ")")
			}
		}
		// Generate: innerRetType (*funcname(innerParams))(originalParams)
		if attrStr != "" {
			funcDecl = fmt.Sprintf("%s %s (*%s(%s))(%s)", attrStr, strings.TrimSpace(innerRetType), funcName, paramStr, innerParams)
		} else {
			funcDecl = fmt.Sprintf("%s (*%s(%s))(%s)", strings.TrimSpace(innerRetType), funcName, paramStr, innerParams)
		}
	} else {
		if attrStr != "" {
			funcDecl = fmt.Sprintf("%s %s %s(%s)", attrStr, returnType, funcName, paramStr)
		} else {
			funcDecl = fmt.Sprintf("%s %s(%s)", returnType, funcName, paramStr)
		}
	}
	if isStatic {
		funcDecl = "static " + funcDecl
	}

	// Add to root scope info (so class methods end up in the output)
	parentInfo := CGetScopeInformation(scope.GetRoot())

	if len(m.Value) > 0 {
		// Full function definition
		funcDef := funcDecl + " {\n" + mthInfo.Code + "}\n"
		parentInfo.Functions = append(parentInfo.Functions, funcDef)

		// Treeshake roots: keep user-declared external Gecko function definitions.
		if m.Visibility == "external" {
			parentInfo.ExternalRootSymbols = append(parentInfo.ExternalRootSymbols, funcName)
		}
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
