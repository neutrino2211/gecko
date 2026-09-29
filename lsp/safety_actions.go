package main

import (
	"strings"

	"github.com/neutrino2211/gecko/parser"
	"go.lsp.dev/protocol"
)

func unsafeOperationActions(content, filePath string, diagnostics []protocol.Diagnostic) []protocol.CodeAction {
	result := make([]protocol.CodeAction, 0)
	for _, diagnostic := range diagnostics {
		if !strings.HasPrefix(diagnostic.Message, "Unsafe Required:") {
			continue
		}
		offset := sourceOffset(content, diagnostic.Range.Start)
		if offset < 0 || offset >= len(content) {
			continue
		}
		lineStart := strings.LastIndexByte(content[:offset], '\n') + 1
		lineEnd := len(content)
		if end := strings.IndexByte(content[offset:], '\n'); end >= 0 {
			lineEnd = offset + end
		}
		line := content[lineStart:lineEnd]
		indentSize := len(line) - len(strings.TrimLeft(line, " \t"))
		operation := strings.TrimSpace(line[indentSize:])
		if lineStart+indentSize != offset || !strings.HasPrefix(operation, "@") || strings.HasPrefix(operation, "@unsafe") || strings.Contains(operation, "//") || strings.ContainsAny(operation, "{}") {
			continue
		}
		replacement := line[:indentSize] + "@unsafe { " + operation + " }"
		candidate := content[:lineStart] + replacement + content[lineEnd:]
		if _, err := parser.Parser.ParseString(filePath, candidate); err != nil {
			continue
		}
		result = append(result, protocol.CodeAction{
			Title:       "Wrap operation in @unsafe",
			Kind:        protocol.QuickFix,
			Diagnostics: []protocol.Diagnostic{diagnostic},
			Edit: &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{
				pathToURI(filePath): {{
					Range:   protocol.Range{Start: sourcePosition(content, lineStart), End: sourcePosition(content, lineEnd)},
					NewText: replacement,
				}},
			}},
		})
	}
	return result
}
