package tokens

import (
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/ast"
)

func findPrimitive(name string) *ast.PrimitiveType {
	for _, p := range ast.Primitives {
		if p.Class.Scope == name {
			return p
		}
	}

	return nil
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

func (t *TypeRef) GetLLIRType(scope *ast.Ast) types.Type {
	prim := findPrimitive(t.Type)

	if prim != nil {
		return prim.Type
	}

	// TODO: Make LLIR type from compound types
	if t.Array != nil {
		return &types.PointerType{
			ElemType: t.Array.GetLLIRType(scope),
		}
	}

	return nil
}
