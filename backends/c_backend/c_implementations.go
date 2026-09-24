// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// Helper functions for implementations

func (impl *CBackendImplementation) validateInherentImplCoherence(scope *ast.Ast, className string, classOrigin string, pos lexer.Position) bool {
	currentPackage := scope.GetRoot().Scope
	if classOrigin == "" || classOrigin == currentPackage {
		return true
	}

	typeName := className
	if classOrigin != "" {
		typeName = classOrigin + "." + className
	}

	scope.ErrorScope.NewCompileTimeError(
		"Coherence Error",
		"cannot add inherent impl for foreign type '"+typeName+"'\nhelp: inherent impls are only allowed in the defining package '"+classOrigin+"'",
		pos,
	)
	return false
}

func (impl *CBackendImplementation) resolveTraitOrigin(scope *ast.Ast, traitName string) string {
	if origin, ok := TraitDefinitionOrigins[traitName]; ok && origin != "" {
		return origin
	}

	traitOpt := scope.ResolveTrait(traitName)
	if !traitOpt.IsNil() {
		traitMethods := traitOpt.Unwrap()
		if traitMethods != nil && len(*traitMethods) > 0 {
			first := (*traitMethods)[0]
			if first != nil {
				return first.GetOriginModule()
			}
		}
	}

	return ""
}

func (impl *CBackendImplementation) validateTraitImplCoherence(scope *ast.Ast, class *ast.Ast, className string, traitName string, pos lexer.Position) bool {
	currentPackage := scope.GetRoot().Scope
	classOrigin := class.GetOriginModule()
	traitOrigin := impl.resolveTraitOrigin(scope, traitName)

	classLocal := classOrigin == "" || classOrigin == currentPackage
	traitLocal := traitOrigin == "" || traitOrigin == currentPackage
	if classLocal || traitLocal {
		return true
	}

	typeName := className
	if classOrigin != "" {
		typeName = classOrigin + "." + className
	}

	qualifiedTraitName := traitName
	if traitOrigin != "" {
		qualifiedTraitName = traitOrigin + "." + traitName
	}

	scope.ErrorScope.NewCompileTimeError(
		"Coherence Error",
		"orphan impl is not allowed: both trait '"+qualifiedTraitName+"' and type '"+typeName+"' are foreign\nhelp: define a local trait or wrap the foreign type in a local newtype",
		pos,
	)
	return false
}

func (impl *CBackendImplementation) CImplementationForClass(scope *ast.Ast, i *tokens.Implementation) {
	className := i.GetFor()
	typeParams := i.GetTypeParams()

	// Check if this is a generic trait impl (impl<T> Trait for GenericClass<T>)
	// If so, store it with the generic class for later instantiation
	if len(typeParams) > 0 && Generics.IsGenericClass(className) {
		originModule := Generics.GenericClassOrigins[className]
		classLocal := originModule == "" || originModule == scope.GetRoot().Scope
		traitOrigin := impl.resolveTraitOrigin(scope, i.GetName())
		traitLocal := traitOrigin == "" || traitOrigin == scope.GetRoot().Scope
		if !classLocal && !traitLocal {
			typeName := className
			if originModule != "" {
				typeName = originModule + "." + className
			}
			qualifiedTraitName := i.GetName()
			if traitOrigin != "" {
				qualifiedTraitName = traitOrigin + "." + i.GetName()
			}
			scope.ErrorScope.NewCompileTimeError(
				"Coherence Error",
				"orphan impl is not allowed: both trait '"+qualifiedTraitName+"' and type '"+typeName+"' are foreign\nhelp: define a local trait or wrap the foreign type in a local newtype",
				i.Pos,
			)
			return
		}

		classToken := Generics.GenericClasses[className]
		if classToken != nil {
			classToken.Implementations = append(classToken.Implementations, i)
		}

		// Also register the trait name on the base class so trait lookups work
		// This is needed for `or`/`try` expressions to find the trait before instantiation
		classOpt := scope.ResolveClass(className)
		if !classOpt.IsNil() {
			class := classOpt.Unwrap()
			traitName := i.GetName()
			mangledTraitName := traitName
			if len(i.GetTypeArgs()) > 0 {
				for _, typeArg := range i.GetTypeArgs() {
					mangledTraitName += "__" + MangleTypeArgForIdentifier(typeArg.Type)
				}
			}
			if class.Traits == nil {
				class.Traits = make(map[string]*[]*ast.Method)
			}
			if _, exists := class.Traits[mangledTraitName]; !exists {
				class.Traits[mangledTraitName] = nil // Placeholder - methods generated during instantiation
			}
		}
		return
	}

	classOpt := scope.ResolveClass(className)
	traitOpt := scope.ResolveTrait(i.GetName())

	if classOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the class '"+className+"'", i.Pos)
		return
	}

	if traitOpt.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the trait '"+i.GetName()+"'", i.Pos)
		return
	}

	class := classOpt.Unwrap()
	_ = traitOpt.Unwrap() // Validate trait exists (methods come from TraitDefinitions for default impls)
	traitName := i.GetName()
	if !impl.validateTraitImplCoherence(scope, class, className, traitName, i.Pos) {
		return
	}

	// Build mangled trait name with type arguments (e.g., "Add__Point" for Add<Point>)
	mangledTraitName := traitName
	if len(i.GetTypeArgs()) > 0 {
		for _, typeArg := range i.GetTypeArgs() {
			mangledTraitName += "__" + MangleTypeArgForIdentifier(TypeRefToCType(typeArg, scope))
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

		traitFields, ok := CollectTraitFields(i.GetName())
		if !ok {
			scope.ErrorScope.NewCompileTimeError(
				"Implementation Error",
				"Could not resolve inherited methods for trait '"+i.GetName()+"'",
				i.Pos,
			)
			return
		}

		for _, f := range traitFields {
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
