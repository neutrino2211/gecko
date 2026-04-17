package tokens

import (
	"github.com/neutrino2211/gecko/ast"
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

func (t *TypeRef) Check(scope *ast.Ast) bool {
	if t.Array != nil {
		return t.Array.Check(scope)
	}

	// Primitive types are always valid and don't need class resolution
	if IsPrimitive(t.Type) {
		return true
	}

	classAstOpt := scope.ResolveClass(t.Type)

	classAst := classAstOpt.UnwrapOrElse(func(err error) *ast.Ast {
		scope.ErrorScope.NewCompileTimeError(
			"Type Check Error",
			"Unable to resolve type '"+t.Type+"'",
			t.Pos,
		)

		return &ast.Ast{}
	})

	hasTrait := t.checkTrait(classAst, scope)

	return hasTrait
}
