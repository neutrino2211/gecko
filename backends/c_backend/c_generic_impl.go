// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// GenerateGenericTraitImpl handles trait impl instantiation for generic classes
// Called when we instantiate e.g. Vec<int32> and need to generate impl<T> Index for Vec<T>
func (impl *CBackendImplementation) GenerateGenericTraitImpl(scope *ast.Ast, classToken *tokens.Class, i *tokens.Implementation, inst *GenericInstantiation) {
	// Build type substitution map: T -> int32, U -> string, etc.
	typeMap := make(map[string]string)
	for idx, param := range classToken.TypeParams {
		if idx < len(inst.TypeArgs) {
			typeMap[param.Name] = inst.TypeArgs[idx]
		}
	}

	// Substitute type params in the trait type args
	// e.g., impl<T> Index<uint64, T> -> Index<uint64, int32>
	mangledTraitName := i.GetName()
	if len(i.GetTypeArgs()) > 0 {
		for _, typeArg := range i.GetTypeArgs() {
			substitutedType := typeArg.Type
			if concrete, ok := typeMap[substitutedType]; ok {
				substitutedType = concrete
			}
			mangledTraitName += "__" + MangleTypeArgForIdentifier(substitutedType)
		}
	}

	// Build method name prefix
	methodPrefix := inst.FullName
	if inst.OriginModule != "" {
		methodPrefix = inst.OriginModule + "__" + inst.FullName
	}

	// Resolve the class AST for the instantiated type
	classOpt := scope.ResolveClass(inst.FullName)
	if classOpt.IsNil() {
		return
	}
	class := classOpt.Unwrap()

	var mthdList []*ast.Method

	// Generate each trait method
	for _, f := range i.GetFields() {
		m := f.ToMethodToken()
		mangledName := methodPrefix + "__" + mangledTraitName + "__" + m.Name
		impl.GenerateClassMethodDef(scope, classToken, m, mangledName, inst.FullName, inst.TypeArgs)

		// Register method in AST
		geckoReturnType := "void"
		if m.Type != nil {
			geckoReturnType = m.Type.Type
			// Substitute type param in return type
			if concrete, ok := typeMap[geckoReturnType]; ok {
				geckoReturnType = concrete
			}
		}

		astMth := &ast.Method{
			Name:       mangledName,
			Visibility: m.Visibility,
			Parent:     class,
			Type:       geckoReturnType,
			Unsafe:     tokens.HasAttribute(m.Attributes, "unsafe"),
		}
		scope.Methods[mangledName] = astMth
		mthdList = append(mthdList, astMth)
	}

	// Register the trait on the class
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
			Unsafe:     tokens.HasAttribute(m.Attributes, "unsafe"),
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
