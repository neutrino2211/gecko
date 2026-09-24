// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

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
