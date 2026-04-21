package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// ParsedGenericType represents a parsed generic type like Vec<int>
type ParsedGenericType struct {
	BaseName string   // The base type name (e.g., "Vec")
	TypeArgs []string // The type arguments (e.g., ["int"])
}

// parseGenericType parses a type string like "Vec<int>" into its components
func parseGenericType(typeStr string) ParsedGenericType {
	// Handle pointer suffix
	typeStr = strings.TrimSuffix(typeStr, "*")
	typeStr = strings.TrimSuffix(typeStr, "!")

	// Find the generic brackets
	ltIdx := strings.Index(typeStr, "<")
	if ltIdx == -1 {
		return ParsedGenericType{BaseName: typeStr}
	}

	baseName := typeStr[:ltIdx]

	// Extract type arguments (simple parsing - doesn't handle nested generics perfectly)
	gtIdx := strings.LastIndex(typeStr, ">")
	if gtIdx == -1 || gtIdx <= ltIdx {
		return ParsedGenericType{BaseName: baseName}
	}

	argsStr := typeStr[ltIdx+1 : gtIdx]
	args := strings.Split(argsStr, ",")
	for i := range args {
		args[i] = strings.TrimSpace(args[i])
	}

	return ParsedGenericType{BaseName: baseName, TypeArgs: args}
}

// substituteTypeParams replaces type parameters with actual type arguments in a type string
func substituteTypeParams(typeStr string, typeParams []string, typeArgs []string) string {
	if len(typeParams) == 0 || len(typeArgs) == 0 {
		return typeStr
	}

	result := typeStr
	for i, param := range typeParams {
		if i < len(typeArgs) {
			// Replace whole word occurrences only
			result = replaceTypeParam(result, param, typeArgs[i])
		}
	}
	return result
}

// replaceTypeParam replaces a type parameter with its argument, handling word boundaries
func replaceTypeParam(str, param, replacement string) string {
	// Simple word boundary replacement
	var result strings.Builder
	i := 0
	for i < len(str) {
		// Check if we're at the start of the parameter
		if strings.HasPrefix(str[i:], param) {
			// Check word boundaries
			before := i == 0 || !isIdentChar(str[i-1])
			after := i+len(param) >= len(str) || !isIdentChar(str[i+len(param)])
			if before && after {
				result.WriteString(replacement)
				i += len(param)
				continue
			}
		}
		result.WriteByte(str[i])
		i++
	}
	return result.String()
}

// HoverInfo contains information for hover tooltips
type HoverInfo struct {
	Name    string
	Type    string
	DocComment string
}

// GetHoverInfo returns hover information for a position in the source
func GetHoverInfo(content string, line, col int) *HoverInfo {
	file, err := parser.Parser.ParseString("", content)
	if err != nil {
		return nil
	}
	file.ComputeRanges()

	word := getWordAt(content, line, col)
	if word == "" {
		return nil
	}

	// First check if we're inside a method body - look for local variables
	for _, entry := range file.Entries {
		if info := findLocalVariable(entry, word, line+1, col+1, file); info != nil {
			return info
		}
	}

	// Search for the symbol in top-level definitions
	for _, entry := range file.Entries {
		if info := findInEntry(entry, word); info != nil {
			return info
		}
	}

	return nil
}

// findLocalVariable searches for a local variable within method bodies
func findLocalVariable(entry *tokens.Entry, name string, line, _ int, file *tokens.File) *HoverInfo {
	// Check if we're in a method
	if entry.Method != nil {
		method := entry.Method

		// Check if cursor is within the method body (rough check based on method position)
		if method.Pos.Line <= line {
			// Check function arguments first
			for _, arg := range method.Arguments {
				if arg.Name == name {
					typeStr := "unknown"
					if arg.Type != nil {
						typeStr = analysis.FormatTypeRef(arg.Type)
					}
					return &HoverInfo{
						Name: arg.Name,
						Type: fmt.Sprintf("(parameter) %s: %s", arg.Name, typeStr),
					}
				}
			}

			// Search in method body for local variables
			if info := findInEntries(method.Value, name, file); info != nil {
				return info
			}
		}
	}

	// Check in class methods
	if entry.Class != nil {
		for _, field := range entry.Class.Fields {
			if field.Method != nil && field.Method.Pos.Line <= line {
				// Check arguments
				for _, arg := range field.Method.Arguments {
					if arg.Name == name {
						typeStr := "unknown"
						if arg.Type != nil {
							typeStr = analysis.FormatTypeRef(arg.Type)
						}
						return &HoverInfo{
							Name: arg.Name,
							Type: fmt.Sprintf("(parameter) %s: %s", arg.Name, typeStr),
						}
					}
				}

				// Check method body
				if info := findInEntries(field.Method.Value, name, file); info != nil {
					return info
				}
			}
		}
	}

	// Check in implementation methods
	if entry.Implementation != nil {
		for _, field := range entry.Implementation.GetFields() {
			if field.Pos.Line <= line {
				// Check arguments
				for _, arg := range field.Arguments {
					if arg.Name == name {
						typeStr := "unknown"
						if arg.Type != nil {
							typeStr = analysis.FormatTypeRef(arg.Type)
						}
						return &HoverInfo{
							Name: arg.Name,
							Type: fmt.Sprintf("(parameter) %s: %s", arg.Name, typeStr),
						}
					}
				}

				// Check method body
				if info := findInEntries(field.Value, name, file); info != nil {
					return info
				}
			}
		}
	}

	return nil
}

