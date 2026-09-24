// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// getStaticMethodCompletions returns static method completions for a type (Type::)
func getStaticMethodCompletions(ctx *analysis.AnalysisContext, file *tokens.File, typeName, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Prefer the shared semantic graph: it is the compiler's authoritative view
	// of the type's methods and enum variants, including those defined in
	// imported modules.
	if ctx != nil && ctx.SemanticGraph != nil {
		for _, sig := range ctx.SemanticGraph.MethodsOfType(typeName) {
			if strings.HasPrefix(sig.Name, prefix) {
				items = append(items, protocol.CompletionItem{
					Label:  sig.Name,
					Kind:   protocol.CompletionItemKindMethod,
					Detail: formatFunctionSignature(sig),
				})
			}
		}
		if ci := ctx.SemanticGraph.Class(typeName); ci != nil {
			for fname := range ci.Fields {
				if strings.HasPrefix(fname, prefix) {
					items = append(items, protocol.CompletionItem{
						Label:  fname,
						Kind:   protocol.CompletionItemKindEnumMember,
						Detail: typeName + "::" + fname,
					})
				}
			}
		}
		return items
	}

	for _, entry := range file.Entries {
		// Check for class static methods
		if entry.Class != nil && entry.Class.Name == typeName {
			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					name := field.Method.Name
					if strings.HasPrefix(name, prefix) {
						items = append(items, protocol.CompletionItem{
							Label:  name,
							Kind:   protocol.CompletionItemKindMethod,
							Detail: analysis.FormatMethodSignature(field.Method),
						})
					}
				}
			}
		}

		// Check for enum variants
		if entry.Enum != nil && entry.Enum.Name == typeName {
			for _, caseName := range entry.Enum.Cases {
				if strings.HasPrefix(caseName, prefix) {
					items = append(items, protocol.CompletionItem{
						Label:  caseName,
						Kind:   protocol.CompletionItemKindEnumMember,
						Detail: typeName + "::" + caseName,
					})
				}
			}
		}
	}

	return items
}

// getMemberCompletionsWithScope resolves variable type from local scope and returns member completions
func getMemberCompletionsWithScope(ctx *analysis.AnalysisContext, file *tokens.File, filePath, objName, prefix string, cursorLine int) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Check if objName is an imported module
	for _, entry := range file.Entries {
		if entry.Import != nil && entry.Import.ModuleName() == objName {
			items = append(items, getImportedModuleCompletions(filePath, objName, prefix)...)
			return items
		}
	}

	// Resolve the receiver type. Prefer the shared semantic graph so inference
	// and imported types match the compiler.
	typeName := ""
	if ctx != nil {
		if t := ctx.VariableType(objName, cursorLine, 0); t != nil {
			typeName = analysis.FormatTypeRef(t)
		}
	}
	if typeName == "" {
		typeName = lookupVariableTypeInScope(file, objName, cursorLine)
	}
	if typeName == "" {
		typeName = lookupVariableType(file, objName)
	}
	if typeName == "" {
		// Maybe it's a class name directly
		typeName = objName
	}

	// Remove pointer/non-null suffix for class lookup
	baseType := strings.TrimSuffix(typeName, "*")
	baseType = strings.TrimSuffix(baseType, "!")

	parsedType := parseGenericType(baseType)

	// Use the shared semantic graph as the authoritative member source when available.
	if ctx != nil && ctx.SemanticGraph != nil {
		if ci := ctx.SemanticGraph.Class(parsedType.BaseName); ci != nil {
			for fname, ftype := range ci.Fields {
				if strings.HasPrefix(fname, prefix) {
					detail := analysis.FormatTypeRef(ftype)
					items = appendUnique(items, protocol.CompletionItem{
						Label:  fname,
						Kind:   protocol.CompletionItemKindField,
						Detail: detail,
					})
				}
			}
		}
		for _, sig := range ctx.SemanticGraph.MethodsOfType(parsedType.BaseName) {
			if strings.HasPrefix(sig.Name, prefix) {
				items = appendUnique(items, protocol.CompletionItem{
					Label:  sig.Name,
					Kind:   protocol.CompletionItemKindMethod,
					Detail: formatFunctionSignature(sig),
				})
			}
		}
	}

	// Check if the type is module-qualified (e.g., "shapes.Circle")
	if strings.Contains(parsedType.BaseName, ".") {
		parts := strings.SplitN(parsedType.BaseName, ".", 2)
		moduleName := parts[0]
		className := parts[1]

		items = append(items, getImportedClassMemberCompletionsGeneric(filePath, moduleName, className, prefix, parsedType.TypeArgs)...)
		items = append(items, getImportedTraitMethodCompletions(filePath, moduleName, className, prefix)...)
		return items
	}

	// Fall back to file-based member/trait completion (covers trait methods the
	// semantic graph does not key directly under the class).
	items = append(items, getClassMemberCompletionsGeneric(file, parsedType.BaseName, prefix, parsedType.TypeArgs)...)
	items = append(items, getTraitMethodCompletions(file, parsedType.BaseName, prefix, parsedType.TypeArgs)...)

	return items
}

