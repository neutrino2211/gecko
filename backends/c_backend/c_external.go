// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

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
		Scope:      ext.Name,
		Visibility: "external", // External types should be accessible from anywhere
	}

	// Generate C typedef for opaque type
	info := CGetScopeInformation(scope)
	typedef := fmt.Sprintf("typedef struct %s %s;", ext.Name, ext.Name)
	info.TypeDefs = append(info.TypeDefs, typedef)
}

// NewExternalVariable handles external variable declarations
func (impls *CBackendImplementation) NewExternalVariable(scope *ast.Ast, f *tokens.Field) {
	info := CGetScopeInformation(scope)

	// Validate the type before conversion
	if f.Type != nil {
		f.Type.Check(scope)
	}

	cType := TypeRefToCType(f.Type, scope)
	varName := f.Name

	// Generate extern declaration
	externDecl := fmt.Sprintf("extern %s %s;", cType, varName)
	info.Declarations = append(info.Declarations, externDecl)

	// Register the variable in AST
	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    bindingIsConst(f),
		IsVolatile: f.Type != nil && f.Type.Volatile,
		IsPointer:  f.Type != nil && f.Type.Pointer,
		IsExternal: true,
		Parent:     scope,
	}
	scope.Variables[f.Name] = fieldVariable
}

func buildExternalFuncDecl(returnType string, name string, params string) string {
	return fmt.Sprintf("extern %s %s(%s);", returnType, name, params)
}