// findInEntries searches for a variable declaration in a list of entries
func findInEntries(entries []*tokens.Entry, name string, file *tokens.File) *HoverInfo {
	// Create analysis context for type inference
	var ctx *analysis.AnalysisContext
	if file != nil && file.Path != "" {
		ctx, _ = analysis.NewAnalysisContext(file.Path, file.Content)
	}

	for _, entry := range entries {
		// Check for variable declarations (let/const)
		if entry.Field != nil && entry.Field.Name == name {
			typeStr := "unknown"
			if entry.Field.Type != nil {
				typeStr = analysis.FormatTypeRef(entry.Field.Type)
			} else if entry.Field.Value != nil && ctx != nil {
				// Use analysis package for type inference
				typeStr = analysis.InferExpressionType(entry.Field.Value, ctx)
			}
			mutability := entry.Field.Mutability
			if mutability == "" {
				mutability = "let"
			}
			return &HoverInfo{
				Name: entry.Field.Name,
				Type: fmt.Sprintf("%s %s: %s", mutability, entry.Field.Name, typeStr),
			}
		}

		// Recurse into if blocks
		if entry.If != nil {
			if info := findInEntries(entry.If.Value, name, file); info != nil {
				return info
			}
			// Check else-if chain
			elseIf := entry.If.ElseIf
			for elseIf != nil {
				if info := findInEntries(elseIf.Value, name, file); info != nil {
					return info
				}
				elseIf = elseIf.ElseIf
			}
			// Check else
			if entry.If.Else != nil {
				if info := findInEntries(entry.If.Else.Value, name, file); info != nil {
					return info
				}
			}
		}

		// Recurse into loops
		if entry.Loop != nil {
			if info := findInEntries(entry.Loop.Value, name, file); info != nil {
				return info
			}
		}
	}
	return nil
}

// lookupVariableType looks up a variable's type from the file
func lookupVariableType(file *tokens.File, varName string) string {
	if file == nil {
		return ""
	}

	// Search all methods for the variable
	for _, entry := range file.Entries {
		if entry.Method != nil {
			// Check arguments
			for _, arg := range entry.Method.Arguments {
				if arg.Name == varName && arg.Type != nil {
					return analysis.FormatTypeRef(arg.Type)
				}
			}
			// Check local variables in method body
			if typeStr := lookupVarInEntries(entry.Method.Value, varName); typeStr != "" {
				return typeStr
			}
		}
		// Check class methods
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					for _, arg := range field.Method.Arguments {
						if arg.Name == varName && arg.Type != nil {
							return analysis.FormatTypeRef(arg.Type)
						}
					}
					if typeStr := lookupVarInEntries(field.Method.Value, varName); typeStr != "" {
						return typeStr
					}
				}
			}
		}
	}
	return ""
}

// lookupVarInEntries searches for a variable declaration in a list of entries
func lookupVarInEntries(entries []*tokens.Entry, varName string) string {
	for _, entry := range entries {
		if entry.Field != nil && entry.Field.Name == varName {
			if entry.Field.Type != nil {
				return analysis.FormatTypeRef(entry.Field.Type)
			}
			// Type might be inferred - try to get it from the value
			if entry.Field.Value != nil {
				// Avoid infinite recursion by using simple inference
				return inferSimpleType(entry.Field.Value)
			}
		}
		// Recurse into blocks
		if entry.If != nil {
			if typeStr := lookupVarInEntries(entry.If.Value, varName); typeStr != "" {
				return typeStr
			}
		}
		if entry.Loop != nil {
			if typeStr := lookupVarInEntries(entry.Loop.Value, varName); typeStr != "" {
				return typeStr
			}
		}
	}
	return ""
}

