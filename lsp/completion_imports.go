// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

func importedModuleInFile(ctx *analysis.AnalysisContext, file *tokens.File, filePath, moduleName string) *tokens.File {
	if ctx != nil {
		return ctx.ImportedFiles[moduleName]
	}
	if file != nil {
		for _, imported := range file.Imports {
			if imported.Name == moduleName {
				return imported
			}
		}
	}
	var content string
	if file != nil && file.Content != "" {
		content = file.Content
	} else {
		source, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}
		content = string(source)
	}
	loaded, err := analysis.NewAnalysisContext(filePath, content)
	if err != nil {
		return nil
	}
	return loaded.ImportedFiles[moduleName]
}

func importedModuleFiles(ctx *analysis.AnalysisContext, file *tokens.File, filePath, moduleName string) []*tokens.File {
	if ctx != nil {
		return ctx.FilesForModule(moduleName)
	}
	if imported := importedModuleInFile(nil, file, filePath, moduleName); imported != nil {
		return []*tokens.File{imported}
	}
	return nil
}

func importedModuleEntries(ctx *analysis.AnalysisContext, file *tokens.File, filePath, moduleName string) []*tokens.Entry {
	var entries []*tokens.Entry
	for _, imported := range importedModuleFiles(ctx, file, filePath, moduleName) {
		entries = append(entries, imported.Entries...)
	}
	return entries
}

func importedSymbolFile(ctx *analysis.AnalysisContext, file *tokens.File, content string, line, col int, name string) *tokens.File {
	if ctx == nil || file == nil {
		return nil
	}
	if ctx.SemanticGraph != nil {
		if symbol := ctx.SemanticGraph.SymbolAt(ctx.FilePath, ctx.Offset(line+1, col+1)); symbol != nil {
			if declared := ctx.FileForSymbol(symbol.ID); declared != nil {
				return declared
			}
		}
	}
	if qualifier := memberQualifier(content, line, col); qualifier != "" {
		for _, entry := range file.Entries {
			if entry.Import != nil && entry.Import.ModuleName() == qualifier {
				for _, imported := range ctx.FilesForModule(qualifier) {
					if findHoverInFile(imported, name) != nil {
						return imported
					}
				}
			}
		}
		return nil
	}
	for _, entry := range file.Entries {
		if entry.Import == nil {
			continue
		}
		for _, selected := range entry.Import.Objects {
			if selected == name {
				for _, imported := range ctx.FilesForModule(entry.Import.ModuleName()) {
					if findHoverInFile(imported, name) != nil {
						return imported
					}
				}
			}
		}
	}
	return nil
}

func memberQualifier(content string, line, col int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	text := lines[line]
	if len(text) == 0 {
		return ""
	}
	start := col
	if start >= len(text) {
		start = len(text) - 1
	}
	for start > 0 && isIdentChar(text[start-1]) {
		start--
	}
	if start == 0 || text[start-1] != '.' {
		return ""
	}
	end := start - 1
	begin := end
	for begin > 0 && isIdentChar(text[begin-1]) {
		begin--
	}
	return text[begin:end]
}

// getImportedClassMemberCompletions returns completions for a class from an imported module
// Only returns public members since the class is from a different module
func getImportedClassMemberCompletions(filePath, moduleName, className, prefix string) []protocol.CompletionItem {
	return getImportedClassMemberCompletionsGeneric(nil, nil, filePath, moduleName, className, prefix, nil)
}

// getImportedClassMemberCompletionsGeneric returns completions with type parameter substitution for imported classes
func getImportedClassMemberCompletionsGeneric(ctx *analysis.AnalysisContext, file *tokens.File, filePath, moduleName, className, prefix string, typeArgs []string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Parse generic class name if needed
	parsedClass := parseGenericType(className)
	baseClassName := parsedClass.BaseName
	if len(parsedClass.TypeArgs) > 0 && len(typeArgs) == 0 {
		typeArgs = parsedClass.TypeArgs
	}

	entries := importedModuleEntries(ctx, file, filePath, moduleName)
	if len(entries) == 0 {
		return items
	}

	for _, entry := range entries {
		if entry.Class != nil && entry.Class.Name == baseClassName {
			// Get type parameter names for substitution
			var typeParams []string
			for _, tp := range entry.Class.TypeParams {
				typeParams = append(typeParams, tp.Name)
			}

			for _, field := range entry.Class.Fields {
				// Methods - only show public ones
				if field.Method != nil {
					name := field.Method.Name
					if strings.HasPrefix(name, prefix) && analysis.IsPublic(field.Method.Visibility) {
						detail := analysis.FormatMethodSignature(field.Method)
						// Substitute type parameters with actual type arguments
						if len(typeArgs) > 0 {
							detail = substituteTypeParams(detail, typeParams, typeArgs)
						}
						items = append(items, protocol.CompletionItem{
							Label:  name,
							Kind:   protocol.CompletionItemKindMethod,
							Detail: detail,
						})
					}
				}
				// Fields - only show public ones
				if field.Field != nil {
					name := field.Field.Name
					if strings.HasPrefix(name, prefix) && analysis.IsPublic(field.Field.Visibility) {
						typeStr := "unknown"
						if field.Field.Type != nil {
							typeStr = analysis.FormatTypeRef(field.Field.Type)
							// Substitute type parameters with actual type arguments
							if len(typeArgs) > 0 {
								typeStr = substituteTypeParams(typeStr, typeParams, typeArgs)
							}
						}
						items = append(items, protocol.CompletionItem{
							Label:  name,
							Kind:   protocol.CompletionItemKindField,
							Detail: typeStr,
						})
					}
				}
			}
		}
	}

	return items
}

