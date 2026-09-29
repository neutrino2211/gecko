// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md

package compiler

import (
	stdErrors "errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
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

func emitSemanticDiagnostics(sourceFile *tokens.File, diags []semantic.Diagnostic, diagnostics *errors.Collector) {
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
			scope = diagnostics.NewScope("semantic", scopeName, content)
			scopes[scopeKey] = scope
		}

		title := diag.Title
		if title == "" {
			title = "Semantic Error"
		}

		var emitted *errors.CompileTimeMessage
		if diag.Severity == semantic.SeverityWarning {
			emitted = scope.NewCompileTimeWarning(title, diag.Message, diag.Pos)
		} else {
			emitted = scope.NewCompileTimeError(title, diag.Message, diag.Pos)
		}
		emitted.Help = diag.Help
		emitted.Code = diag.Code
		emitted.EndOffset = diag.EndOffset
		emitted.Related = append([]errors.RelatedLocation(nil), diag.Related...)
		emitted.Fixes = append([]errors.SuggestedFix(nil), diag.Fixes...)
	}
}

func parseSyntaxError(tokenError error, compileErrorScope *errors.ErrorScope) {
	if tokenError == nil {
		return
	}
	errorMsg := tokenError.Error()
	var parseErr participle.Error
	if stdErrors.As(tokenError, &parseErr) {
		compileErrorScope.NewCompileTimeError("Syntax Error", errorMsg, parseErr.Position())
		return
	}
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

// DiagnosticMessage represents an error or warning for external consumers (like LSP)
type DiagnosticMessage struct {
	Line      int
	Column    int
	Offset    int
	EndOffset int
	Message   string
	Title     string
	File      string
	Code      string
	Related   []errors.RelatedLocation
	Fixes     []errors.SuggestedFix
}

// ResetCompilationState clears all compiler/backend global state.
// Useful for long-lived processes (e.g. LSP) between checks.
func ResetCompilationState() {
	ResetTypeRegistry()
	LastNativeLibraries = nil
	LastNativeObjects = nil
	cbackend.ResetState()
	hooks.ResetHookRegistry()
	backends.GlobalScopeLifecycle.Reset()
}

func GetAllErrors(scopes []*errors.ErrorScope) []DiagnosticMessage {
	var result []DiagnosticMessage
	for _, scope := range scopes {
		for _, err := range scope.CompileTimeErrors {
			result = append(result, DiagnosticMessage{
				Line:      err.Pos.Line,
				Column:    err.Pos.Column,
				Offset:    err.Pos.Offset,
				EndOffset: err.EndOffset,
				Message:   diagnosticMessageText(err),
				Title:     err.Title,
				File:      scope.SourceName,
				Code:      err.Code,
				Related:   append([]errors.RelatedLocation(nil), err.Related...),
				Fixes:     append([]errors.SuggestedFix(nil), err.Fixes...),
			})
		}
	}
	return result
}

func GetAllWarnings(scopes []*errors.ErrorScope) []DiagnosticMessage {
	var result []DiagnosticMessage
	for _, scope := range scopes {
		for _, warn := range scope.CompileTimeWarnings {
			result = append(result, DiagnosticMessage{
				Line:      warn.Pos.Line,
				Column:    warn.Pos.Column,
				Offset:    warn.Pos.Offset,
				EndOffset: warn.EndOffset,
				Message:   diagnosticMessageText(warn),
				Title:     warn.Title,
				File:      scope.SourceName,
				Code:      warn.Code,
				Related:   append([]errors.RelatedLocation(nil), warn.Related...),
				Fixes:     append([]errors.SuggestedFix(nil), warn.Fixes...),
			})
		}
	}
	return result
}

func diagnosticMessageText(message *errors.CompileTimeMessage) string {
	text := message.Title + ": " + message.Message
	if message.Help != "" {
		text += "\nhelp: " + message.Help
	}
	return text
}

func PrintErrorSummary(scopes []*errors.ErrorScope) bool {
	var warnings, errCount int = 0, 0
	var bold, boldYellow, boldRed *color.Color = color.New(color.Bold), color.New(color.Bold, color.FgHiYellow), color.New(color.Bold, color.FgHiRed)
	for _, e := range scopes {
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