// inferSimpleType does basic type inference without recursion risk
func inferSimpleType(expr *tokens.Expression) string {
	if expr == nil || expr.LogicalOr == nil {
		return ""
	}
	primary := getPrimaryFromExpr(expr)
	if primary == nil || primary.Literal == nil {
		return ""
	}
	lit := primary.Literal
	if lit.StructType != "" {
		return lit.StructType
	}
	if lit.Number != "" {
		return "int"
	}
	if lit.String != "" {
		return "string"
	}
	if lit.Bool != "" {
		return "bool"
	}
	// For function calls, try to get the return type from static calls
	if lit.FuncCall != nil && lit.FuncCall.StaticType != "" {
		return lit.FuncCall.StaticType
	}
	return ""
}

func getPrimaryFromExpr(expr *tokens.Expression) *tokens.Primary {
	if expr.LogicalOr == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition.Multiplication == nil {
		return nil
	}
	if expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary == nil {
		return nil
	}
	return expr.LogicalOr.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary
}

func findInEntry(entry *tokens.Entry, name string) *HoverInfo {
	if entry.Class != nil && entry.Class.Name == name {
		return &HoverInfo{
			Name:       entry.Class.Name,
			Type:       analysis.FormatClassType(entry.Class),
			DocComment: strings.Join(entry.Class.DocComment, "\n"),
		}
	}

	if entry.Class != nil {
		for _, field := range entry.Class.Fields {
			if field.Method != nil && field.Method.Name == name {
				return &HoverInfo{
					Name:       field.Method.Name,
					Type:       analysis.FormatMethodSignature(field.Method),
					DocComment: strings.Join(field.Method.DocComment, "\n"),
				}
			}
			if field.Field != nil && field.Field.Name == name {
				return &HoverInfo{
					Name: field.Field.Name,
					Type: fmt.Sprintf("%s %s: %s", field.Field.Mutability, field.Field.Name, analysis.FormatTypeRef(field.Field.Type)),
				}
			}
		}
	}

	if entry.Trait != nil && entry.Trait.Name == name {
		return &HoverInfo{
			Name:       entry.Trait.Name,
			Type:       analysis.FormatTraitType(entry.Trait),
			DocComment: strings.Join(entry.Trait.DocComment, "\n"),
		}
	}

	if entry.Method != nil && entry.Method.Name == name {
		return &HoverInfo{
			Name:       entry.Method.Name,
			Type:       analysis.FormatMethodSignature(entry.Method),
			DocComment: strings.Join(entry.Method.DocComment, "\n"),
		}
	}

	if entry.Field != nil && entry.Field.Name == name {
		return &HoverInfo{
			Name:       entry.Field.Name,
			Type:       fmt.Sprintf("%s %s: %s", entry.Field.Mutability, entry.Field.Name, analysis.FormatTypeRef(entry.Field.Type)),
			DocComment: strings.Join(entry.Field.DocComment, "\n"),
		}
	}

	if entry.Declaration != nil {
		if entry.Declaration.Method != nil && entry.Declaration.Method.Name == name {
			return &HoverInfo{
				Name:       entry.Declaration.Method.Name,
				Type:       "declare " + analysis.FormatMethodSignature(entry.Declaration.Method),
				DocComment: strings.Join(entry.Declaration.Method.DocComment, "\n"),
			}
		}
	}

	return nil
}

func getWordAt(content string, line, col int) string {
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}

	lineContent := lines[line]
	if col < 0 || col >= len(lineContent) {
		if col == len(lineContent) && col > 0 {
			col = col - 1
		} else {
			return ""
		}
	}

	start := col
	for start > 0 && isIdentChar(lineContent[start-1]) {
		start--
	}

	end := col
	for end < len(lineContent) && isIdentChar(lineContent[end]) {
		end++
	}

	if start == end {
		return ""
	}

	return lineContent[start:end]
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// Keywords in Gecko
var geckoKeywords = []string{
	"func", "let", "const", "if", "else", "while", "for", "return",
	"class", "trait", "impl", "import", "package", "declare", "external",
	"true", "false", "break", "continue", "asm", "as", "is",
}