// getClassMemberCompletions returns completions for a class's fields and methods
func getClassMemberCompletions(file *tokens.File, className, prefix string) []protocol.CompletionItem {
	return getClassMemberCompletionsGeneric(file, className, prefix, nil)
}

// getClassMemberCompletionsGeneric returns completions with type parameter substitution
func getClassMemberCompletionsGeneric(file *tokens.File, className, prefix string, typeArgs []string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	for _, entry := range file.Entries {
		if entry.Class != nil && entry.Class.Name == className {
			// Get type parameter names for substitution
			var typeParams []string
			for _, tp := range entry.Class.TypeParams {
				typeParams = append(typeParams, tp.Name)
			}

			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					name := field.Method.Name
					if strings.HasPrefix(name, prefix) {
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
				if field.Field != nil {
					name := field.Field.Name
					if strings.HasPrefix(name, prefix) {
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

// isImplForClass checks if an implementation is for a given class name.
// Handles both trait impls (impl Trait for Class) and inherent impls (impl Class).
func isImplForClass(impl *tokens.Implementation, className string) bool {
	// Trait impl: impl Trait for Class
	if impl.GetFor() == className {
		return true
	}
	// Inherent impl: impl Class (no 'for' clause, Name is the class name)
	if impl.GetFor() == "" && impl.GetName() == className {
		return true
	}
	return false
}

func buildTraitMap(file *tokens.File) map[string]*tokens.Trait {
	traits := make(map[string]*tokens.Trait)
	if file == nil {
		return traits
	}

	for _, entry := range file.Entries {
		if entry.Trait != nil {
			traits[entry.Trait.Name] = entry.Trait
		}
	}

	return traits
}

func collectTraitFieldsForCompletions(traitName string, traits map[string]*tokens.Trait, visiting map[string]bool) []*tokens.ImplementationField {
	if traitName == "" {
		return nil
	}
	if visiting[traitName] {
		return nil
	}

	trait, ok := traits[traitName]
	if !ok || trait == nil {
		return nil
	}

	visiting[traitName] = true
	defer delete(visiting, traitName)

	fields := make([]*tokens.ImplementationField, 0, len(trait.Fields))
	indexByMethod := make(map[string]int)

	for _, parent := range trait.AllParents() {
		parentFields := collectTraitFieldsForCompletions(parent, traits, visiting)
		for _, field := range parentFields {
			if idx, exists := indexByMethod[field.Name]; exists {
				fields[idx] = field
				continue
			}
			indexByMethod[field.Name] = len(fields)
			fields = append(fields, field)
		}
	}

	for _, field := range trait.Fields {
		if idx, exists := indexByMethod[field.Name]; exists {
			fields[idx] = field
			continue
		}
		indexByMethod[field.Name] = len(fields)
		fields = append(fields, field)
	}

	return fields
}

// getTraitMethodCompletions returns completions for trait methods implemented for a class
func getTraitMethodCompletions(file *tokens.File, className, prefix string, typeArgs []string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	addedMethods := make(map[string]bool)
	traitMap := buildTraitMap(file)

	// Get type parameters from the class definition for substitution
	var typeParams []string
	for _, entry := range file.Entries {
		if entry.Class != nil && entry.Class.Name == className {
			for _, tp := range entry.Class.TypeParams {
				typeParams = append(typeParams, tp.Name)
			}
			break
		}
	}

	for _, entry := range file.Entries {
		if entry.Implementation != nil && isImplForClass(entry.Implementation, className) {
			// This is an impl block for the class
			for _, field := range entry.Implementation.GetFields() {
				name := field.Name
				if strings.HasPrefix(name, prefix) && !addedMethods[name] {
					addedMethods[name] = true
					detail := formatImplMethodSignature(field)
					// Substitute type parameters with actual type arguments
					if len(typeArgs) > 0 {
						detail = substituteTypeParams(detail, typeParams, typeArgs)
					}
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

			// Add methods from the implemented trait hierarchy (handles inherited/default methods).
			if entry.Implementation.GetFor() != "" && entry.Implementation.GetName() != "" {
				traitName := entry.Implementation.GetName()
				for _, field := range collectTraitFieldsForCompletions(traitName, traitMap, map[string]bool{}) {
					name := field.Name
					if !strings.HasPrefix(name, prefix) || addedMethods[name] {
						continue
					}
					addedMethods[name] = true
					detail := formatImplMethodSignature(field)
					if len(typeArgs) > 0 {
						detail = substituteTypeParams(detail, typeParams, typeArgs)
					}
					detail = fmt.Sprintf("(%s) %s", traitName, detail)
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

// formatImplMethodSignature formats an implementation field as a method signature
func formatImplMethodSignature(f *tokens.ImplementationField) string {
	var sb strings.Builder
	sb.WriteString("func(")
	for i, arg := range f.Arguments {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(arg.Name)
		if arg.Type != nil {
			sb.WriteString(": ")
			sb.WriteString(analysis.FormatTypeRef(arg.Type))
		}
	}
	sb.WriteString(")")
	if f.Type != nil {
		sb.WriteString(": ")
		sb.WriteString(analysis.FormatTypeRef(f.Type))
	}
	return sb.String()
}
