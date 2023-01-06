package compiler

import (
	"fmt"
	"os"

	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/repr"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
)

func Compile(file string) {
	fileContents, err := os.ReadFile(file)

	if err != nil {
		return
	}

	sourceFile := &tokens.File{}

	tokenError := parser.Parser.ParseString(
		string(fileContents),
		sourceFile,
	)

	sourceFile.Content = string(fileContents)
	sourceFile.Path = file

	ast := sourceFile.ToAst()

	repr.Println(ast)

	if tokenError != nil {
		var line, column int
		var unexpectedToken, expectedToken string
		r := tokenError.Error()
		res, e := fmt.Sscanf(r, "%d:%d: unexpected token \"%s\" (expected \"%s\")", &line, &column, &unexpectedToken, &expectedToken)

		if res < 2 {
			ast.ErrorScope.NewCompileTimeError(
				"Horrible syntax error",
				"There is a horrible syntax error in the file that even the lexer can't recover from\n\n"+e.Error(),
				lexer.Position{
					Line:   line,
					Column: column,
				},
			)
		} else {
			ast.ErrorScope.NewCompileTimeError(
				"Syntax error",
				tokenError.Error(),
				lexer.Position{
					Line:   line,
					Column: column,
				},
			)
		}
	}

	if ast.ErrorScope.HasErrors() {
		for _, e := range ast.ErrorScope.CompileTimeErrors {
			fmt.Println(e.GetText())
		}
	}
}
