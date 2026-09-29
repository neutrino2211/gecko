// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	geckoerrors "github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetCodeActions returns code actions for the given range and diagnostics
func GetCodeActions(content, filePath string, rng protocol.Range, diagnostics []protocol.Diagnostic) []protocol.CodeAction {
	actions := suggestedFixActions(content, filePath, diagnostics)
	actions = append(actions, unsafeOperationActions(content, filePath, diagnostics)...)

	// Parse the file to find import insertion point
	file, err := parser.Parser.ParseString(filePath, content)
	if err != nil {
		return actions
	}
	actions = append(actions, legacyForeignActions(content, filePath, diagnostics, file)...)
	if action := organizeImportAction(content, filePath, file); action != nil {
		actions = append(actions, *action)
	}

	// Find the line after the last import (or after package declaration)
	importInsertLine := findImportInsertionLine(content, file)

	// Convert file path to URI
	docURI := pathToURI(filePath)

	// Get stdlib index
	stdlibIndex := GetStdlibIndex()

	// Look for diagnostics about unresolved types
	for _, diag := range diagnostics {
		if isUnresolvedTypeDiagnostic(diag) {
			typeName := extractTypeNameFromDiagnostic(diag, content)
			if typeName == "" {
				continue
			}

			// Search stdlib for this type
			exports := stdlibIndex.FindByName(typeName)

			for _, export := range exports {
				action := createImportAction(export, importInsertLine, diag, docURI)
				actions = append(actions, action)
			}
		}
	}

	return actions
}

func suggestedFixActions(content, filePath string, diagnostics []protocol.Diagnostic) []protocol.CodeAction {
	actions := make([]protocol.CodeAction, 0)
	for _, diagnostic := range diagnostics {
		if diagnostic.Data == nil {
			continue
		}
		raw, err := json.Marshal(diagnostic.Data)
		if err != nil {
			continue
		}
		var fixes []geckoerrors.SuggestedFix
		if err := json.Unmarshal(raw, &fixes); err != nil {
			continue
		}
		for _, fix := range fixes {
			path := fix.Span.File
			if path == "" {
				path = filePath
			}
			if filepath.Clean(path) != filepath.Clean(filePath) || fix.Title == "" || fix.Span.Start < 0 || fix.Span.End < fix.Span.Start || fix.Span.End > len(content) {
				continue
			}
			actions = append(actions, protocol.CodeAction{
				Title:       fix.Title,
				Kind:        protocol.QuickFix,
				Diagnostics: []protocol.Diagnostic{diagnostic},
				Edit: &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{
					pathToURI(filePath): {{
						Range:   protocol.Range{Start: sourcePosition(content, fix.Span.Start), End: sourcePosition(content, fix.Span.End)},
						NewText: fix.NewText,
					}},
				}},
			})
		}
	}
	return actions
}

func legacyForeignActions(content, filePath string, diagnostics []protocol.Diagnostic, file *tokens.File) []protocol.CodeAction {
	module := foreignMigrationModule(content, file)
	actions := make([]protocol.CodeAction, 0)
	for _, diagnostic := range diagnostics {
		if !strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
			continue
		}
		line := int(diagnostic.Range.Start.Line)
		var declaration *tokens.Declaration
		for _, entry := range file.Entries {
			if entry.Declaration != nil && entry.Declaration.Pos.Line == line+1 {
				declaration = entry.Declaration
				break
			}
		}
		if declaration == nil {
			continue
		}
		span, ok := foreignDeclarationSpan(content, declaration)
		if !ok {
			continue
		}
		member, wrapper, ok := foreignDeclarationParts(content, span, declaration, module)
		if !ok || declaration.Method != nil && wrapper == "" {
			continue
		}
		replacement := foreignBlock(span.indent, module, []string{member}, wrapper)
		if strings.Contains(content, "\r\n") {
			replacement = strings.ReplaceAll(replacement, "\n", "\r\n")
		}
		candidate := content[:span.start] + replacement + content[span.end:]
		if _, err := parser.Parser.ParseString(filePath, candidate); err != nil {
			continue
		}
		actions = append(actions, protocol.CodeAction{
			Title:       "Convert declaration to foreign C module",
			Kind:        protocol.QuickFix,
			Diagnostics: []protocol.Diagnostic{diagnostic},
			Edit: &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{
				pathToURI(filePath): {{
					Range: protocol.Range{
						Start: sourcePosition(content, span.start),
						End:   sourcePosition(content, span.end),
					},
					NewText: replacement,
				}},
			}},
		})
	}
	return actions
}

