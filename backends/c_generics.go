// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
)

// generateGenericInstantiations generates C code for all pending generic instantiations
func (b *CBackend) generateGenericInstantiations(scope *ast.Ast) {
	impl := b.impls.(*cbackend.CBackendImplementation)

	// Generate class instantiations (struct + methods).
	// Use an index-based queue so nested instantiations requested during codegen
	// (e.g., Option<Vec<int32>> discovered while generating Vec<int32>) are
	// processed in the same compilation pass.
	for i := 0; i < len(cbackend.Generics.ClassInstantiations); i++ {
		inst := cbackend.Generics.ClassInstantiations[i]
		classToken, ok := cbackend.Generics.GenericClasses[inst.Name]
		if !ok {
			continue
		}
		impl.GenerateClassDef(scope, classToken, inst.FullName, inst.TypeArgs)

		// Build method name prefix with origin module if present
		methodPrefix := inst.FullName
		if inst.OriginModule != "" {
			methodPrefix = inst.OriginModule + "__" + inst.FullName
		}

		// Also generate methods for this class instantiation
		for _, f := range classToken.Fields {
			if f.Method != nil {
				methodName := methodPrefix + "__" + f.Method.Name
				impl.GenerateClassMethodDef(scope, classToken, f.Method, methodName, inst.FullName, inst.TypeArgs)
			}
		}

		// Generate methods from attached impl blocks
		for _, implBlock := range classToken.Implementations {
			if implBlock.GetFor() != "" {
				// Trait impl (impl<T> Trait for Class<T>)
				// Generate trait methods with proper naming
				impl.GenerateGenericTraitImpl(scope, classToken, implBlock, inst)
			} else {
				// Inherent impl (impl<T> Class<T> { ... })
				for _, f := range implBlock.GetFields() {
					m := f.ToMethodToken()
					methodName := methodPrefix + "__" + m.Name
					impl.GenerateClassMethodDef(scope, classToken, m, methodName, inst.FullName, inst.TypeArgs)
				}
			}
		}
	}

	// Generate method instantiations. Use queue semantics for the same reason as
	// class instantiations: generation may request additional method monomorphs.
	for i := 0; i < len(cbackend.Generics.MethodInstantiations); i++ {
		inst := cbackend.Generics.MethodInstantiations[i]
		methodToken, ok := cbackend.Generics.GenericMethods[inst.Name]
		if !ok {
			continue
		}

		// Validate trait constraints
		for i, param := range methodToken.TypeParams {
			if len(param.AllTraits()) > 0 && i < len(inst.TypeArgs) {
				concreteType := inst.TypeArgs[i]
				// Remove pointer suffix for class lookup
				baseType := concreteType
				if len(baseType) > 0 && baseType[len(baseType)-1] == '*' {
					baseType = baseType[:len(baseType)-1]
				}
				// Also remove "struct " prefix if present
				if strings.HasPrefix(baseType, "struct ") {
					baseType = baseType[7:]
				}

				// Look up the class and check if it implements the trait
				classOpt := scope.ResolveClass(baseType)
				if !classOpt.IsNil() {
					class := classOpt.Unwrap()
					for _, requiredTrait := range param.AllTraits() {
						hasTrait := false
						for implementedTrait := range class.Traits {
							if cbackend.TraitMatchesOrExtends(implementedTrait, requiredTrait) {
								hasTrait = true
								break
							}
						}
						if !hasTrait {
							scope.ErrorScope.NewCompileTimeError(
								"Trait Constraint Error",
								"Type '"+baseType+"' does not implement trait '"+requiredTrait+"' required by type parameter '"+param.Name+"'",
								lexer.Position{},
							)
						}
					}
				}
			}
		}

		impl.GenerateMethodDef(scope, methodToken, inst.FullName, inst.TypeArgs)
	}

	// Clear for next compilation
	cbackend.ResetGenerics()
}
