// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetCompletions returns completion items for a position
func GetCompletions(content, filePath string, line, col int) []protocol.CompletionItem {
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

	// Sanitize content for parsing - remove incomplete expressions on current line
	sanitizedContent := sanitizeForParsing(content, line)
	file, _ := parser.Parser.ParseString(filePath, sanitizedContent)
	if file == nil {
		file = &tokens.File{}
	}
	file.ComputeRanges()

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
				items = append(items, getStaticMethodCompletions(file, typeName, prefix)...)
				return items
			}
		}
	}

	// Check if we're after a dot (member access)
	if line < len(lines) && col > 0 {
		dotPos := col - len(prefix) - 1
		if dotPos >= 0 && dotPos < len(lineText) && lineText[dotPos] == '.' {
			// Member completion - find the object before the dot
			objEnd := dotPos
			objStart := objEnd
			for objStart > 0 && isIdentChar(lineText[objStart-1]) {
				objStart--
			}
			if objStart < objEnd {
				objName := lineText[objStart:objEnd]
				// Find the type using local scope resolution
				items = append(items, getMemberCompletionsWithScope(file, filePath, objName, prefix, line+1)...)
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
		items = append(items, getEntryCompletions(entry, prefix)...)
	}

	// Add local variables if inside a function
	enclosingMethod := findEnclosingMethod(file, line+1)
	items = append(items, getLocalCompletions(enclosingMethod, prefix, line+1, file)...)

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
		lines[cursorLine] = ""
	}

	return strings.Join(lines, "\n")
}

// getStaticMethodCompletions returns static method completions for a type (Type::)
func getStaticMethodCompletions(file *tokens.File, typeName, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

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
func getMemberCompletionsWithScope(file *tokens.File, filePath, objName, prefix string, cursorLine int) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Check if objName is an imported module
	for _, entry := range file.Entries {
		if entry.Import != nil && entry.Import.ModuleName() == objName {
			items = append(items, getImportedModuleCompletions(filePath, objName, prefix)...)
			return items
		}
	}

	// Look up the type from local scope first
	typeName := lookupVariableTypeInScope(file, objName, cursorLine)
	if typeName == "" {
		// Fall back to global lookup
		typeName = lookupVariableType(file, objName)
	}
	if typeName == "" {
		// Maybe it's a class name directly
		typeName = objName
	}

	// Remove pointer suffix for class lookup
	baseType := strings.TrimSuffix(typeName, "*")
	baseType = strings.TrimSuffix(baseType, "!")

	// Parse generic type to extract base name and type arguments
	parsedType := parseGenericType(baseType)

	// Check if the type is module-qualified (e.g., "shapes.Circle")
	if strings.Contains(parsedType.BaseName, ".") {
		parts := strings.SplitN(parsedType.BaseName, ".", 2)
		moduleName := parts[0]
		className := parts[1]

		// Get completions from imported module with visibility filtering
		items = append(items, getImportedClassMemberCompletionsGeneric(filePath, moduleName, className, prefix, parsedType.TypeArgs)...)
		items = append(items, getImportedTraitMethodCompletions(filePath, moduleName, className, prefix)...)
		return items
	}

	// Find the class with this type name and return its members (same module - no visibility filter)
	items = append(items, getClassMemberCompletionsGeneric(file, parsedType.BaseName, prefix, parsedType.TypeArgs)...)

	// Also add trait methods implemented for this class
	items = append(items, getTraitMethodCompletions(file, parsedType.BaseName, prefix, parsedType.TypeArgs)...)

	return items
}

