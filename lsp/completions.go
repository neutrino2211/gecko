// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetCompletions returns completion items for a position. When ctx is non-nil it
// provides the shared semantic graph so member/static/symbol completions
// reflect the compiler's resolved types (including inference and imports).
func GetCompletions(ctx *analysis.AnalysisContext, content, filePath string, line, col int) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Get the text before cursor to determine context
	lines := strings.Split(content, "\n")
	var prefix string
	var lineText string
	if line < len(lines) {
		lineText = lines[line]
		if col > 0 && col <= len(lineText) {
			// Find word start
			start := col
			for start > 0 && isIdentChar(lineText[start-1]) {
				start--
			}
			prefix = lineText[start:col]
		}
	}

	var file *tokens.File
	if ctx != nil && ctx.MainFile != nil && ctx.SourceContent == content && filepath.Clean(ctx.FilePath) == filepath.Clean(filePath) {
		file = ctx.MainFile
	} else {
		// Sanitize content for parsing - remove incomplete expressions on current line
		parseLines := append([]string(nil), lines...)
		if line >= 0 && line < len(lines) {
			dot := col - len(prefix) - 1
			if dot >= 0 && dot < len(lineText) && lineText[dot] == '.' {
				if receiver := completionReceiver(lineText, dot); receiver != "" {
					parseLines[line] = lineText[:dot-len(receiver)] + "0" + strings.Repeat(" ", col-dot+len(receiver)-1) + lineText[col:]
				}
			}
		}
		sanitizedContent := sanitizeForParsing(strings.Join(parseLines, "\n"), line)
		readSource := os.ReadFile
		if ctx != nil && ctx.MainFile != nil && ctx.MainFile.Config != nil && ctx.MainFile.Config.ReadSource != nil {
			readSource = ctx.MainFile.Config.ReadSource
		}
		recovered, err := analysis.NewAnalysisContextWithReader(filePath, sanitizedContent, readSource)
		if err != nil && line >= 0 && line < len(lines) {
			maskedLines := append([]string{}, lines...)
			maskedLines[line] = strings.Repeat(" ", len(maskedLines[line]))
			recovered, err = analysis.NewAnalysisContextWithReader(filePath, strings.Join(maskedLines, "\n"), readSource)
		}
		if err == nil {
			ctx = recovered
			file = ctx.MainFile
		} else if ctx != nil && filepath.Clean(ctx.FilePath) == filepath.Clean(filePath) {
			file = ctx.MainFile
		}

		if file == nil {
			file = &tokens.File{}
		}
		file.ComputeRanges()

	}

	// Check if we're after :: (static method access)
	if line < len(lines) && col > 1 {
		colonPos := col - len(prefix) - 1
		if colonPos >= 1 && colonPos < len(lineText) && lineText[colonPos] == ':' && lineText[colonPos-1] == ':' {
			// Static method completion - find the type before ::
			typeEnd := colonPos - 1
			typeStart := typeEnd
			for typeStart > 0 && isIdentChar(lineText[typeStart-1]) {
				typeStart--
			}
			if typeStart < typeEnd {
				typeName := lineText[typeStart:typeEnd]
				items = append(items, getStaticMethodCompletions(ctx, file, typeName, prefix)...)
				return items
			}
		}
	}

	// Check if we're after a dot (member access)
	if line < len(lines) && col > 0 {
		dotPos := col - len(prefix) - 1
		if dotPos >= 0 && dotPos < len(lineText) && lineText[dotPos] == '.' {
			if receiver := completionReceiver(lineText, dotPos); receiver != "" {
				items = append(items, qualifiedCompletions(ctx, file, filePath, receiver, prefix, line+1)...)
				return items
			}
		}
	}

	// Add keywords
	for _, kw := range geckoKeywords {
		if strings.HasPrefix(kw, prefix) {
			items = append(items, protocol.CompletionItem{
				Label:  kw,
				Kind:   protocol.CompletionItemKindKeyword,
				Detail: "keyword",
			})
		}
	}

	// Add types
	for _, t := range geckoTypes {
		if strings.HasPrefix(t, prefix) {
			items = append(items, protocol.CompletionItem{
				Label:  t,
				Kind:   protocol.CompletionItemKindTypeParameter,
				Detail: "type",
			})
		}
	}

	// Add symbols from the file
	for _, entry := range file.Entries {
		items = appendDistinctCompletions(items, getEntryCompletions(entry, prefix)...)
	}
	items = appendUnique(items, selectedImportCompletions(ctx, file, filePath, prefix)...)

	// Add local variables if inside a function
	if ctx != nil && ctx.SemanticGraph != nil {
		for _, symbol := range ctx.SemanticGraph.VisibleSymbols(ctx.FilePath, ctx.Offset(line+1, col+1)) {
			if symbol.Local && strings.HasPrefix(symbol.Name, prefix) {
				detail := analysis.FormatTypeRef(symbol.Type)
				if symbol.Parameter {
					detail = "(parameter) " + detail
				}
				items = appendUnique(items, protocol.CompletionItem{Label: symbol.Name, Kind: protocol.CompletionItemKindVariable, Detail: detail})
			}
		}
	} else {
		enclosingMethod := findEnclosingMethod(file, line+1)
		items = append(items, getLocalCompletions(enclosingMethod, prefix, line+1, ctx)...)
	}

	// Add stdlib suggestions if prefix looks like a type name (starts with uppercase)
	if len(prefix) > 0 && prefix[0] >= 'A' && prefix[0] <= 'Z' {
		items = append(items, getStdlibCompletions(file, prefix)...)
	}

	return items
}

