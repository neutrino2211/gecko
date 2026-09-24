// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// NewImplementation handles implementations (trait impls, inherent impls, arch impls)
func (impl *CBackendImplementation) NewImplementation(scope *ast.Ast, i *tokens.Implementation) {
	// Register unsafe-handler coverage from `@attach_handler(...)` attributes.
	// `impl UnsafeHandler for X` declares (globally) which intrinsics X guards;
	// the `@unsafe with` lowering consults this to wrap intrinsics with guards.
	if i.GetName() == "UnsafeHandler" && i.GetFor() != "" {
		for _, attr := range i.GetAttributes() {
			if attr.Name == "attach_handler" {
				handler := normalizeHandlerTypeName(i.GetFor())
				for _, arg := range attr.Args {
					intr := strings.TrimSpace(stripAttributeString(arg.String))
					if intr != "" {
						UnsafeHandlerCoverage[handler] = append(UnsafeHandlerCoverage[handler], intr)
					}
				}
			}
		}
	}
	if i.GetFor() != "" {
		// `impl Trait for Class` - trait implementation
		impl.CImplementationForClass(scope, i)
	} else {
		// Check if this is a generic impl (impl<T> ClassName<T>)
		typeParams := i.GetTypeParams()
		className := i.GetName()

		// Check if the target class is a registered generic class
		if Generics.IsGenericClass(className) {
			originModule := Generics.GenericClassOrigins[className]
			if !impl.validateInherentImplCoherence(scope, className, originModule, i.Pos) {
				return
			}

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
			class := classOpt.Unwrap()
			if !impl.validateInherentImplCoherence(scope, className, class.GetOriginModule(), i.Pos) {
				return
			}

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
			impl.CInherentImplementation(scope, i, class)
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
		SourceFile:   scope.GetSourceFile(),
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
	// Register early so recursive lookups can resolve this trait while validating inheritance.
	prevDef, hadPrevDef := TraitDefinitions[t.Name]
	prevOrigin, hadPrevOrigin := TraitDefinitionOrigins[t.Name]
	TraitDefinitions[t.Name] = t
	TraitDefinitionOrigins[t.Name] = scope.GetRoot().Scope
	restoreOnError := func() {
		if hadPrevDef {
			TraitDefinitions[t.Name] = prevDef
		} else {
			delete(TraitDefinitions, t.Name)
		}
		if hadPrevOrigin {
			TraitDefinitionOrigins[t.Name] = prevOrigin
		} else {
			delete(TraitDefinitionOrigins, t.Name)
		}
	}

	// Validate parent trait existence and inheritance cycles.
	if t.Parent != "" {
		if t.Parent == t.Name {
			scope.ErrorScope.NewCompileTimeError(
				"Trait Inheritance Error",
				"Trait '"+t.Name+"' cannot inherit from itself",
				t.Pos,
			)
			restoreOnError()
			return
		}
		if _, ok := TraitDefinitions[t.Parent]; !ok && scope.ResolveTrait(t.Parent).IsNil() {
			scope.ErrorScope.NewCompileTimeError(
				"Resolution Error",
				"Could not resolve parent trait '"+t.Parent+"' for trait '"+t.Name+"'",
				t.Pos,
			)
			restoreOnError()
			return
		}
		if TraitExtendsTrait(t.Parent, t.Name) {
			scope.ErrorScope.NewCompileTimeError(
				"Trait Inheritance Error",
				"Circular trait inheritance detected: '"+t.Name+"' and '"+t.Parent+"' depend on each other",
				t.Pos,
			)
			restoreOnError()
			return
		}

		// Validate child overrides against inherited method signatures.
		inheritedFields, ok := CollectTraitFields(t.Parent)
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Trait Inheritance Error",
				"Could not resolve inherited methods for trait '"+t.Name+"'",
				t.Pos,
			)
			restoreOnError()
			return
		}
		inheritedByName := make(map[string]*tokens.ImplementationField)
		inheritedOwner := make(map[string]string)
		for _, field := range inheritedFields {
			inheritedByName[field.Name] = field
			owner := FindTraitMethodOwner(t.Parent, field.Name)
			if owner == "" {
				owner = t.Parent
			}
			inheritedOwner[field.Name] = owner
		}
		for _, childField := range t.Fields {
			parentField, exists := inheritedByName[childField.Name]
			if !exists {
				continue
			}
			signaturesMatch, reason := TraitMethodSignaturesCompatible(parentField, childField)
			if signaturesMatch {
				continue
			}
			owner := inheritedOwner[childField.Name]
			scope.ErrorScope.NewCompileTimeError(
				"Trait Inheritance Error",
				"Method '"+childField.Name+"' in trait '"+t.Name+"' conflicts with inherited method '"+owner+"."+childField.Name+
					"': "+reason+" (parent: "+TraitMethodSignature(parentField)+", child: "+TraitMethodSignature(childField)+")",
				childField.Pos,
			)
			restoreOnError()
			return
		}
	}

	// Set up type parameter checking for traits
	// Include "Self" and the trait's own name as valid type parameters
	oldIsTypeParameter := tokens.IsTypeParameter
	typeParams := map[string]bool{
		"Self": true,
		t.Name: true, // Allow trait to reference itself in method signatures
	}
	for _, tp := range t.TypeParams {
		typeParams[tp.Name] = true
	}
	tokens.IsTypeParameter = func(name string) bool {
		return typeParams[name]
	}
	defer func() { tokens.IsTypeParameter = oldIsTypeParameter }()

	// Validate methods declared directly on this trait.
	for _, f := range t.Fields {
		m := f.ToMethodToken()

		// Validate return type
		if m.Type != nil {
			m.Type.Check(scope)
		}

		// Validate argument types
		for _, arg := range m.Arguments {
			if arg.Type != nil && arg.Name != "self" {
				arg.Type.Check(scope)
			}
		}

	}

	// Store full trait method set (own + inherited) for implementation checks.
	allFields, ok := CollectTraitFields(t.Name)
	if !ok {
		scope.ErrorScope.NewCompileTimeError(
			"Trait Inheritance Error",
			"Could not resolve inherited methods for trait '"+t.Name+"'",
			t.Pos,
		)
		restoreOnError()
		return
	}

	mthds := []*ast.Method{}
	for _, f := range allFields {
		m := f.ToMethodToken()
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
		SourceFile:   scope.GetSourceFile(),
	}

	methodScope.Init(scope.ErrorScope)
	methodScope.Config = scope.Config

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
			// Validate parameter type before conversion
			if arg.Type != nil {
				arg.Type.Check(scope)
			}
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
			IsConst:    argBindingIsConst(arg.Type),
			IsVolatile: arg.Type != nil && arg.Type.Volatile,
			IsArgument: true,
			IsReceiver: paramName == "self",
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
		Unsafe:     tokens.HasAttribute(m.Attributes, "unsafe"),
	}

	if len(m.Value) == 0 {
		astMth.Scope = nil
	}

	scope.Methods[mangledName] = astMth
	Methods[scope.FullScopeName()+"#"+mangledName] = astMth

	// Track full return type for generic type support
	if m.Type != nil {
		MethodReturnTypes[scope.FullScopeName()+"#"+mangledName] = m.Type
		MethodReturnTypes[mangledName] = m.Type
		// Alias for static method return-type lookup by source-level name (Type::method).
		MethodReturnTypes[className+"#"+m.Name] = m.Type
	}

	// Initialize method scope info
	mthInfo := &CScopeInformation{}
	mthInfo.Init()
	mthInfo.FunctionBoundary = true
	mthInfo.CurrentFunc = mangledName
	mthInfo.CurrentFuncReturnType = m.Type // Track return type for validation
	markMethodScopeUnsafe(m, mthInfo)
	// guard/catch of an UnsafeHandler are themselves the unsafe boundary, so
	// their bodies may use the unsafe intrinsics (e.g. @trap) without an
	// explicit @unsafe block.
	if strings.Contains(mangledName, "__UnsafeHandler__guard") || strings.Contains(mangledName, "__UnsafeHandler__catch") {
		mthInfo.InUnsafe = true
	}
	(*CScopeDataMap)[methodScope.GetFullName()] = mthInfo

	// Process method body
	if len(m.Value) > 0 {
		loadPrimitives(&methodScope)
		prevTypeState := CurrentTypeState
		CurrentTypeState = mthInfo.TypeState
		impl.Backend.ProcessEntries(m.Value, &methodScope)
		CurrentTypeState = prevTypeState

		if returnType == "void" && !strings.HasSuffix(strings.TrimSpace(mthInfo.Code), "return;") {
			impl.emitDefers(&methodScope, mthInfo)
			impl.generateDropCalls(&methodScope, mthInfo)
			mthInfo.Code += "    return;\n"
		}
	}

	// Generate function signature
	isStatic := tokens.HasAttribute(m.Attributes, "static")
	funcDecl := fmt.Sprintf("%s %s(%s)", returnType, mangledName, paramStr)
	if isStatic {
		funcDecl = "static " + funcDecl
	}

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
