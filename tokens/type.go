package tokens

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/go-option"
)

// PrimitiveTypes contains all built-in primitive type names
var PrimitiveTypes = map[string]bool{
	"void":   true,
	"bool":   true,
	"int":    true,
	"int8":   true,
	"int16":  true,
	"int32":  true,
	"int64":  true,
	"uint":   true,
	"uint8":  true,
	"uint16": true,
	"uint32": true,
	"uint64": true,
	"string": true,
}

// IsPrimitive returns true if the type name is a built-in primitive
func IsPrimitive(typeName string) bool {
	return PrimitiveTypes[typeName]
}

func (t *TypeRef) checkTrait(typeAst *ast.Ast, scope *ast.Ast) bool {
	if t.Trait == "" {
		return true
	}

	traitOpt := typeAst.ResolveTrait(t.Trait)

	hasTrait := traitOpt.IsNil()

	if !hasTrait {
		scope.ErrorScope.NewCompileTimeError(
			"Type Check Error",
			"Type '"+t.Type+"' does not implement the trait '"+t.Trait+"'",
			t.Pos,
		)
	}

	return hasTrait
}

// IsTypeParameter is set by the backend during generic processing
// to allow Check to skip type parameters like T
var IsTypeParameter func(name string) bool

func (t *TypeRef) Check(scope *ast.Ast) bool {
	if t.Array != nil {
		return t.Array.Check(scope)
	}

	// Fixed-size arrays: [N]T - check the inner type
	if t.Size != nil && t.Size.Type != nil {
		return t.Size.Type.Check(scope)
	}

	// Primitive types are always valid and don't need class resolution
	if IsPrimitive(t.Type) {
		return true
	}

	// Skip type parameters (e.g., T in generic contexts)
	if IsTypeParameter != nil && IsTypeParameter(t.Type) {
		return true
	}

	// Handle module-qualified types (e.g., console.Console)
	var classAstOpt *option.Optional[*ast.Ast]
	if t.Module != "" {
		root := scope.GetRoot()
		if moduleAst, ok := root.Children[t.Module]; ok {
			classAstOpt = moduleAst.ResolveClass(t.Type)
		} else if root.LazyModuleTypeResolver != nil {
			// Try lazy resolution for directory imports
			if classAst, found := root.LazyModuleTypeResolver(t.Module, t.Type); found {
				classAstOpt = option.Some(classAst)
			} else {
				scope.ErrorScope.NewCompileTimeError(
					"Type Check Error",
					"Unable to resolve type '"+t.Module+"."+t.Type+"'",
					t.Pos,
				)
				return false
			}
		} else {
			// Debug: print available modules
			availableModules := ""
			for name := range root.Children {
				if availableModules != "" {
					availableModules += ", "
				}
				availableModules += name
			}
			scope.ErrorScope.NewCompileTimeError(
				"Module Error",
				"Module '"+t.Module+"' not found (available: "+availableModules+", root scope: "+root.Scope+")",
				t.Pos,
			)
			return false
		}
	} else {
		classAstOpt = scope.ResolveClass(t.Type)
	}

	// If class not found, report error
	if classAstOpt.IsNil() {
		errorMsg := "Unable to resolve type '" + t.Type + "'"

		// Try to get import suggestions
		root := scope.GetRoot()
		if root.SuggestionProvider != nil {
			if suggestion := root.SuggestionProvider(t.Type); suggestion != "" {
				errorMsg += suggestion
			}
		}

		scope.ErrorScope.NewCompileTimeError(
			"Type Check Error",
			errorMsg,
			t.Pos,
		)
		return false
	}

	classAst := classAstOpt.Unwrap()

	// Check visibility for cross-module access
	// Only check if the class has a valid origin module (not an empty placeholder)
	if classAst.Scope != "" {
		if visibilityErr := classAst.CheckVisibility(scope, t.Type); visibilityErr != "" {
			scope.ErrorScope.NewCompileTimeError(
				"Visibility Error",
				visibilityErr,
				t.Pos,
			)
			return false
		}
	}

	hasTrait := t.checkTrait(classAst, scope)

	return hasTrait
}
