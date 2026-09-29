package analysis

import (
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
	"strings"
)

func (ctx *AnalysisContext) Offset(line, col int) int {
	if ctx == nil || ctx.MainFile == nil || line < 1 {
		return -1
	}
	lines := strings.Split(ctx.MainFile.Content, "\n")
	if line > len(lines) {
		return -1
	}
	offset := 0
	for _, text := range lines[:line-1] {
		offset += len(text) + 1
	}
	if col <= 0 || col > len(lines[line-1])+1 {
		return offset + len(lines[line-1])
	}
	return offset + col - 1
}

func (ctx *AnalysisContext) VariableType(name string, line, col int) *tokens.TypeRef {
	if ctx == nil || ctx.SemanticGraph == nil {
		return nil
	}
	offset := ctx.Offset(line, col)
	if symbol := ctx.SemanticGraph.SymbolAt(ctx.FilePath, offset); symbol != nil && symbol.Name == name {
		return semantic.CloneTypeRef(symbol.Type)
	}
	if symbol := ctx.SemanticGraph.LookupSymbol(ctx.FilePath, offset, name); symbol != nil {
		return symbol.Type
	}
	return nil
}