// Built-in types
var geckoTypes = []string{
	"int", "int8", "int16", "int32", "int64",
	"uint", "uint8", "uint16", "uint32", "uint64",
	"bool", "string", "void", "float32", "float64",
}

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

// GetSignatureHelp returns signature help for a function call at the given position
func GetSignatureHelp(content, filePath string, line, col int) *protocol.SignatureHelp {
	lines := strings.Split(content, "\n")
	if line >= len(lines) {
		return nil
	}
	lineText := lines[line]
	if col > len(lineText) {
		col = len(lineText)
	}

	// Find the opening parenthesis and count commas to determine active parameter
	parenDepth := 0
	activeParam := 0
	callStart := -1

	// Scan backwards from cursor to find the function call
	for i := col - 1; i >= 0; i-- {
		ch := lineText[i]
		if ch == ')' {
			parenDepth++
		} else if ch == '(' {
			if parenDepth == 0 {
				callStart = i
				break
			}
			parenDepth--
		} else if ch == ',' && parenDepth == 0 {
			activeParam++
		}
	}

	if callStart < 0 {
		return nil
	}

	// Extract function/method name before the parenthesis
	nameEnd := callStart
	nameStart := nameEnd
	for nameStart > 0 && isIdentChar(lineText[nameStart-1]) {
		nameStart--
	}
	if nameStart >= nameEnd {
		return nil
	}
	funcName := lineText[nameStart:nameEnd]

	// Check if it's a method call (preceded by '.')
	var objName string
	if nameStart > 0 && lineText[nameStart-1] == '.' {
		objEnd := nameStart - 1
		objStart := objEnd
		for objStart > 0 && isIdentChar(lineText[objStart-1]) {
			objStart--
		}
		if objStart < objEnd {
			objName = lineText[objStart:objEnd]
		}
	}

	// Check if it's a static method call (preceded by '::')
	var typeName string
	if nameStart > 1 && lineText[nameStart-1] == ':' && lineText[nameStart-2] == ':' {
		typeEnd := nameStart - 2
		typeStart := typeEnd
		for typeStart > 0 && isIdentChar(lineText[typeStart-1]) {
			typeStart--
		}
		if typeStart < typeEnd {
			typeName = lineText[typeStart:typeEnd]
		}
	}

	// Parse the file to look up function signatures
	sanitizedContent := sanitizeForParsing(content, line)
	file, _ := parser.Parser.ParseString(filePath, sanitizedContent)
	if file == nil {
		return nil
	}
	file.ComputeRanges()

	var signature *protocol.SignatureInformation

	if typeName != "" {
		// Static method call - look up in impl blocks
		signature = findStaticMethodSignature(file, typeName, funcName)
	} else if objName != "" {
		// Instance method call - resolve variable type and look up method
		varType := lookupVariableTypeInScope(file, objName, line+1)
		if varType == "" {
			varType = lookupVariableType(file, objName)
		}
		if varType != "" {
			parsedType := parseGenericType(varType)
			signature = findMethodSignature(file, filePath, parsedType.BaseName, funcName, parsedType.TypeArgs)
		}
	} else {
		// Free function call
		signature = findFunctionSignature(file, funcName)
	}

	if signature == nil {
		return nil
	}

	return &protocol.SignatureHelp{
		Signatures:      []protocol.SignatureInformation{*signature},
		ActiveSignature: 0,
		ActiveParameter: uint32(activeParam),
	}
}

// findFunctionSignature finds a top-level function signature
func findFunctionSignature(file *tokens.File, funcName string) *protocol.SignatureInformation {
	for _, entry := range file.Entries {
		if entry.Method != nil && entry.Method.Name == funcName {
			return buildSignatureInfo(entry.Method.Name, entry.Method.Arguments, entry.Method.Type)
		}
	}
	return nil
}

// findStaticMethodSignature finds a static method signature in impl blocks
func findStaticMethodSignature(file *tokens.File, typeName, methodName string) *protocol.SignatureInformation {
	for _, entry := range file.Entries {
		if entry.Implementation != nil && isImplForClass(entry.Implementation, typeName) {
			for _, field := range entry.Implementation.GetFields() {
				if field.Name == methodName {
					return buildImplSignatureInfo(field)
				}
			}
		}
	}
	return nil
}

