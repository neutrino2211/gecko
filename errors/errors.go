// spec: spec/modules.md, spec/scoping.md

package errors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/fatih/color"
)

type DiagnosticSeverity int

const (
	SeverityError   DiagnosticSeverity = iota
	SeverityWarning
	SeverityNote
	SeverityHelp
)

func (d DiagnosticSeverity) String() string {
	switch d {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityNote:
		return "note"
	case SeverityHelp:
		return "help"
	default:
		return "unknown"
	}
}

// Diagnostic error codes for machine-readable error identification
const (
	// Expression errors
	CodeIncDecAsExpr = "E0001"

	// Type errors
	CodeTypeMismatch     = "E0010"
	CodeIncompatibleType = "E0011"
	CodeCannotInfer      = "E0012"

	// Name resolution errors
	CodeUndefinedSymbol   = "E0020"
	CodeUndefinedType     = "E0021"
	CodeUndefinedFunction = "E0022"

	// Control flow errors
	CodeMissingReturn     = "E0030"
	CodeUnreachableCode   = "E0031"
	CodeNonExhaustiveMatch = "E0032"

	// Declaration errors
	CodeRedefinition      = "E0040"
	CodeInvalidModifier   = "E0041"
	CodeMissingFieldInit  = "E0042"

	// Import errors
	CodeImportNotFound    = "E0050"
	CodeCircularImport    = "E0051"

	// Semantic warnings (not errors)
	CodeUnusedVariable    = "W0001"
	CodeDeprecatedSyntax  = "W0002"
	CodeUnreachableBranch = "W0003"
)

type CompileTimeMessage struct {
	Message  string
	Scope    *ErrorScope
	Title    string
	Pos      lexer.Position
	Severity DiagnosticSeverity
	Code     string
	Help     string
	Notes    []string
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

	var lineNumber, columnNumber = 0, 0

	if ce.Pos.Line > 0 {
		lineNumber = ce.Pos.Line - 1
	}

	if ce.Pos.Column > 0 {
		columnNumber = ce.Pos.Column - 1
	}

	if ce.Scope == nil || ce.Scope.Source == nil {
		return ce.Title + ": " + ce.Message
	}

	lines := strings.Split(*ce.Scope.Source, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	if lineNumber < 0 {
		lineNumber = 0
	}
	if lineNumber >= len(lines) {
		lineNumber = len(lines) - 1
	}

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
	if columnNumber < 0 {
		columnNumber = 0
	}
	if columnNumber > len(line) {
		columnNumber = len(line)
	}
	offendingCode := line[columnNumber:]
	unoffendingCode := line[:columnNumber]

	if lineNumber > 0 {
		previousLine = lines[lineNumber-1]
	}

	if len(lines) > lineNumber+1 {
		nextLine = lines[lineNumber+1]
	}

	code := previousLine + "\n" + unoffendingCode + underline.Sprint(offendingCode) + "\n" + nextLine

	heading := color.HiGreenString(ce.Scope.SourceName+":"+ce.Pos.String()) + " => " + bold.Sprint(ce.Title) + normal.Sprint(": "+ce.Message)

	if ce.Code != "" {
		heading += color.HiWhiteString(fmt.Sprintf(" [%s]", ce.Code))
	}

	result := heading + "\n" + addTabs(code)

	for _, note := range ce.Notes {
		result += color.HiCyanString("    = note: ") + note + "\n"
	}

	if ce.Help != "" {
		result += color.HiCyanString("    = help: ") + ce.Help + "\n"
	}

	return result
}

func (c *CompileTimeMessage) GetError() string {
	return c.getText("error")
}

func (c *CompileTimeMessage) GetWarning() string {
	return c.getText("warning")
}

func (s *ErrorScope) NewCompileTimeError(title string, message string, pos lexer.Position) {
	e := &CompileTimeMessage{
		Message:  message,
		Pos:      pos,
		Scope:    s,
		Title:    title,
		Severity: SeverityError,
	}

	s.CompileTimeErrors = append(s.CompileTimeErrors, e)
}

func (s *ErrorScope) NewCompileTimeWarning(title string, message string, pos lexer.Position) {
	e := &CompileTimeMessage{
		Message:  message,
		Pos:      pos,
		Scope:    s,
		Title:    title,
		Severity: SeverityWarning,
	}

	s.CompileTimeWarnings = append(s.CompileTimeWarnings, e)
}

// NewError creates an error with an error code and optional help text
func (s *ErrorScope) NewError(code string, title string, message string, pos lexer.Position, help ...string) {
	e := &CompileTimeMessage{
		Message:  message,
		Pos:      pos,
		Scope:    s,
		Title:    title,
		Severity: SeverityError,
		Code:     code,
	}
	if len(help) > 0 {
		e.Help = help[0]
	}
	s.CompileTimeErrors = append(s.CompileTimeErrors, e)
}

// NewWarning creates a warning with an error code and optional help text
func (s *ErrorScope) NewWarning(code string, title string, message string, pos lexer.Position, help ...string) {
	e := &CompileTimeMessage{
		Message:  message,
		Pos:      pos,
		Scope:    s,
		Title:    title,
		Severity: SeverityWarning,
		Code:     code,
	}
	if len(help) > 0 {
		e.Help = help[0]
	}
	s.CompileTimeWarnings = append(s.CompileTimeWarnings, e)
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

	return color.HiWhiteString(e.SourceName + ": No warnings or errors generated")
}

// Global registry of all error scopes for centralized warning/error collection
var allScopes []*ErrorScope

// RegisterScope adds an ErrorScope to the global registry for later printing
func RegisterScope(scope *ErrorScope) {
	allScopes = append(allScopes, scope)
}

// GetAllScopes returns all registered error scopes
func GetAllScopes() []*ErrorScope {
	return allScopes
}

// ResetScopes clears the global scope registry
func ResetScopes() {
	allScopes = make([]*ErrorScope, 0)
}

func NewErrorScope(name string, sourceName string, source string) *ErrorScope {
	scope := &ErrorScope{
		Name:                name,
		CompileTimeErrors:   make([]*CompileTimeMessage, 0),
		CompileTimeWarnings: make([]*CompileTimeMessage, 0),
		Source:              &source,
		SourceName:          sourceName,
	}
	// Auto-register for centralized collection
	RegisterScope(scope)
	return scope
}