// lookupVariableTypeInScope looks up a variable's type considering scope at cursor position
func lookupVariableTypeInScope(file *tokens.File, varName string, cursorLine int) string {
	if file == nil {
		return ""
	}

	for _, entry := range file.Entries {
		// Check if cursor is in a top-level method
		if entry.Method != nil {
			if cursorLine >= entry.Method.Pos.Line && cursorLine <= entry.Method.EndPos.Line {
				// Check arguments
				for _, arg := range entry.Method.Arguments {
					if arg.Name == varName && arg.Type != nil {
						return analysis.FormatTypeRef(arg.Type)
					}
				}
				// Check local variables
				if typeStr := lookupVarInEntriesBeforeLine(entry.Method.Value, varName, cursorLine); typeStr != "" {
					return typeStr
				}
			}
		}

		// Check class methods
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					if cursorLine >= field.Method.Pos.Line && cursorLine <= field.Method.EndPos.Line {
						// Check arguments
						for _, arg := range field.Method.Arguments {
							if arg.Name == varName && arg.Type != nil {
								return analysis.FormatTypeRef(arg.Type)
							}
						}
						// Check local variables
						if typeStr := lookupVarInEntriesBeforeLine(field.Method.Value, varName, cursorLine); typeStr != "" {
							return typeStr
						}
					}
				}
			}
		}

		// Check implementation methods
		if entry.Implementation != nil {
			for _, implMethod := range entry.Implementation.GetFields() {
				if cursorLine >= implMethod.Pos.Line && cursorLine <= implMethod.EndPos.Line {
					// Check arguments
					for _, arg := range implMethod.Arguments {
						if arg.Name == varName && arg.Type != nil {
							return analysis.FormatTypeRef(arg.Type)
						}
					}
					// Check local variables
					if typeStr := lookupVarInEntriesBeforeLine(implMethod.Value, varName, cursorLine); typeStr != "" {
						return typeStr
					}
				}
			}
		}
	}

	return ""
}

// lookupVarInEntriesBeforeLine searches for a variable declared before the cursor line
func lookupVarInEntriesBeforeLine(entries []*tokens.Entry, varName string, cursorLine int) string {
	for _, entry := range entries {
		if entry.Field != nil && entry.Field.Name == varName && entry.Field.Pos.Line < cursorLine {
			if entry.Field.Type != nil {
				return analysis.FormatTypeRef(entry.Field.Type)
			}
			// Try to infer from value
			if entry.Field.Value != nil {
				return inferSimpleType(entry.Field.Value)
			}
		}
		// Recurse into blocks only if cursor is within the block
		if entry.If != nil {
			if typeStr := lookupVarInIfBlock(entry.If, varName, cursorLine); typeStr != "" {
				return typeStr
			}
		}
		if entry.Loop != nil {
			if typeStr := lookupVarInLoopBlock(entry.Loop, varName, cursorLine); typeStr != "" {
				return typeStr
			}
		}
	}
	return ""
}

// lookupVarInIfBlock searches for a variable in if/else-if/else blocks
func lookupVarInIfBlock(ifBlock *tokens.If, varName string, cursorLine int) string {
	// Check if cursor is within the if block
	if cursorLine >= ifBlock.Pos.Line && cursorLine <= ifBlock.EndPos.Line {
		if typeStr := lookupVarInEntriesBeforeLine(ifBlock.Value, varName, cursorLine); typeStr != "" {
			return typeStr
		}
	}

	// Check else-if branches
	if ifBlock.ElseIf != nil {
		if typeStr := lookupVarInElseIfBlock(ifBlock.ElseIf, varName, cursorLine); typeStr != "" {
			return typeStr
		}
	}

	// Check else branch
	if ifBlock.Else != nil && ifBlock.Else.Value != nil {
		if cursorLine >= ifBlock.Else.Pos.Line && cursorLine <= ifBlock.Else.EndPos.Line {
			if typeStr := lookupVarInEntriesBeforeLine(ifBlock.Else.Value, varName, cursorLine); typeStr != "" {
				return typeStr
			}
		}
	}

	return ""
}

// lookupVarInElseIfBlock searches for a variable in else-if blocks
func lookupVarInElseIfBlock(elseIf *tokens.ElseIf, varName string, cursorLine int) string {
	if cursorLine >= elseIf.Pos.Line && cursorLine <= elseIf.EndPos.Line {
		if typeStr := lookupVarInEntriesBeforeLine(elseIf.Value, varName, cursorLine); typeStr != "" {
			return typeStr
		}
	}

	// Recurse into nested else-if
	if elseIf.ElseIf != nil {
		if typeStr := lookupVarInElseIfBlock(elseIf.ElseIf, varName, cursorLine); typeStr != "" {
			return typeStr
		}
	}

	// Check else branch
	if elseIf.Else != nil && elseIf.Else.Value != nil {
		if cursorLine >= elseIf.Else.Pos.Line && cursorLine <= elseIf.Else.EndPos.Line {
			if typeStr := lookupVarInEntriesBeforeLine(elseIf.Else.Value, varName, cursorLine); typeStr != "" {
				return typeStr
			}
		}
	}

	return ""
}