func foreignMigrationModule(content string, file *tokens.File) string {
	for _, entry := range file.Entries {
		if entry.Foreign != nil && entry.Foreign.Module == "c_abi" && entry.Foreign.Backend == "\"c\"" && len(entry.Foreign.GetWithHeaders()) == 0 {
			return "c_abi"
		}
	}
	if !hasIdentifier(content, "c_abi") {
		return "c_abi"
	}
	for suffix := 2; ; suffix++ {
		name := fmt.Sprintf("c_abi_%d", suffix)
		if !hasIdentifier(content, name) {
			return name
		}
	}
}

// findImportInsertionLine finds the line number where a new import should be inserted
func findImportInsertionLine(content string, file *tokens.File) int {
	lastImportLine := 0

	// Find the last import statement
	for _, entry := range file.Entries {
		if entry.Import != nil && entry.Import.Pos.Line > lastImportLine {
			lastImportLine = entry.Import.Pos.Line
		}
	}

	// If we found imports, insert after the last one
	if lastImportLine > 0 {
		return lastImportLine
	}

	// Otherwise, find the package declaration line
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			return i + 1 // Insert after package line (0-indexed, so +1)
		}
	}

	return 0
}

// isUnresolvedTypeDiagnostic checks if a diagnostic indicates an unresolved type
func isUnresolvedTypeDiagnostic(diag protocol.Diagnostic) bool {
	msg := strings.ToLower(diag.Message)
	return strings.Contains(msg, "unknown type") ||
		strings.Contains(msg, "unresolved type") ||
		strings.Contains(msg, "undefined type") ||
		strings.Contains(msg, "not defined") ||
		strings.Contains(msg, "could not resolve")
}

// extractTypeNameFromDiagnostic extracts the type name from a diagnostic message or position
func extractTypeNameFromDiagnostic(diag protocol.Diagnostic, content string) string {
	// Try to extract from the diagnostic message
	// Common patterns: "unknown type 'Vec'", "type 'String' not defined"
	msg := diag.Message

	// Pattern: 'TypeName'
	start := strings.Index(msg, "'")
	if start != -1 {
		end := strings.Index(msg[start+1:], "'")
		if end != -1 {
			return msg[start+1 : start+1+end]
		}
	}

	// Pattern: "TypeName"
	start = strings.Index(msg, "\"")
	if start != -1 {
		end := strings.Index(msg[start+1:], "\"")
		if end != -1 {
			return msg[start+1 : start+1+end]
		}
	}

	// Fall back to extracting word at diagnostic position
	lines := strings.Split(content, "\n")
	lineIdx := int(diag.Range.Start.Line)
	if lineIdx < 0 || lineIdx >= len(lines) {
		return ""
	}
	lineText := lines[lineIdx]
	col := int(diag.Range.Start.Character)
	if col < 0 || col >= len(lineText) {
		return ""
	}

	// Find word at position
	start = col
	for start > 0 && isIdentChar(lineText[start-1]) {
		start--
	}
	end := col
	for end < len(lineText) && isIdentChar(lineText[end]) {
		end++
	}
	if start < end {
		return lineText[start:end]
	}

	return ""
}

// createImportAction creates a code action to add an import
func createImportAction(export StdlibExport, insertLine int, diag protocol.Diagnostic, docURI protocol.DocumentURI) protocol.CodeAction {
	importText := fmt.Sprintf("import %s\n", export.UsePath)

	return protocol.CodeAction{
		Title:       fmt.Sprintf("Import '%s' from %s", export.Name, export.ModulePath),
		Kind:        protocol.QuickFix,
		Diagnostics: []protocol.Diagnostic{diag},
		Edit: &protocol.WorkspaceEdit{
			Changes: map[protocol.DocumentURI][]protocol.TextEdit{
				docURI: {
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(insertLine), Character: 0},
							End:   protocol.Position{Line: uint32(insertLine), Character: 0},
						},
						NewText: importText,
					},
				},
			},
		},
	}
}