// findMethodSignature finds an instance method signature
func findMethodSignature(file *tokens.File, filePath, typeName, methodName string, typeArgs []string) *protocol.SignatureInformation {
	// Get type parameters for substitution
	var typeParams []string
	for _, entry := range file.Entries {
		if entry.Class != nil && entry.Class.Name == typeName {
			for _, tp := range entry.Class.TypeParams {
				typeParams = append(typeParams, tp.Name)
			}
			break
		}
	}

	// Look in impl blocks
	for _, entry := range file.Entries {
		if entry.Implementation != nil && isImplForClass(entry.Implementation, typeName) {
			for _, field := range entry.Implementation.GetFields() {
				if field.Name == methodName {
					sig := buildImplSignatureInfo(field)
					if len(typeArgs) > 0 {
						sig.Label = substituteTypeParams(sig.Label, typeParams, typeArgs)
					}
					return sig
				}
			}
		}
	}
	return nil
}

// buildSignatureInfo builds a SignatureInformation from function arguments
func buildSignatureInfo(name string, args []*tokens.Value, returnType *tokens.TypeRef) *protocol.SignatureInformation {
	var params []protocol.ParameterInformation
	var paramStrs []string

	for _, arg := range args {
		typeStr := "unknown"
		if arg.Type != nil {
			typeStr = analysis.FormatTypeRef(arg.Type)
		}
		paramStr := fmt.Sprintf("%s: %s", arg.Name, typeStr)
		paramStrs = append(paramStrs, paramStr)
		params = append(params, protocol.ParameterInformation{
			Label: paramStr,
		})
	}

	retStr := "void"
	if returnType != nil {
		retStr = analysis.FormatTypeRef(returnType)
	}

	label := fmt.Sprintf("%s(%s): %s", name, strings.Join(paramStrs, ", "), retStr)

	return &protocol.SignatureInformation{
		Label:      label,
		Parameters: params,
	}
}

// buildImplSignatureInfo builds a SignatureInformation from an impl field
func buildImplSignatureInfo(field *tokens.ImplementationField) *protocol.SignatureInformation {
	var params []protocol.ParameterInformation
	var paramStrs []string

	for _, arg := range field.Arguments {
		typeStr := "unknown"
		if arg.Type != nil {
			typeStr = analysis.FormatTypeRef(arg.Type)
		}
		paramStr := fmt.Sprintf("%s: %s", arg.Name, typeStr)
		paramStrs = append(paramStrs, paramStr)
		params = append(params, protocol.ParameterInformation{
			Label: paramStr,
		})
	}

	retStr := "void"
	if field.Type != nil {
		retStr = analysis.FormatTypeRef(field.Type)
	}

	label := fmt.Sprintf("%s(%s): %s", field.Name, strings.Join(paramStrs, ", "), retStr)

	return &protocol.SignatureInformation{
		Label:      label,
		Parameters: params,
	}
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

// getTraitMethodCompletions returns completions for trait methods implemented for a class
func getTraitMethodCompletions(file *tokens.File, className, prefix string, typeArgs []string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	addedMethods := make(map[string]bool)

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
	baseDir := filepath.Dir(filePath)
	candidates := []string{
		filepath.Join(baseDir, moduleName+".gecko"),
		filepath.Join(baseDir, moduleName, "mod.gecko"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
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

// GetDefinitionLocation returns the location of a symbol's definition
func GetDefinitionLocation(content string, line, col int, uri string) *protocol.Location {
	file, err := parser.Parser.ParseString("", content)
	if err != nil {
		return nil
	}
	file.ComputeRanges()

	word := getWordAt(content, line, col)
	if word == "" {
		return nil
	}

	// First check for local variables within method bodies
	for _, entry := range file.Entries {
		if loc := findLocalDefinition(entry, word, line+1, col+1, uri); loc != nil {
			return loc
		}
	}

	// Search for top-level definitions
	for _, entry := range file.Entries {
		if loc := findDefinitionInEntry(entry, word, uri); loc != nil {
			return loc
		}
	}

	return nil
}

func findLocalDefinition(entry *tokens.Entry, name string, line, _ int, uri string) *protocol.Location {
	// Check if we're in a method
	if entry.Method != nil {
		method := entry.Method
		if method.Pos.Line <= line {
			// Check function arguments
			for _, arg := range method.Arguments {
				if arg.Name == name {
					return &protocol.Location{
						URI: protocol.DocumentURI(uri),
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1)},
							End:   protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1 + len(arg.Name))},
						},
					}
				}
			}

			// Search in method body
			if loc := findDefinitionInEntries(method.Value, name, uri); loc != nil {
				return loc
			}
		}
	}

	// Check in class methods
	if entry.Class != nil {
		for _, field := range entry.Class.Fields {
			if field.Method != nil && field.Method.Pos.Line <= line {
				for _, arg := range field.Method.Arguments {
					if arg.Name == name {
						return &protocol.Location{
							URI: protocol.DocumentURI(uri),
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1 + len(arg.Name))},
							},
						}
					}
				}
				if loc := findDefinitionInEntries(field.Method.Value, name, uri); loc != nil {
					return loc
				}
			}
		}
	}

	return nil
}