// lookupVarInLoopBlock searches for a variable in loop blocks, including loop variables
func lookupVarInLoopBlock(loop *tokens.Loop, varName string, cursorLine int) string {
	// Check if cursor is within the loop
	if cursorLine < loop.Pos.Line || cursorLine > loop.EndPos.Line {
		return ""
	}

	// Check for-in loop variable
	if loop.ForIn != nil && loop.ForIn.Variable != nil {
		if loop.ForIn.Variable.Name == varName {
			if loop.ForIn.Variable.Type != nil {
				return analysis.FormatTypeRef(loop.ForIn.Variable.Type)
			}
			return "unknown"
		}
	}

	// Check for-of loop variable
	if loop.ForOf != nil && loop.ForOf.Variable != nil {
		if loop.ForOf.Variable.Name == varName {
			if loop.ForOf.Variable.Type != nil {
				return analysis.FormatTypeRef(loop.ForOf.Variable.Type)
			}
			return "unknown"
		}
	}

	// Search in loop body
	return lookupVarInEntriesBeforeLine(loop.Value, varName, cursorLine)
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

	return items
}

// MethodInfo holds method-like info from Method or ImplementationField
type MethodInfo struct {
	Arguments []*tokens.Value
	Value     []*tokens.Entry
}

// findEnclosingMethod finds the method that contains the cursor position using ranges
func findEnclosingMethod(file *tokens.File, cursorLine int) *MethodInfo {
	for _, entry := range file.Entries {
		// Check top-level methods
		if entry.Method != nil {
			if cursorLine >= entry.Method.Pos.Line && cursorLine <= entry.Method.EndPos.Line {
				return &MethodInfo{
					Arguments: entry.Method.Arguments,
					Value:     entry.Method.Value,
				}
			}
		}

		// Check class methods
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					if cursorLine >= field.Method.Pos.Line && cursorLine <= field.Method.EndPos.Line {
						return &MethodInfo{
							Arguments: field.Method.Arguments,
							Value:     field.Method.Value,
						}
					}
				}
			}
		}

		// Check trait methods
		if entry.Trait != nil {
			for _, field := range entry.Trait.Fields {
				if cursorLine >= field.Pos.Line && cursorLine <= field.EndPos.Line {
					return &MethodInfo{
						Arguments: field.Arguments,
						Value:     field.Value,
					}
				}
			}
		}

		// Check implementation methods
		if entry.Implementation != nil {
			for _, implMethod := range entry.Implementation.GetFields() {
				if cursorLine >= implMethod.Pos.Line && cursorLine <= implMethod.EndPos.Line {
					return &MethodInfo{
						Arguments: implMethod.Arguments,
						Value:     implMethod.Value,
					}
				}
			}
		}
	}

	return nil
}

func getLocalCompletions(method *MethodInfo, prefix string, cursorLine int, file *tokens.File) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	if method == nil {
		return items
	}

	// Add function arguments
	for _, arg := range method.Arguments {
		if strings.HasPrefix(arg.Name, prefix) {
			typeStr := "unknown"
			if arg.Type != nil {
				typeStr = analysis.FormatTypeRef(arg.Type)
			}
			items = append(items, protocol.CompletionItem{
				Label:  arg.Name,
				Kind:   protocol.CompletionItemKindVariable,
				Detail: "(parameter) " + typeStr,
			})
		}
	}

	// Add local variables from method body
	items = append(items, getLocalVarsFromEntries(method.Value, prefix, cursorLine, file)...)

	return items
}

func getLocalVarsFromEntries(entries []*tokens.Entry, prefix string, cursorLine int, file *tokens.File) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Create analysis context for type inference
	var ctx *analysis.AnalysisContext
	if file != nil && file.Path != "" {
		ctx, _ = analysis.NewAnalysisContext(file.Path, file.Content)
	}

	for _, entry := range entries {
		// Only include variables declared before the cursor
		if entry.Field != nil && entry.Field.Pos.Line <= cursorLine {
			if strings.HasPrefix(entry.Field.Name, prefix) {
				typeStr := "unknown"
				if entry.Field.Type != nil {
					typeStr = analysis.FormatTypeRef(entry.Field.Type)
				} else if entry.Field.Value != nil && ctx != nil {
					typeStr = analysis.InferExpressionType(entry.Field.Value, ctx)
				}
				items = append(items, protocol.CompletionItem{
					Label:  entry.Field.Name,
					Kind:   protocol.CompletionItemKindVariable,
					Detail: typeStr,
				})
			}
		}

		// Recurse into blocks only if cursor is within the block
		if entry.If != nil {
			items = append(items, getVarsFromIfBlock(entry.If, prefix, cursorLine, file)...)
		}
		if entry.Loop != nil {
			items = append(items, getVarsFromLoopBlock(entry.Loop, prefix, cursorLine, file)...)
		}
	}

	return items
}

