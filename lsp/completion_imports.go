// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/parser"
	"go.lsp.dev/protocol"
)

// resolveModuleFile resolves a module name to its file path
func resolveModuleFile(filePath, moduleName string) string {
	importPath := moduleName
	if sourceContents, err := os.ReadFile(filePath); err == nil {
		if sourceFile, parseErr := parser.Parser.ParseString(filePath, string(sourceContents)); parseErr == nil {
			for _, entry := range sourceFile.Entries {
				if entry.Import != nil && entry.Import.ModuleName() == moduleName {
					importPath = entry.Import.Package()
					break
				}
			}
		}
	}

	location := compiler.ResolveImportLocation(filePath, importPath, nil)
	if location.FilePath != "" {
		return location.FilePath
	}
	return ""
}

// getImportedClassMemberCompletions returns completions for a class from an imported module
// Only returns public members since the class is from a different module
func getImportedClassMemberCompletions(filePath, moduleName, className, prefix string) []protocol.CompletionItem {
	return getImportedClassMemberCompletionsGeneric(filePath, moduleName, className, prefix, nil)
}

// getImportedClassMemberCompletionsGeneric returns completions with type parameter substitution for imported classes
func getImportedClassMemberCompletionsGeneric(filePath, moduleName, className, prefix string, typeArgs []string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Parse generic class name if needed
	parsedClass := parseGenericType(className)
	baseClassName := parsedClass.BaseName
	if len(parsedClass.TypeArgs) > 0 && len(typeArgs) == 0 {
		typeArgs = parsedClass.TypeArgs
	}

	modulePath := resolveModuleFile(filePath, moduleName)
	if modulePath == "" {
		return items
	}

	moduleContents, err := os.ReadFile(modulePath)
	if err != nil {
		return items
	}

	moduleFile, err := parser.Parser.ParseString(modulePath, string(moduleContents))
	if err != nil {
		return items
	}

	for _, entry := range moduleFile.Entries {
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
func getImportedTraitMethodCompletions(filePath, moduleName, className, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	addedMethods := make(map[string]bool)

	modulePath := resolveModuleFile(filePath, moduleName)
	if modulePath == "" {
		return items
	}

	moduleContents, err := os.ReadFile(modulePath)
	if err != nil {
		return items
	}

	moduleFile, err := parser.Parser.ParseString(modulePath, string(moduleContents))
	if err != nil {
		return items
	}

	for _, entry := range moduleFile.Entries {
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
func getImportedModuleCompletions(filePath, moduleName, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	modulePath := resolveModuleFile(filePath, moduleName)
	if modulePath == "" {
		return items
	}

	moduleContents, err := os.ReadFile(modulePath)
	if err != nil {
		return items
	}

	moduleFile, err := parser.Parser.ParseString(modulePath, string(moduleContents))
	if err != nil {
		return items
	}

	// Extract exported symbols from the module - respect visibility
	for _, entry := range moduleFile.Entries {
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
