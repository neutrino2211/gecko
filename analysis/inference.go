// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

package analysis

import (
	"fmt"

	"github.com/neutrino2211/gecko/tokens"
)

// InferExpressionType infers the type of an expression
func InferExpressionType(expr *tokens.Expression, ctx *AnalysisContext) string {
	if expr == nil {
		return "unknown"
	}
	if ctx != nil && ctx.SemanticGraph != nil {
		if t := ctx.SemanticGraph.TypeOfExpression(expr); t != nil {
			return FormatTypeRef(t)
		}
	}

	// Use the tokens package's inference first
	resolveSymbol := func(name string) *tokens.TypeRef {
		// Look up in context
		if varInfo := ctx.findVariable(name); varInfo != nil {
			return varInfo.Type
		}
		return nil
	}

	if inferred := tokens.InferType(expr, resolveSymbol); inferred != nil {
		return FormatTypeRef(inferred)
	}

	// Fall back to basic inference
	primary := getPrimaryFromExpr(expr)
	if primary == nil || primary.Literal == nil {
		return "unknown"
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

	if lit.FuncCall != nil {
		return ctx.inferFuncCallType(lit.FuncCall)
	}

	if lit.Symbol != "" {
		if varInfo := ctx.findVariable(lit.Symbol); varInfo != nil && varInfo.Type != nil {
			return FormatTypeRef(varInfo.Type)
		}
	}

	return "unknown"
}

// VariableInfo holds information about a variable
type VariableInfo struct {
	Name       string
	Type       *tokens.TypeRef
	Mutability string
	Line       int
}

// findVariable looks up a variable in the analysis context
func (ctx *AnalysisContext) findVariable(name string) *VariableInfo {
	// Search in main file
	for _, entry := range ctx.MainFile.Entries {
		if entry.Method != nil {
			// Check arguments
			for _, arg := range entry.Method.Arguments {
				if arg.Name == name {
					return &VariableInfo{Name: name, Type: arg.Type, Line: arg.Pos.Line}
				}
			}
			// Check local variables
			if varInfo := findVarInEntries(entry.Method.Value, name); varInfo != nil {
				return varInfo
			}
		}
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field.Method != nil {
					for _, arg := range field.Method.Arguments {
						if arg.Name == name {
							return &VariableInfo{Name: name, Type: arg.Type, Line: arg.Pos.Line}
						}
					}
					if varInfo := findVarInEntries(field.Method.Value, name); varInfo != nil {
						return varInfo
					}
				}
			}
		}
		if entry.Field != nil && entry.Field.Name == name {
			return &VariableInfo{
				Name:       name,
				Type:       entry.Field.Type,
				Mutability: entry.Field.Mutability,
				Line:       entry.Field.Pos.Line,
			}
		}
	}
	return nil
}

func findVarInEntries(entries []*tokens.Entry, name string) *VariableInfo {
	for _, entry := range entries {
		if entry.Field != nil && entry.Field.Name == name {
			return &VariableInfo{
				Name:       name,
				Type:       entry.Field.Type,
				Mutability: entry.Field.Mutability,
				Line:       entry.Field.Pos.Line,
			}
		}
		if entry.If != nil {
			if v := findVarInEntries(entry.If.Value, name); v != nil {
				return v
			}
		}
		if entry.Loop != nil {
			if v := findVarInEntries(entry.Loop.Value, name); v != nil {
				return v
			}
		}
	}
	return nil
}

func (ctx *AnalysisContext) inferFuncCallType(f *tokens.FuncCall) string {
	if f == nil {
		return "unknown"
	}

	// Static method call
	if f.StaticType != "" {
		if classScope, ok := ctx.RootScope.Classes[f.StaticType]; ok {
			if method, ok := classScope.Methods[f.Function]; ok {
				return method.Type
			}
		}
		return f.StaticType
	}

	// Regular function call
	if method, ok := ctx.RootScope.Methods[f.Function]; ok {
		return method.Type
	}

	return fmt.Sprintf("(result of %s())", f.Function)
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
