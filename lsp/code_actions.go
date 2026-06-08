// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetCodeActions returns code actions for the given range and diagnostics
func GetCodeActions(content, filePath string, rng protocol.Range, diagnostics []protocol.Diagnostic) []protocol.CodeAction {
	var actions []protocol.CodeAction

	// Parse the file to find import insertion point
	file, _ := parser.Parser.ParseString(filePath, content)
	if file == nil {
		return actions
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
