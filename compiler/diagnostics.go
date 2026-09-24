// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md

package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/fatih/color"
	"github.com/neutrino2211/gecko/backends"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

func buildFileContentMap(sourceFile *tokens.File) map[string]string {
	out := make(map[string]string)
	visited := make(map[string]bool)

	var walk func(file *tokens.File)
	walk = func(file *tokens.File) {
		if file == nil {
			return
		}
		key := file.Path
		if key == "" {
			key = file.Name
		}
		if key == "" {
			key = fmt.Sprintf("%p", file)
		}
		if visited[key] {
			return
		}
		visited[key] = true

		if file.Path != "" && file.Content != "" {
			out[file.Path] = file.Content
		}

		for _, imported := range file.Imports {
			walk(imported)
		}
	}

	walk(sourceFile)
	return out
}

func emitSemanticDiagnostics(sourceFile *tokens.File, diags []semantic.Diagnostic) {
	if len(diags) == 0 {
		return
	}

	contentByFile := buildFileContentMap(sourceFile)
	scopes := make(map[string]*errors.ErrorScope)
	defaultPath := sourceFile.Path
	defaultContent := sourceFile.Content

	for _, diag := range diags {
		path := diag.Pos.Filename
		if path == "" {
			path = defaultPath
		}
		content := defaultContent
		if path != "" {
			if c, ok := contentByFile[path]; ok {
				content = c
			}
		}
		if content == "" {
			content = defaultContent
		}

		scopeKey := path
		if scopeKey == "" {
			scopeKey = "__semantic_default__"
		}
		scope, ok := scopes[scopeKey]
		if !ok {
			scopeName := path
			if scopeName == "" {
				scopeName = defaultPath
			}
			scope = errors.NewErrorScope("semantic", scopeName, content)
			scopes[scopeKey] = scope
		}

		message := diag.Message
		if diag.Help != "" {
			message = message + "\nhelp: " + diag.Help
		}

		title := diag.Title
		if title == "" {
			title = "Semantic Error"
		}

		if diag.Severity == semantic.SeverityWarning {
			scope.NewCompileTimeWarning(title, message, diag.Pos)
		} else {
			scope.NewCompileTimeError(title, message, diag.Pos)
		}
	}
}

func parseSyntaxError(tokenError error, compileErrorScope *errors.ErrorScope) {
	if tokenError == nil {
		return
	}
	errorMsg := tokenError.Error()
	var line, column int = 1, 1

	// Try to extract position from Participle error format: "filename:line:col: message"
	// or just "line:col: message"
	parts := strings.SplitN(errorMsg, ":", 4)
	if len(parts) >= 3 {
		// Check if first part is a number (line) or filename
		if l, err := strconv.Atoi(parts[0]); err == nil {
			line = l
			if c, err := strconv.Atoi(parts[1]); err == nil {
				column = c
			}
		} else if len(parts) >= 4 {
			// Format is "filename:line:col: message"
			if l, err := strconv.Atoi(parts[1]); err == nil {
				line = l
				if c, err := strconv.Atoi(parts[2]); err == nil {
					column = c
				}
			}
		}
	}

	compileErrorScope.NewCompileTimeError(
		"Syntax Error",
		errorMsg,
		lexer.Position{
			Line:   line,
			Column: column,
		},
	)
}

func haveErrors() bool {
	for _, e := range errors.GetAllScopes() {
		if e.HasErrors() {
			return true
		}
	}

	return false
}

// DiagnosticMessage represents an error or warning for external consumers (like LSP)
type DiagnosticMessage struct {
	Line    int
	Column  int
	Message string
	Title   string
}

// ResetErrorScopes clears all error scopes (useful for LSP between checks)
func ResetErrorScopes() {
	ResetCompilationState()
}

// ResetCompilationState clears all compiler/backend global state.
// Useful for long-lived processes (e.g. LSP) between checks.
func ResetCompilationState() {
	errors.ResetScopes()
	ResetTypeRegistry()
	LastNativeLibraries = nil
	LastNativeObjects = nil
	cbackend.ResetState()
	hooks.ResetHookRegistry()
	backends.GlobalScopeLifecycle.Reset()
}

// GetAllErrors returns all errors from all scopes
func GetAllErrors() []DiagnosticMessage {
	var result []DiagnosticMessage
	for _, scope := range errors.GetAllScopes() {
		for _, err := range scope.CompileTimeErrors {
			result = append(result, DiagnosticMessage{
				Line:    err.Pos.Line,
				Column:  err.Pos.Column,
				Message: err.Title + ": " + err.Message,
				Title:   err.Title,
			})
		}
	}
	return result
}

// GetAllWarnings returns all warnings from all scopes
func GetAllWarnings() []DiagnosticMessage {
	var result []DiagnosticMessage
	for _, scope := range errors.GetAllScopes() {
		for _, warn := range scope.CompileTimeWarnings {
			result = append(result, DiagnosticMessage{
				Line:    warn.Pos.Line,
				Column:  warn.Pos.Column,
				Message: warn.Title + ": " + warn.Message,
				Title:   warn.Title,
			})
		}
	}
	return result
}

func PrintErrorSummary() bool {
	var warnings, errCount int = 0, 0
	var bold, boldYellow, boldRed *color.Color = color.New(color.Bold), color.New(color.Bold, color.FgHiYellow), color.New(color.Bold, color.FgHiRed)
	for _, e := range errors.GetAllScopes() {
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

		errCount += len(e.CompileTimeErrors)
		warnings += len(e.CompileTimeWarnings)
	}

	fmt.Print(
		bold.Sprint("\nTotal of ") +
			boldYellow.Sprintf("%d warnings", warnings) +
			bold.Sprint(" and ") +
			boldRed.Sprintf("%d errors", errCount) +
			bold.Sprint(" generated\n"),
	)

	return errCount > 0
}