func findDefinitionInEntries(entries []*tokens.Entry, name string, uri string) *protocol.Location {
	for _, entry := range entries {
		if entry.Field != nil && entry.Field.Name == name {
			return &protocol.Location{
				URI: protocol.DocumentURI(uri),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1)},
					End:   protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1 + len(entry.Field.Name))},
				},
			}
		}

		// Recurse into if/else/loop blocks
		if entry.If != nil {
			if loc := findDefinitionInEntries(entry.If.Value, name, uri); loc != nil {
				return loc
			}
			elseIf := entry.If.ElseIf
			for elseIf != nil {
				if loc := findDefinitionInEntries(elseIf.Value, name, uri); loc != nil {
					return loc
				}
				elseIf = elseIf.ElseIf
			}
			if entry.If.Else != nil {
				if loc := findDefinitionInEntries(entry.If.Else.Value, name, uri); loc != nil {
					return loc
				}
			}
		}

		if entry.Loop != nil {
			if loc := findDefinitionInEntries(entry.Loop.Value, name, uri); loc != nil {
				return loc
			}
		}
	}
	return nil
}

func findDefinitionInEntry(entry *tokens.Entry, name string, uri string) *protocol.Location {
	if entry.Class != nil && entry.Class.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Class.Pos.Line - 1), Character: uint32(entry.Class.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Class.Pos.Line - 1), Character: uint32(entry.Class.Pos.Column - 1 + len(entry.Class.Name))},
			},
		}
	}

	// Check class fields and methods
	if entry.Class != nil {
		for _, field := range entry.Class.Fields {
			if field.Method != nil && field.Method.Name == name {
				return &protocol.Location{
					URI: protocol.DocumentURI(uri),
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(field.Method.Pos.Line - 1), Character: uint32(field.Method.Pos.Column - 1)},
						End:   protocol.Position{Line: uint32(field.Method.Pos.Line - 1), Character: uint32(field.Method.Pos.Column - 1 + len(field.Method.Name))},
					},
				}
			}
			if field.Field != nil && field.Field.Name == name {
				return &protocol.Location{
					URI: protocol.DocumentURI(uri),
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(field.Field.Pos.Line - 1), Character: uint32(field.Field.Pos.Column - 1)},
						End:   protocol.Position{Line: uint32(field.Field.Pos.Line - 1), Character: uint32(field.Field.Pos.Column - 1 + len(field.Field.Name))},
					},
				}
			}
		}
	}

	if entry.Trait != nil && entry.Trait.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Trait.Pos.Line - 1), Character: uint32(entry.Trait.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Trait.Pos.Line - 1), Character: uint32(entry.Trait.Pos.Column - 1 + len(entry.Trait.Name))},
			},
		}
	}

	if entry.Method != nil && entry.Method.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Method.Pos.Line - 1), Character: uint32(entry.Method.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Method.Pos.Line - 1), Character: uint32(entry.Method.Pos.Column - 1 + len(entry.Method.Name))},
			},
		}
	}

	if entry.Field != nil && entry.Field.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1 + len(entry.Field.Name))},
			},
		}
	}

	if entry.Declaration != nil {
		if entry.Declaration.Method != nil && entry.Declaration.Method.Name == name {
			return &protocol.Location{
				URI: protocol.DocumentURI(uri),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(entry.Declaration.Method.Pos.Line - 1), Character: uint32(entry.Declaration.Method.Pos.Column - 1)},
					End:   protocol.Position{Line: uint32(entry.Declaration.Method.Pos.Line - 1), Character: uint32(entry.Declaration.Method.Pos.Column - 1 + len(entry.Declaration.Method.Name))},
				},
			}
		}
	}

	return nil
}
