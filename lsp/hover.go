// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
)

// HoverInfo contains information for hover tooltips
type HoverInfo struct {
	Name       string
	Type       string
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

		// Check if cursor is within the method body
		if method.Pos.Line <= line && line <= method.EndPos.Line {
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
			if field.Method != nil && field.Method.Pos.Line <= line && line <= field.Method.EndPos.Line {
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
			if field.Pos.Line <= line && line <= field.EndPos.Line {
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
	if expr == nil || expr.GetLogicalOr() == nil {
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
	lo := expr.GetLogicalOr()
	if lo == nil {
		return nil
	}
	if lo.LogicalAnd == nil {
		return nil
	}
	if lo.LogicalAnd.Equality == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison.Addition == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison.Addition.Multiplication == nil {
		return nil
	}
	if lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary == nil {
		return nil
	}
	return lo.LogicalAnd.Equality.Comparison.Addition.Multiplication.Unary.Primary
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

	if entry.Trait != nil {
		for _, field := range entry.Trait.Fields {
			if field.Name == name {
				return &HoverInfo{
					Name:       field.Name,
					Type:       fmt.Sprintf("func %s%s", field.Name, strings.TrimPrefix(formatImplMethodSignature(field), "func")),
					DocComment: strings.Join(field.DocComment, "\n"),
				}
			}
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

	if entry.Implementation != nil {
		for _, field := range entry.Implementation.GetFields() {
			if field.Name == name {
				return &HoverInfo{
					Name:       field.Name,
					Type:       fmt.Sprintf("func %s%s", field.Name, strings.TrimPrefix(formatImplMethodSignature(field), "func")),
					DocComment: strings.Join(field.DocComment, "\n"),
				}
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
