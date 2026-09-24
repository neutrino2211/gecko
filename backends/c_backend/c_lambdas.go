// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// InferLambdaTypes fills in missing param types and return type on a Lambda
// from an expected FuncType (e.g., from a variable's type annotation).
func InferLambdaTypes(l *tokens.Lambda, ft *tokens.FuncType) {
	if l == nil || ft == nil {
		return
	}
	for i, param := range l.Params {
		if param.Type == nil && i < len(ft.ParamTypes) {
			param.Type = ft.ParamTypes[i]
		}
	}
	if l.ReturnType == nil && ft.ReturnType != nil {
		l.ReturnType = ft.ReturnType
	}
}

// LambdaToCString generates a static C function for a lambda expression and returns its name
func (impl *CBackendImplementation) LambdaToCString(l *tokens.Lambda, scope *ast.Ast) string {
	info := CGetScopeInformation(scope)

	// Generate unique function name
	lambdaCount := len(info.Functions)
	funcName := fmt.Sprintf("__lambda_%d_%d", lambdaCount, l.Pos.Line)

	// Detect captured variables
	capturedFields := collectCapturedVariables(l, scope)

	// Build parameter list
	params := ""
	for i, param := range l.Params {
		if i > 0 {
			params += ", "
		}
		paramType := "void*"
		if param.Type != nil {
			paramType = TypeRefToCType(param.Type, scope)
		}
		paramName := param.Name
		if paramName == "" {
			paramName = fmt.Sprintf("p%d", i)
		}
		params += paramType + " " + paramName
	}
	if params == "" {
		params = "void"
	}

	// Determine return type
	retType := "void"
	if l.ReturnType != nil {
		retType = TypeRefToCType(l.ReturnType, scope)
	}

	// If we have captures, generate a global capture slot
	var capContext *ClosureCaptureContext
	var captureGlobalName string
	if len(capturedFields) > 0 {
		structName, structDef, _ := generateCaptureStruct(capturedFields, scope)
		captureGlobalName = fmt.Sprintf("__%s_global", structName)

		// Add struct definition to root scope
		rootScope := scope.GetRoot()
		rootInfo := CGetScopeInformation(rootScope)
		rootInfo.StructDefs = append(rootInfo.StructDefs, &StructDefinition{
			Name: structName,
			Code: structDef,
		})

		// Add global capture slot
		rootInfo.Globals = append(rootInfo.Globals,
			fmt.Sprintf("static struct %s %s;\n", structName, captureGlobalName))

		// Create capture context for the lambda body
		capContext = &ClosureCaptureContext{
			StructName: structName,
			ParamName:  captureGlobalName,
			Fields:     capturedFields,
			FieldMap:   make(map[string]*ClosureCaptureField),
			GlobalSlot: captureGlobalName,
			OuterScope: scope,
		}
		for _, field := range capturedFields {
			capContext.FieldMap[field.FullName] = field
		}

		// Generate capture initialization code to be emitted at creation site
		initCode := generateCaptureInit(capturedFields, captureGlobalName)
		capContext.StructDef = initCode

		// Store the init code so NewAssignment can retrieve it
		key := fmt.Sprintf("%d:%d", l.Pos.Line, l.Pos.Column)
		PendingCaptureInit[key] = initCode
	}

	// Generate the function definition
	funcDef := fmt.Sprintf("%s %s(%s) {\n", retType, funcName, params)

	lambdaScope := newLexicalScope(scope, false)
	lambdaInfo := CGetScopeInformation(lambdaScope)
	lambdaInfo.FunctionBoundary = true
	lambdaInfo.CurrentFunc = funcName
	lambdaInfo.CurrentFuncReturnType = l.ReturnType
	lambdaInfo.ClosureCaptures = capContext
	for _, param := range l.Params {
		v := ast.Variable{Name: param.Name, Parent: lambdaScope, IsArgument: true, IsPointer: param.Type != nil && param.Type.Pointer}
		lambdaScope.Variables[param.Name] = v
		(*CProgramValues)[v.GetFullName()] = &CValueInformation{CType: TypeRefToCType(param.Type, scope), GeckoType: param.Type}
	}
	savedState := CurrentTypeState
	CurrentTypeState = lambdaInfo.TypeState
	for _, entry := range l.Body {
		impl.processEntry(lambdaScope, entry)
	}
	if retType == "void" {
		impl.cleanupThrough(lambdaScope, lambdaInfo, false)
	}
	CurrentTypeState = savedState
	bodyCode := lambdaInfo.Code

	funcDef += bodyCode
	funcDef += "}\n"

	// Add the function definition to the root/file scope's Functions list
	rootScope := scope.GetRoot()
	rootInfo := CGetScopeInformation(rootScope)
	rootInfo.Functions = append(rootInfo.Functions, funcDef)

	return funcName
}
