// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package analysis

import (
	"github.com/neutrino2211/gecko/tokens"
)

// VariableType returns the resolved (compiler-inferred) type of a variable,
// parameter, or field visible at the given position. It prefers the shared
// semantic graph so the type matches what the compiler computed (including
// let-binding inference), falling back to the declared type.
func (ctx *AnalysisContext) VariableType(name string, line, col int) *tokens.TypeRef {
	if ctx == nil || ctx.SemanticGraph == nil {
		return nil
	}
	if t := ctx.variableTypeInEntries(ctx.MainFile.Entries, name); t != nil {
		return t
	}
	for _, f := range ctx.ImportedFiles {
		if t := ctx.variableTypeInEntries(f.Entries, name); t != nil {
			return t
		}
	}
	// Fall back to top-level globals recorded by the analyzer.
	if t := ctx.SemanticGraph.GlobalType(name); t != nil {
		return t
	}
	return nil
}

func (ctx *AnalysisContext) variableTypeInEntries(entries []*tokens.Entry, name string) *tokens.TypeRef {
	for _, entry := range entries {
		if entry.Method != nil {
			for _, arg := range entry.Method.Arguments {
				if arg.Name == name && arg.Type != nil {
					return arg.Type
				}
			}
			if t := ctx.variableTypeInEntries(entry.Method.Value, name); t != nil {
				return t
			}
		}
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					for _, arg := range field.Method.Arguments {
						if arg.Name == name && arg.Type != nil {
							return arg.Type
						}
					}
					if t := ctx.variableTypeInEntries(field.Method.Value, name); t != nil {
						return t
					}
				}
			}
		}
		if entry.Field != nil && entry.Field.Name == name {
			if entry.Field.Type != nil {
				return entry.Field.Type
			}
			if entry.Field.Value != nil {
				if t := ctx.SemanticGraph.TypeOfExpression(entry.Field.Value); t != nil {
					return t
				}
			}
		}
		if entry.If != nil {
			if t := ctx.variableTypeInIf(entry.If, name); t != nil {
				return t
			}
		}
		if entry.Loop != nil {
			if t := ctx.variableTypeInEntries(entry.Loop.Value, name); t != nil {
				return t
			}
		}
	}
	return nil
}

func (ctx *AnalysisContext) variableTypeInIf(stmt *tokens.If, name string) *tokens.TypeRef {
	if stmt == nil {
		return nil
	}
	if t := ctx.variableTypeInEntries(stmt.Value, name); t != nil {
		return t
	}
	elseIf := stmt.ElseIf
	for elseIf != nil {
		if t := ctx.variableTypeInEntries(elseIf.Value, name); t != nil {
			return t
		}
		elseIf = elseIf.ElseIf
	}
	if stmt.Else != nil {
		if t := ctx.variableTypeInEntries(stmt.Else.Value, name); t != nil {
			return t
		}
	}
	return nil
}
