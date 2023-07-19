package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func (i *Implementation) ToMethodTokens(scope *ast.Ast) []*Method {
	mTokens := make([]*Method, 0)

	for _, m := range i.Fields {
		mTokens = append(mTokens, m.ToMethodToken())
	}

	return mTokens
}

func (i *Implementation) ForClass(scope *ast.Ast) {
	classOpt := scope.ResolveClass(i.For)
	traitOpt := scope.ResolveTrait(i.Name)

	class := classOpt.UnwrapOrElse(func(err error) *ast.Ast {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the class '"+i.For+"'", i.Pos)

		return &ast.Ast{}
	})
	traitMthds := traitOpt.UnwrapOrElse(func(err error) *[]*ast.Method {
		scope.ErrorScope.NewCompileTimeError("Resolution Error", "Could not resolve the trait '"+i.Name+"'", i.Pos)

		return &[]*ast.Method{}
	})

	if i.Fields != nil && i.Default {
		scope.ErrorScope.NewCompileTimeError(
			"Implementation Error",
			"A default trait implementation must not have a body",
			i.Pos,
		)
	}

	if classOpt.IsNil() || traitOpt.IsNil() {
		return
	}

	var mthdList []*ast.Method

	if i.Default {
		mthdList = *traitMthds
	} else {
		for _, m := range i.ToMethodTokens(class) {
			mthdList = append(mthdList, m.ToAstMethod(scope))
		}
	}

	class.Traits[i.Name] = &mthdList
}

func (i *Implementation) ForArch(scope *ast.Ast) {
	if i.Default {
		scope.ErrorScope.NewCompileTimeError(
			"Implementation Error",
			"An architecture implementation must not be default",
			i.Pos,
		)

		return
	}

	if scope.Config.Arch == i.Name {
		for _, m := range i.ToMethodTokens(scope) {
			scope.Methods[m.Name] = m.ToAstMethod(scope)
		}
	} else {
		scope.ErrorScope.NewCompileTimeWarning(
			"Arch Implementation",
			"Implementation for the arch '"+i.Name+"' was skipped due to target being '"+scope.Config.Arch+"'",
			i.Pos,
		)
	}
}