// getStdlibCompletions returns completions for stdlib types that aren't imported
func getStdlibCompletions(file *tokens.File, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Get already imported modules
	importedModules := make(map[string]bool)
	for _, entry := range file.Entries {
		if entry.Import != nil {
			importedModules[entry.Import.Package()] = true
			// Also check for selective imports
			for _, obj := range entry.Import.Objects {
				importedModules[obj] = true
			}
		}
	}

	// Search stdlib for matching exports
	stdlibIndex := GetStdlibIndex()
	exports := stdlibIndex.FindByPrefix(prefix)

	for _, export := range exports {
		// Skip if already imported
		if importedModules[export.ModulePath] || importedModules[export.Name] {
			continue
		}

		kind := protocol.CompletionItemKindClass
		switch export.Kind {
		case "trait":
			kind = protocol.CompletionItemKindInterface
		case "func":
			kind = protocol.CompletionItemKindFunction
		case "const":
			kind = protocol.CompletionItemKindConstant
		case "enum":
			kind = protocol.CompletionItemKindEnum
		}

		items = append(items, protocol.CompletionItem{
			Label:      export.Name,
			Kind:       kind,
			Detail:     fmt.Sprintf("%s (import %s)", export.Kind, export.ModulePath),
			InsertText: export.Name,
			// Additional edit to add the import would go here with CommitCharacters
		})
	}

	return items
}

// sanitizeForParsing removes incomplete expressions to allow parsing partial code
func sanitizeForParsing(content string, cursorLine int) string {
	lines := strings.Split(content, "\n")
	if cursorLine < 0 || cursorLine >= len(lines) {
		return content
	}

	// Check if the cursor line has incomplete expression
	lineText := strings.TrimRight(lines[cursorLine], " \t")
	if strings.HasSuffix(lineText, ".") || strings.HasSuffix(lineText, "::") ||
		strings.HasSuffix(lineText, "(") || strings.HasSuffix(lineText, ",") {
		// Remove the incomplete line for parsing
		lines[cursorLine] = strings.Repeat(" ", len(lines[cursorLine]))
	}

	return strings.Join(lines, "\n")
}

