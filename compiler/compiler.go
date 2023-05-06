package compiler

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/repr"
	"github.com/fatih/color"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/go-option"
)

func streamPipe(std io.ReadCloser) {
	buf := bufio.NewReader(std) // Notice that this is not in a loop
	for {

		line, _, err := buf.ReadLine()
		if err != nil {
			break
		}
		fmt.Println(string(line))
	}
}

func streamCommand(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Start()
	streamPipe(stdout)
	streamPipe(stderr)

	return nil
}

var allErrorScopes []*errors.ErrorScope = make([]*errors.ErrorScope, 0)

func Compile(file string, config *config.CompileCfg) string {
	fileOpt := option.SomePair(os.ReadFile(file))
	fileContents := fileOpt.Expect("Unable to read file '" + file + "'")

	sourceFile := &tokens.File{}

	tokenError := parser.Parser.ParseString(
		string(fileContents),
		sourceFile,
	)

	sourceFile.Content = string(fileContents)
	sourceFile.Path = file

	ast := sourceFile.ToAst(config)

	// repr.Println(ast)

	repr.Println(ast.ProgramContext.Module)
	llir := ast.ProgramContext.Module.String()

	ts := strconv.Itoa(int(time.Now().UnixNano()))

	buildDir := os.TempDir() + "/gecko/build/" + ts

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

	os.MkdirAll(buildDir, 0o755)

	outName := buildDir + "/" + file + ".ll"
	compiledName := buildDir + "/" + file + ".o"

	os.WriteFile(outName, []byte(llir), 0o755)

	cmd := exec.Command("llc", "-filetype=obj", outName)

	err := streamCommand(cmd)

	if err != nil {
		ast.ErrorScope.NewCompileTimeError("LLVM compilation", "Error compiling LLVM IR "+err.Error(), lexer.Position{})
	}

	allErrorScopes = append(allErrorScopes, ast.ErrorScope)

	return compiledName
}

func PrintErrorSummary() {
	var warnings, errors int = 0, 0
	var bold, boldYellow, boldRed *color.Color = color.New(color.Bold), color.New(color.Bold, color.FgHiYellow), color.New(color.Bold, color.FgHiRed)
	for _, e := range allErrorScopes {
		if e.HasWarnings() {
			for _, e := range e.CompileTimeWarnings {
				fmt.Println(e.GetWarning())
			}
		}

		if e.HasErrors() {
			for _, e := range e.CompileTimeErrors {
				fmt.Println(e.GetError())
			}
		}

		fmt.Println(e.GetSummary() + "\n")

		errors += len(e.CompileTimeErrors)
		warnings += len(e.CompileTimeWarnings)
	}

	fmt.Printf(
		bold.Sprint("\nTotal of ")+
			boldYellow.Sprint("%d warnings")+
			bold.Sprint(" and ")+
			boldRed.Sprint("%d errors")+
			bold.Sprint(" generated\n"),
		warnings, errors)
}
