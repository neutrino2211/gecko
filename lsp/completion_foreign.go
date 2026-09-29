package main

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

func completionReceiver(line string, dot int) string {
	start := dot
	for start > 0 {
		end := start
		for start > 0 && isIdentChar(line[start-1]) {
			start--
		}
		if start == end {
			return ""
		}
		if start == 0 || line[start-1] != '.' {
			break
		}
		start--
	}
	return line[start:dot]
}

func qualifiedCompletions(ctx *analysis.AnalysisContext, file *tokens.File, filePath, receiver, prefix string, cursorLine int) []protocol.CompletionItem {
	parts := strings.Split(receiver, ".")
	if len(parts) == 2 {
		var items []protocol.CompletionItem
		for _, imported := range importedModuleFiles(ctx, file, filePath, parts[0]) {
			items = appendDistinctCompletions(items, foreignMemberCompletions(imported, parts[1], prefix, false)...)
		}
		return items
	}
	if len(parts) != 1 {
		return nil
	}
	if receiverVariableType(ctx, file, receiver, cursorLine) != "" {
		return getMemberCompletionsWithScope(ctx, file, filePath, receiver, prefix, cursorLine)
	}
	if items := foreignMemberCompletions(file, receiver, prefix, true); len(items) > 0 || hasForeignModule(file, receiver) {
		return items
	}
	if variants := enumVariantCompletions(file, receiver, prefix); len(variants) > 0 {
		return variants
	}
	return getMemberCompletionsWithScope(ctx, file, filePath, receiver, prefix, cursorLine)
}

func receiverVariableType(ctx *analysis.AnalysisContext, file *tokens.File, name string, cursorLine int) string {
	if ctx != nil {
		if typ := ctx.VariableType(name, cursorLine, 0); typ != nil {
			return analysis.FormatTypeRef(typ)
		}
	}
	if typ := lookupVariableTypeInScope(file, name, cursorLine); typ != "" {
		return typ
	}
	if file != nil {
		for _, entry := range file.Entries {
			if entry.Field != nil && entry.Field.Name == name && entry.Field.Type != nil {
				return analysis.FormatTypeRef(entry.Field.Type)
			}
		}
	}
	return ""
}

func enumVariantCompletions(file *tokens.File, name, prefix string) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0)
	if file == nil {
		return items
	}
	for _, entry := range file.Entries {
		if entry.Enum == nil || entry.Enum.Name != name {
			continue
		}
		for _, caseName := range entry.Enum.Cases {
			if strings.HasPrefix(caseName, prefix) {
				items = append(items, protocol.CompletionItem{
					Label: caseName, Kind: protocol.CompletionItemKindEnumMember,
					Detail: name + "::" + caseName,
				})
			}
		}
	}
	return items
}

func hasForeignModule(file *tokens.File, module string) bool {
	if file == nil {
		return false
	}
	for _, entry := range file.Entries {
		if entry.Foreign != nil && entry.Foreign.Module == module {
			return true
		}
	}
	return false
}

func foreignMemberCompletions(file *tokens.File, module, prefix string, includeTypes bool) []protocol.CompletionItem {
	if file == nil {
		return nil
	}
	items := make([]protocol.CompletionItem, 0)
	for _, entry := range file.Entries {
		foreign := entry.Foreign
		if foreign == nil || foreign.Module != module {
			continue
		}
		for _, member := range foreign.Members {
			if member.Method != nil && strings.HasPrefix(member.Method.Name, prefix) {
				items = appendUnique(items, protocol.CompletionItem{
					Label: member.Method.Name, Kind: protocol.CompletionItemKindFunction,
					Detail: foreignMethodDetail(member.Method),
				})
			}
			if includeTypes && member.Type != nil && strings.HasPrefix(member.Type.Name, prefix) {
				items = appendUnique(items, protocol.CompletionItem{
					Label: member.Type.Name, Kind: protocol.CompletionItemKindClass,
					Detail: "opaque foreign type",
				})
			}
		}
	}
	return items
}

func foreignMethodDetail(method *tokens.ForeignMethod) string {
	params := make([]string, 0, len(method.Arguments)+1)
	for _, arg := range method.Arguments {
		parameter := arg.Name
		if arg.Type != nil {
			parameter += ": "
			if arg.Out {
				parameter += "out "
			}
			parameter += analysis.FormatTypeRef(arg.Type)
		}
		params = append(params, parameter)
	}
	if method.Variadic {
		params = append(params, "...")
	}
	detail := fmt.Sprintf("func %s(%s)", method.Name, strings.Join(params, ", "))
	if method.Type != nil {
		detail += ": " + analysis.FormatTypeRef(method.Type)
	}
	return detail
}

func selectedImportCompletions(ctx *analysis.AnalysisContext, file *tokens.File, filePath, prefix string) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0)
	for _, entry := range file.Entries {
		if entry.Import == nil || len(entry.Import.Objects) == 0 {
			continue
		}
		selected := make(map[string]bool, len(entry.Import.Objects))
		for _, name := range entry.Import.Objects {
			selected[name] = true
		}
		for _, item := range getImportedModuleCompletions(ctx, file, filePath, entry.Import.ModuleName(), prefix) {
			if selected[item.Label] {
				items = appendUnique(items, item)
			}
		}
	}
	return items
}

func appendDistinctCompletions(items []protocol.CompletionItem, more ...protocol.CompletionItem) []protocol.CompletionItem {
	for _, candidate := range more {
		duplicate := false
		for _, existing := range items {
			if existing.Label == candidate.Label && existing.Kind == candidate.Kind && existing.Detail == candidate.Detail {
				duplicate = true
				break
			}
		}
		if !duplicate {
			items = append(items, candidate)
		}
	}
	return items
}