// formatFunctionSignature formats a resolved FunctionSignature for display.
func formatFunctionSignature(sig *semantic.FunctionSignature) string {
	if sig == nil {
		return ""
	}
	parts := make([]string, 0, len(sig.Params))
	for _, p := range sig.Params {
		typeStr := "unknown"
		if p.Type != nil {
			typeStr = analysis.FormatTypeRef(p.Type)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", p.Name, typeStr))
	}
	retStr := "void"
	if sig.ReturnType != nil {
		retStr = analysis.FormatTypeRef(sig.ReturnType)
	}
	return fmt.Sprintf("%s(%s): %s", sig.Name, strings.Join(parts, ", "), retStr)
}

// appendUnique appends completion items, skipping labels already present.
func appendUnique(items []protocol.CompletionItem, more ...protocol.CompletionItem) []protocol.CompletionItem {
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		seen[it.Label] = true
	}
	for _, it := range more {
		if seen[it.Label] {
			continue
		}
		seen[it.Label] = true
		items = append(items, it)
	}
	return items
}

func getEntryCompletions(entry *tokens.Entry, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	if entry.Class != nil && strings.HasPrefix(entry.Class.Name, prefix) {
		items = append(items, protocol.CompletionItem{
			Label:  entry.Class.Name,
			Kind:   protocol.CompletionItemKindClass,
			Detail: analysis.FormatClassType(entry.Class),
		})
	}

	if entry.Trait != nil && strings.HasPrefix(entry.Trait.Name, prefix) {
		items = append(items, protocol.CompletionItem{
			Label:  entry.Trait.Name,
			Kind:   protocol.CompletionItemKindInterface,
			Detail: analysis.FormatTraitType(entry.Trait),
		})
	}

	if entry.Method != nil && strings.HasPrefix(entry.Method.Name, prefix) {
		items = append(items, protocol.CompletionItem{
			Label:  entry.Method.Name,
			Kind:   protocol.CompletionItemKindFunction,
			Detail: analysis.FormatMethodSignature(entry.Method),
		})
	}

	if entry.Field != nil && strings.HasPrefix(entry.Field.Name, prefix) {
		typeStr := "unknown"
		if entry.Field.Type != nil {
			typeStr = analysis.FormatTypeRef(entry.Field.Type)
		}
		items = append(items, protocol.CompletionItem{
			Label:  entry.Field.Name,
			Kind:   protocol.CompletionItemKindVariable,
			Detail: typeStr,
		})
	}

	if entry.Enum != nil && strings.HasPrefix(entry.Enum.Name, prefix) {
		items = append(items, protocol.CompletionItem{
			Label:  entry.Enum.Name,
			Kind:   protocol.CompletionItemKindEnum,
			Detail: fmt.Sprintf("enum %s (%d variants)", entry.Enum.Name, len(entry.Enum.Cases)),
		})
	}
	if entry.Import != nil && strings.HasPrefix(entry.Import.ModuleName(), prefix) {
		items = append(items, protocol.CompletionItem{
			Label:  entry.Import.ModuleName(),
			Kind:   protocol.CompletionItemKindModule,
			Detail: entry.Import.Package(),
		})
	}
	if entry.Foreign != nil && strings.HasPrefix(entry.Foreign.Module, prefix) {
		items = append(items, protocol.CompletionItem{
			Label:  entry.Foreign.Module,
			Kind:   protocol.CompletionItemKindModule,
			Detail: "foreign " + entry.Foreign.Backend,
		})
	}
	if entry.Foreign != nil {
		for _, member := range entry.Foreign.Members {
			if member.Type != nil && strings.HasPrefix(member.Type.Name, prefix) {
				items = append(items, protocol.CompletionItem{
					Label: member.Type.Name, Kind: protocol.CompletionItemKindClass,
					Detail: "opaque foreign type",
				})
			}
		}
	}
	if entry.Declaration != nil {
		if method := entry.Declaration.Method; method != nil && strings.HasPrefix(method.Name, prefix) {
			items = append(items, protocol.CompletionItem{
				Label: method.Name, Kind: protocol.CompletionItemKindFunction,
				Detail: analysis.FormatMethodSignature(method),
			})
		}
		if typ := entry.Declaration.ExternalType; typ != nil && strings.HasPrefix(typ.Name, prefix) {
			items = append(items, protocol.CompletionItem{
				Label: typ.Name, Kind: protocol.CompletionItemKindClass, Detail: "external type",
			})
		}
	}

	return items
}
