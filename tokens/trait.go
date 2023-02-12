package tokens

import "github.com/neutrino2211/gecko/ast"

func (t *Trait) GetMethods() []*Method {
	mthds := make([]*Method, 0)
	for _, f := range t.Fields {
		mthds = append(mthds, f.ToMethodToken())
	}

	return mthds
}

func (t *Trait) AssignToScope(scope *ast.Ast) {
	mthds := []*ast.Method{}

	for _, m := range t.GetMethods() {
		mthds = append(mthds, m.ToAstMethod(scope))
	}
	scope.Traits[t.Name] = &mthds
}