// getImportedTraitMethodCompletions returns trait method completions for a class from an imported module
// Only returns public methods since the class is from a different module
func getImportedTraitMethodCompletions(ctx *analysis.AnalysisContext, file *tokens.File, filePath, moduleName, className, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	addedMethods := make(map[string]bool)

	entries := importedModuleEntries(ctx, file, filePath, moduleName)
	if len(entries) == 0 {
		return items
	}

	for _, entry := range entries {
		if entry.Implementation != nil && isImplForClass(entry.Implementation, className) {
			for _, field := range entry.Implementation.GetFields() {
				name := field.Name
				// Only show public methods from other modules
				if strings.HasPrefix(name, prefix) && !addedMethods[name] && analysis.IsPublic(field.Visibility) {
					addedMethods[name] = true
					detail := formatImplMethodSignature(field)
					// Only show trait name if this is a trait impl (not inherent)
					if entry.Implementation.GetFor() != "" && entry.Implementation.GetName() != "" {
						detail = fmt.Sprintf("(%s) %s", entry.Implementation.GetName(), detail)
					}
					items = append(items, protocol.CompletionItem{
						Label:  name,
						Kind:   protocol.CompletionItemKindMethod,
						Detail: detail,
					})
				}
			}
		}
	}

	return items
}

// getImportedModuleCompletions returns completions for an imported module's exports
func getImportedModuleCompletions(ctx *analysis.AnalysisContext, file *tokens.File, filePath, moduleName, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	entries := importedModuleEntries(ctx, file, filePath, moduleName)
	if len(entries) == 0 {
		return items
	}

	// Extract exported symbols from the module - respect visibility
	for _, entry := range entries {
		if entry.Foreign != nil && strings.HasPrefix(entry.Foreign.Module, prefix) {
			items = appendUnique(items, protocol.CompletionItem{
				Label: entry.Foreign.Module, Kind: protocol.CompletionItemKindModule,
				Detail: "foreign " + entry.Foreign.Backend,
			})
		}
		if entry.Foreign != nil {
			for _, member := range entry.Foreign.Members {
				if member.Type != nil && strings.HasPrefix(member.Type.Name, prefix) {
					items = appendUnique(items, protocol.CompletionItem{
						Label: member.Type.Name, Kind: protocol.CompletionItemKindClass,
						Detail: "opaque foreign type",
					})
				}
			}
		}
		if entry.Declaration != nil {
			if method := entry.Declaration.Method; method != nil && strings.HasPrefix(method.Name, prefix) {
				items = appendUnique(items, protocol.CompletionItem{
					Label: method.Name, Kind: protocol.CompletionItemKindFunction,
					Detail: analysis.FormatMethodSignature(method),
				})
			}
			if typ := entry.Declaration.ExternalType; typ != nil && strings.HasPrefix(typ.Name, prefix) {
				items = appendUnique(items, protocol.CompletionItem{
					Label: typ.Name, Kind: protocol.CompletionItemKindClass,
					Detail: "external type",
				})
			}
		}
		// Functions - only show public ones
		if entry.Method != nil && strings.HasPrefix(entry.Method.Name, prefix) {
			if analysis.IsPublic(entry.Method.Visibility) {
				items = append(items, protocol.CompletionItem{
					Label:  entry.Method.Name,
					Kind:   protocol.CompletionItemKindFunction,
					Detail: analysis.FormatMethodSignature(entry.Method),
				})
			}
		}

		// Classes - only show public ones
		if entry.Class != nil && strings.HasPrefix(entry.Class.Name, prefix) {
			if analysis.IsPublic(entry.Class.Visibility) {
				items = append(items, protocol.CompletionItem{
					Label:  entry.Class.Name,
					Kind:   protocol.CompletionItemKindClass,
					Detail: analysis.FormatClassType(entry.Class),
				})
			}
		}

		// Traits - only show public ones
		if entry.Trait != nil && strings.HasPrefix(entry.Trait.Name, prefix) {
			if analysis.IsPublic(entry.Trait.Visibility) {
				items = append(items, protocol.CompletionItem{
					Label:  entry.Trait.Name,
					Kind:   protocol.CompletionItemKindInterface,
					Detail: analysis.FormatTraitType(entry.Trait),
				})
			}
		}

		// Global variables - only show public ones
		if entry.Field != nil && strings.HasPrefix(entry.Field.Name, prefix) {
			if analysis.IsPublic(entry.Field.Visibility) {
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
		}

		// Enums - always public (no visibility modifier in grammar)
		if entry.Enum != nil && strings.HasPrefix(entry.Enum.Name, prefix) {
			items = append(items, protocol.CompletionItem{
				Label:  entry.Enum.Name,
				Kind:   protocol.CompletionItemKindEnum,
				Detail: fmt.Sprintf("enum %s (%d variants)", entry.Enum.Name, len(entry.Enum.Cases)),
			})
		}
	}

	return items
}
