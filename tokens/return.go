package tokens

import (
	"github.com/neutrino2211/gecko/ast"
)

func returnLiteral(scope *ast.Ast, l *Expression) {
	scope.LocalContext.MainBlock.NewRet(l.ToLLIRValue(scope, &TypeRef{}))
}

func returnVoid(scope *ast.Ast) {
	scope.LocalContext.MainBlock.NewRet(nil)
}
