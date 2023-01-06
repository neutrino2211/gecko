package errors

import (
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/fatih/color"
)

type CompileTimeError struct {
	Message string
	Scope   *ErrorScope
	Title   string
	Pos     lexer.Position
}

type ErrorScope struct {
	Name              string
	CompileTimeErrors []*CompileTimeError
	SourceName        string
	Source            string
}

func (ce *CompileTimeError) GetText() string {
	var previousLine string
	var nextLine string

	lineNumber := ce.Pos.Line - 1
	columnNumber := ce.Pos.Column - 1

	lines := strings.Split(ce.Scope.Source, "\n")

	underlineRed := color.New(color.Underline, color.FgHiRed)
	boldRed := color.New(color.FgRed, color.Bold)

	line := lines[lineNumber]
	offendingCode := line[columnNumber:]
	unoffendingCode := line[:columnNumber]

	if lineNumber > 0 {
		previousLine = lines[lineNumber-1]
	}

	if len(lines)-lineNumber > 0 {
		nextLine = lines[lineNumber+1]
	}

	code := previousLine + "\n" + unoffendingCode + underlineRed.Sprint(offendingCode) + "\n" + nextLine

	heading := color.HiWhiteString(ce.Scope.SourceName+":"+ce.Pos.String()) + " => " + boldRed.Sprint(ce.Title) + color.RedString(": "+ce.Message)

	return heading + "\n" + code
}

func (s *ErrorScope) NewCompileTimeError(title string, message string, pos lexer.Position) {
	e := &CompileTimeError{
		Message: message,
		Pos:     pos,
		Scope:   s,
		Title:   title,
	}

	s.CompileTimeErrors = append(s.CompileTimeErrors, e)
}

func (s *ErrorScope) HasErrors() bool {
	return len(s.CompileTimeErrors) > 0
}

func NewErrorScope(name string, sourceName string, source string) *ErrorScope {
	return &ErrorScope{
		Name:              name,
		CompileTimeErrors: make([]*CompileTimeError, 0),
		Source:            source,
		SourceName:        sourceName,
	}
}