func unquoteIfQuoted(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

func (impls *CBackendImplementation) registerExternalMethod(symbolScope *ast.Ast, declScope *ast.Ast, m *tokens.Method, emitDecl bool) {
	if symbolScope == nil || m == nil {
		return
	}
	if declScope == nil {
		declScope = symbolScope
	}
	info := CGetScopeInformation(declScope)

	// Determine return type
	returnType := "void"
	if m.Type != nil {
		m.Type.Check(symbolScope)
		returnType = TypeRefToCType(m.Type, symbolScope)
	}

	// Build parameter list
	params := []string{}
	for _, arg := range m.Arguments {
		// Validate parameter type
		if arg.Type != nil {
			arg.Type.Check(symbolScope)
		}
		paramType := TypeRefToCType(arg.Type, symbolScope)
		if arg.Out {
			if arg.Type != nil && arg.Type.FuncType != nil {
				symbolScope.ErrorScope.NewCompileTimeError(
					"Unsupported Out Parameter",
					fmt.Sprintf("external out parameter '%s' cannot be a function type yet", arg.Name),
					m.Pos,
				)
			}
			paramType += "*"
		}
		if IsFuncPointerType(paramType) && !arg.Out {
			params = append(params, FormatFuncPointerDecl(paramType, arg.Name))
		} else {
			params = append(params, paramType+" "+arg.Name)
		}
	}

	paramStr := strings.Join(params, ", ")

	// Handle variadic functions
	if m.IsVariadic() {
		if len(params) > 0 {
			paramStr += ", ..."
		} else {
			paramStr = "..."
		}
	}

	symbolName := m.Name
	if strings.TrimSpace(m.LinkName) != "" {
		symbolName = unquoteIfQuoted(strings.TrimSpace(m.LinkName))
	}

	if emitDecl {
		externDecl := buildExternalFuncDecl(returnType, symbolName, paramStr)
		if strings.Contains(returnType, "__FUNCPTR__") {
			// Preserve original form for uncommon function-pointer return declarations.
			externDecl = fmt.Sprintf("extern %s %s(%s);", returnType, symbolName, paramStr)
		}
		info.Declarations = append(info.Declarations, externDecl)
	}

	// Register the method in AST.
	geckoReturnType := "void"
	if m.Type != nil {
		geckoReturnType = m.Type.Type
	}
	astMth := &ast.Method{
		Name:           m.Name,
		Scope:          nil, // External, no scope
		Arguments:      make([]ast.Variable, 0),
		Visibility:     "external",
		Parent:         symbolScope,
		Type:           geckoReturnType,
		ExternalSymbol: map[bool]string{true: symbolName, false: ""}[symbolName != m.Name],
		Unsafe:         tokens.HasAttribute(m.Attributes, "unsafe"),
	}

	for _, arg := range m.Arguments {
		astMth.Arguments = append(astMth.Arguments, ast.Variable{
			Name:      arg.Name,
			IsPointer: (arg.Type != nil && arg.Type.Pointer) || arg.Out,
			Parent:    nil,
		})
	}

	symbolScope.Methods[m.Name] = astMth
	Methods[symbolScope.FullScopeName()+"#"+m.Name] = astMth
	RegisterMethodSignature(m.Name, m)
	RegisterMethodSignature(symbolScope.GetFullName()+"__"+m.Name, m)
	if symbolName != m.Name {
		RegisterMethodSignature(symbolName, m)
		RegisterMethodSignature(symbolScope.GetFullName()+"__"+symbolName, m)
	}

	// Track full return type for generic type support
	if m.Type != nil {
		MethodReturnTypes[symbolScope.FullScopeName()+"#"+m.Name] = m.Type
		funcAlias := astMth.GetFullName()
		MethodReturnTypes[funcAlias] = m.Type
		MethodReturnTypes[symbolScope.FullScopeName()+"#"+funcAlias] = m.Type
		if symbolName != m.Name {
			MethodReturnTypes[symbolName] = m.Type
			MethodReturnTypes[symbolScope.FullScopeName()+"#"+symbolName] = m.Type
		}
	}
}

// NewExternalMethod handles external method declarations (e.g., printf)
func (impls *CBackendImplementation) NewExternalMethod(scope *ast.Ast, m *tokens.Method) {
	impls.registerExternalMethod(scope, scope, m, true)
}

func reportOutParamsOnlyForExternal(scope *ast.Ast, m *tokens.Method) bool {
	if m == nil || scope == nil || m.Visibility == "external" {
		return false
	}

	hasError := false
	for _, arg := range m.Arguments {
		if arg.Out {
			hasError = true
			scope.ErrorScope.NewCompileTimeError(
				"Unsupported Out Parameter",
				fmt.Sprintf("parameter '%s' uses 'out', but out parameters are currently supported only for 'declare external func'", arg.Name),
				m.Pos,
			)
		}
	}
	return hasError
}

func ensureForeignModuleScope(root *ast.Ast, moduleName string) *ast.Ast {
	if root == nil || moduleName == "" {
		return nil
	}
	if existing, ok := root.Children[moduleName]; ok && existing != nil {
		return existing
	}

	moduleScope := &ast.Ast{
		Scope:            moduleName,
		Parent:           nil,
		IsImportedModule: true,
		OriginModule:     root.GetRoot().Scope,
		SourceFile:       root.GetSourceFile(),
	}
	moduleScope.Init(root.ErrorScope)
	moduleScope.Config = root.Config
	root.Children[moduleName] = moduleScope
	return moduleScope
}

// NewForeign handles a foreign interop block.
func (impl *CBackendImplementation) NewForeign(scope *ast.Ast, foreign *tokens.Foreign) {
	if scope == nil || foreign == nil {
		return
	}
	rootScope := scope.GetRoot()
	info := CGetScopeInformation(rootScope)

	backendName := strings.TrimSpace(unquoteIfQuoted(foreign.Backend))
	if backendName == "" {
		backendName = "c"
	}
	if backendName != "c" {
		scope.ErrorScope.NewCompileTimeError(
			"Foreign Backend Error",
			fmt.Sprintf("unsupported foreign backend '%s' for C backend (expected \"c\")", backendName),
			foreign.Pos,
		)
		return
	}

	moduleScope := ensureForeignModuleScope(rootScope, foreign.Module)
	if moduleScope == nil {
		scope.ErrorScope.NewCompileTimeError(
			"Foreign Module Error",
			"unable to initialize foreign module scope",
			foreign.Pos,
		)
		return
	}

	headers := foreign.GetWithHeaders()
	for _, headerRaw := range headers {
		header := strings.TrimSpace(unquoteIfQuoted(headerRaw))
		if header == "" {
			continue
		}
		if strings.HasPrefix(header, "<") {
			info.Includes = append(info.Includes, "#include "+header)
		} else {
			info.Includes = append(info.Includes, "#include \""+header+"\"")
		}
	}

	for _, libRaw := range foreign.GetWithLibraries() {
		lib := strings.TrimSpace(unquoteIfQuoted(libRaw))
		if lib != "" {
			info.CImportLibraries = append(info.CImportLibraries, lib)
		}
	}

	for _, objRaw := range foreign.GetWithObjects() {
		obj := strings.TrimSpace(unquoteIfQuoted(objRaw))
		if obj == "" {
			continue
		}
		if !filepath.IsAbs(obj) && scope.SourceFile != "" {
			obj = filepath.Join(filepath.Dir(scope.SourceFile), obj)
		}
		info.CImportObjects = append(info.CImportObjects, obj)
	}

	hasHeaders := len(headers) > 0

	for _, member := range foreign.Members {
		if member == nil || member.Type == nil {
			continue
		}
		ext := &tokens.ExternalType{Name: member.Type.Name}
		impl.NewExternalType(rootScope, ext)
		if classOpt := rootScope.ResolveClass(member.Type.Name); !classOpt.IsNil() {
			moduleScope.Classes[member.Type.Name] = classOpt.Unwrap()
		}
	}

	for _, member := range foreign.Members {
		if member == nil || member.Method == nil || member.Method.Name == "" {
			continue
		}
		method := &tokens.Method{
			Visibility: "external",
			Name:       member.Method.Name,
			Arguments:  member.Method.Arguments,
			Variadic:   member.Method.IsVariadic(),
			Type:       member.Method.Type,
			Throws:     member.Method.Throws,
			LinkName:   member.Method.As,
		}
		impl.registerExternalMethod(moduleScope, rootScope, method, !hasHeaders)
	}
}

// NewCImport handles a C import statement, adding #include directive
func (impl *CBackendImplementation) NewCImport(scope *ast.Ast, cimport *tokens.CImport) {
	info := CGetScopeInformation(scope)

	// Add the include directive
	header := unquoteIfQuoted(cimport.Header)

	// Check if it's a system header (angle brackets) or local header
	if len(header) > 0 && header[0] == '<' {
		// System header: <stdio.h> -> #include <stdio.h>
		info.Includes = append(info.Includes, "#include "+header)
	} else {
		// Local header: myheader.h -> #include "myheader.h"
		info.Includes = append(info.Includes, "#include \""+header+"\"")
	}

	// Track libraries/objects for compile/link stages.
	for _, lib := range cimport.GetWithLibraries() {
		lib = unquoteIfQuoted(lib)
		if lib != "" {
			info.CImportLibraries = append(info.CImportLibraries, lib)
		}
	}

	for _, obj := range cimport.GetWithObjects() {
		obj = unquoteIfQuoted(obj)
		if obj == "" {
			continue
		}

		// Resolve relative object paths against the current source file.
		if !filepath.IsAbs(obj) && scope.SourceFile != "" {
			obj = filepath.Join(filepath.Dir(scope.SourceFile), obj)
		}

		info.CImportObjects = append(info.CImportObjects, obj)
	}
}