// getVarsFromIfBlock extracts variables from if/else-if/else blocks, respecting scope
func getVarsFromIfBlock(ifBlock *tokens.If, prefix string, cursorLine int, file *tokens.File) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Check if cursor is within the if block's body
	if cursorLine >= ifBlock.Pos.Line && cursorLine <= ifBlock.EndPos.Line {
		// Cursor is somewhere in this if statement - check which branch
		items = append(items, getLocalVarsFromEntries(ifBlock.Value, prefix, cursorLine, file)...)
	}

	// Check else-if branches
	if ifBlock.ElseIf != nil {
		items = append(items, getVarsFromElseIfBlock(ifBlock.ElseIf, prefix, cursorLine, file)...)
	}

	// Check else branch
	if ifBlock.Else != nil && ifBlock.Else.Value != nil {
		if cursorLine >= ifBlock.Else.Pos.Line && cursorLine <= ifBlock.Else.EndPos.Line {
			items = append(items, getLocalVarsFromEntries(ifBlock.Else.Value, prefix, cursorLine, file)...)
		}
	}

	return items
}

// getVarsFromElseIfBlock extracts variables from else-if blocks
func getVarsFromElseIfBlock(elseIf *tokens.ElseIf, prefix string, cursorLine int, file *tokens.File) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	if cursorLine >= elseIf.Pos.Line && cursorLine <= elseIf.EndPos.Line {
		items = append(items, getLocalVarsFromEntries(elseIf.Value, prefix, cursorLine, file)...)
	}

	// Recurse into nested else-if
	if elseIf.ElseIf != nil {
		items = append(items, getVarsFromElseIfBlock(elseIf.ElseIf, prefix, cursorLine, file)...)
	}

	// Check else branch
	if elseIf.Else != nil && elseIf.Else.Value != nil {
		if cursorLine >= elseIf.Else.Pos.Line && cursorLine <= elseIf.Else.EndPos.Line {
			items = append(items, getLocalVarsFromEntries(elseIf.Else.Value, prefix, cursorLine, file)...)
		}
	}

	return items
}

// getVarsFromLoopBlock extracts variables from loop blocks, including loop variables
func getVarsFromLoopBlock(loop *tokens.Loop, prefix string, cursorLine int, file *tokens.File) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Check if cursor is within the loop body
	if cursorLine < loop.Pos.Line || cursorLine > loop.EndPos.Line {
		return items
	}

	// Add for-in loop variable
	if loop.ForIn != nil && loop.ForIn.Variable != nil {
		varName := loop.ForIn.Variable.Name
		if strings.HasPrefix(varName, prefix) {
			typeStr := "unknown"
			if loop.ForIn.Variable.Type != nil {
				typeStr = analysis.FormatTypeRef(loop.ForIn.Variable.Type)
			}
			items = append(items, protocol.CompletionItem{
				Label:  varName,
				Kind:   protocol.CompletionItemKindVariable,
				Detail: "(loop variable) " + typeStr,
			})
		}
	}

	// Add for-of loop variable
	if loop.ForOf != nil && loop.ForOf.Variable != nil {
		varName := loop.ForOf.Variable.Name
		if strings.HasPrefix(varName, prefix) {
			typeStr := "unknown"
			if loop.ForOf.Variable.Type != nil {
				typeStr = analysis.FormatTypeRef(loop.ForOf.Variable.Type)
			}
			items = append(items, protocol.CompletionItem{
				Label:  varName,
				Kind:   protocol.CompletionItemKindVariable,
				Detail: "(loop variable) " + typeStr,
			})
		}
	}

	// Add variables from loop body
	items = append(items, getLocalVarsFromEntries(loop.Value, prefix, cursorLine, file)...)

	return items
}
