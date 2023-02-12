package errors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/fatih/color"
)

type CompileTimeMessage struct {
	Message string
	Scope   *ErrorScope
	Title   string
	Pos     lexer.Position
}

type ErrorScope struct {
	Name                string
	CompileTimeErrors   []*CompileTimeMessage
	CompileTimeWarnings []*CompileTimeMessage
	SourceName          string
	Source              *string
}

func addTabs(in string) string {
	out := ""

	parts := strings.Split(in, "\n")

	for _, part := range parts {
		out += color.YellowString("    | ") + part + "\n"
	}

	return out
}

func (ce *CompileTimeMessage) getText(level string) string {
	var previousLine string
	var nextLine string

	var underline *color.Color
	var normal *color.Color
	var bold *color.Color

	lineNumber := ce.Pos.Line - 1
	columnNumber := ce.Pos.Column - 1

	lines := strings.Split(*ce.Scope.Source, "\n")

	if level == "error" {
		underline = color.New(color.Underline, color.FgHiRed)
		bold = color.New(color.FgRed, color.Bold)
		normal = color.New(color.FgRed)
	} else if level == "warning" {
		underline = color.New(color.Underline, color.FgHiYellow)
		bold = color.New(color.FgYellow, color.Bold)
		normal = color.New(color.FgYellow)
	} else {
		underline = color.New(color.Underline, color.FgHiWhite)
		bold = color.New(color.FgWhite, color.Bold)
		normal = color.New(color.FgWhite)
	}

	line := lines[lineNumber]
	offendingCode := line[columnNumber:]
	unoffendingCode := line[:columnNumber]

	if lineNumber > 0 {
		previousLine = lines[lineNumber-1]
	}

	if len(lines)-lineNumber > 0 {
		nextLine = lines[lineNumber+1]
	}

	code := previousLine + "\n" + unoffendingCode + underline.Sprint(offendingCode) + "\n" + nextLine

	heading := color.HiGreenString(ce.Scope.SourceName+":"+ce.Pos.String()) + " => " + bold.Sprint(ce.Title) + normal.Sprint(": "+ce.Message)

	return heading + "\n" + addTabs(code)
}

func (c *CompileTimeMessage) GetError() string {
	return c.getText("error")
}

func (c *CompileTimeMessage) GetWarning() string {
	return c.getText("warning")
}

func (s *ErrorScope) NewCompileTimeError(title string, message string, pos lexer.Position) {
	e := &CompileTimeMessage{
		Message: message,
		Pos:     pos,
		Scope:   s,
		Title:   title,
	}

	s.CompileTimeErrors = append(s.CompileTimeErrors, e)
}

func (s *ErrorScope) NewCompileTimeWarning(title string, message string, pos lexer.Position) {
	e := &CompileTimeMessage{
		Message: message,
		Pos:     pos,
		Scope:   s,
		Title:   title,
	}

	s.CompileTimeWarnings = append(s.CompileTimeErrors, e)
}

func (s *ErrorScope) HasErrors() bool {
	return len(s.CompileTimeErrors) > 0
}

func (s *ErrorScope) HasWarnings() bool {
	return len(s.CompileTimeWarnings) > 0
}

func (e *ErrorScope) GetSummary() string {
	if len(e.CompileTimeErrors) > 0 || len(e.CompileTimeWarnings) > 0 {
		return fmt.Sprintf(
			"%s and %s generated",
			color.HiYellowString(strconv.Itoa(len(e.CompileTimeWarnings))+" warnings"),
			color.HiRedString(strconv.Itoa(len(e.CompileTimeErrors))+" errors"),
		)
	}

	return color.HiWhiteString("No warnings or errors generated")
}

func NewErrorScope(name string, sourceName string, source string) *ErrorScope {
	return &ErrorScope{
		Name:              name,
		CompileTimeErrors: make([]*CompileTimeMessage, 0),
		Source:            &source,
		SourceName:        sourceName,
	}
}
