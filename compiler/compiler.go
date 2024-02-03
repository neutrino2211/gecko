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
	"github.com/fatih/color"
	"github.com/neutrino2211/gecko/backends"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/interfaces"
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
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
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
	sourceFile.Config = config

	compileErrorScope := errors.NewErrorScope("compile", sourceFile.Name, string(fileContents))

	allErrorScopes = append(allErrorScopes, compileErrorScope)

	ts := strconv.Itoa(int(time.Now().UnixNano()))

	buildDir := os.TempDir() + "/gecko/build/" + ts

	outName := buildDir + "/" + file + ".ll"
	compiledName := buildDir + "/" + file + ".o"
	backend := config.Ctx.String("backend")

	if tokenError != nil {
		var line, column int
		var unexpectedToken, expectedToken string
		r := tokenError.Error()
		res, e := fmt.Sscanf(r, "%d:%d: unexpected token \"%s\" (expected \"%s\")", &line, &column, &unexpectedToken, &expectedToken)

		if res < 2 {
			compileErrorScope.NewCompileTimeError(
				"Horrible syntax error",
				"There is a horrible syntax error in the file that even the lexer can't recover from\n\n"+e.Error(),
				lexer.Position{
					Line:   line,
					Column: column,
				},
			)
		} else {
			compileErrorScope.NewCompileTimeError(
				"Syntax error",
				tokenError.Error(),
				lexer.Position{
					Line:   line,
					Column: column,
				},
			)
		}
	}

	compilationBackend, ok := backends.Backends[backend]

	if !ok {
		println(color.RedString("Backend '" + backend + "' not found."))
		os.Exit(0)
	}

	if haveErrors() {
		return ""
	}

	os.MkdirAll(buildDir, 0o755)

	compilationBackend.Init()

	cmd := compilationBackend.Compile(&interfaces.BackendConfig{
		OutName:    outName,
		File:       file,
		Ctx:        config.Ctx,
		SourceFile: sourceFile,
	})

	var err error = nil

	if cmd != nil {
		err = streamCommand(cmd)
	}

	if err != nil {
		compileErrorScope.NewCompileTimeError("Compilation Backend Error", "Error compiling for backend '"+backend+"' "+err.Error(), lexer.Position{})
	}

	return compiledName
}

func haveErrors() bool {
	for _, e := range allErrorScopes {
		println(e.Name, e.HasErrors())
		if e.HasErrors() {
			return true
		}
	}

	return false
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

	fmt.Print(
		bold.Sprint("\nTotal of ") +
			boldYellow.Sprintf("%d warnings", warnings) +
			bold.Sprint(" and ") +
			boldRed.Sprintf("%d errors", errors) +
			bold.Sprint(" generated\n"),
	)
}
