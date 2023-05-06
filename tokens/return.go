package tokens

import "github.com/neutrino2211/gecko/ast"

func returnLiteral(scope *ast.Ast, l *Literal) {
	scope.LocalContext.MainBlock.NewRet(l.ToLLIRValue(scope))
}
