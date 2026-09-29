package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func sourceOffset(content string, position protocol.Position) int {
	lines := strings.SplitAfter(content, "\n")
	if int(position.Line) >= len(lines) {
		return len(content)
	}
	offset := 0
	for _, line := range lines[:position.Line] {
		offset += len(line)
	}
	return offset + byteColumn(content, position)
}

func sourcePosition(content string, offset int) protocol.Position {
	if offset > len(content) {
		offset = len(content)
	}
	if offset < 0 {
		offset = 0
	}
	before := content[:offset]
	line := strings.Count(before, "\n")
	if newline := strings.LastIndexByte(before, '\n'); newline >= 0 {
		before = before[newline+1:]
	}
	return protocol.Position{Line: uint32(line), Character: uint32(len(utf16.Encode([]rune(before))))}
}

func occurrenceContent(ctx *analysis.AnalysisContext, path string) (string, bool) {
	if ctx == nil || ctx.MainFile == nil {
		return "", false
	}
	visited := make(map[*tokens.File]bool)
	var find func(*tokens.File) (string, bool)
	find = func(file *tokens.File) (string, bool) {
		if file == nil || visited[file] {
			return "", false
		}
		visited[file] = true
		if filepath.Clean(file.Path) == path {
			return file.Content, true
		}
		for _, imported := range file.Imports {
			if content, ok := find(imported); ok {
				return content, true
			}
		}
		return "", false
	}
	return find(ctx.MainFile)
}

func occurrenceLocation(ctx *analysis.AnalysisContext, occurrence semantic.Occurrence) (protocol.Location, bool) {
	content, ok := occurrenceContent(ctx, occurrence.FilePath)
	if !ok || occurrence.Start < 0 || occurrence.End > len(content) {
		return protocol.Location{}, false
	}
	return protocol.Location{
		URI: pathToURI(occurrence.FilePath),
		Range: protocol.Range{
			Start: sourcePosition(content, occurrence.Start),
			End:   sourcePosition(content, occurrence.End),
		},
	}, true
}

func symbolAt(ctx *analysis.AnalysisContext, path, content string, position protocol.Position) (semantic.Occurrence, *semantic.Symbol, bool) {
	if ctx == nil || ctx.SemanticGraph == nil {
		return semantic.Occurrence{}, nil, false
	}
	occurrence, ok := ctx.SemanticGraph.OccurrenceAt(path, sourceOffset(content, position))
	if !ok {
		return semantic.Occurrence{}, nil, false
	}
	symbol := ctx.SemanticGraph.SymbolByID(occurrence.SymbolID)
	return occurrence, symbol, symbol != nil
}

func localReferences(ctx *analysis.AnalysisContext, path, content string, position protocol.Position, includeDeclaration bool) []protocol.Location {
	occurrence, symbol, ok := symbolAt(ctx, path, content, position)
	if !ok || !symbol.Local || !completeLocalIndex(ctx.SemanticGraph, path, content, symbol.Name) {
		return nil
	}
	result := make([]protocol.Location, 0)
	for _, reference := range ctx.SemanticGraph.OccurrencesFor(occurrence.SymbolID, includeDeclaration) {
		if location, ok := occurrenceLocation(ctx, reference); ok {
			result = append(result, location)
		}
	}
	return result
}

func semanticDefinition(ctx *analysis.AnalysisContext, path, content string, position protocol.Position) *protocol.Location {
	occurrence, symbol, ok := symbolAt(ctx, path, content, position)
	if !ok || symbol.Kind == semantic.SymbolVariable && !symbol.Local {
		return nil
	}
	for _, reference := range ctx.SemanticGraph.OccurrencesFor(occurrence.SymbolID, true) {
		if reference.Declaration {
			if location, found := occurrenceLocation(ctx, reference); found {
				return &location
			}
		}
	}
	return nil
}

func renameLocal(ctx *analysis.AnalysisContext, path, content string, position protocol.Position, newName string) (*protocol.WorkspaceEdit, error) {
	if !validRename(newName) {
		return nil, fmt.Errorf("invalid Gecko identifier %q", newName)
	}
	occurrence, symbol, ok := symbolAt(ctx, path, content, position)
	if !ok || !symbol.Local {
		return nil, nil
	}
	if !completeLocalIndex(ctx.SemanticGraph, path, content, symbol.Name) {
		return nil, fmt.Errorf("cannot rename %q while some uses are unresolved", symbol.Name)
	}
	for _, other := range ctx.SemanticGraph.Symbols {
		if other.ID != symbol.ID && other.Name == newName && !other.Local {
			return nil, fmt.Errorf("%q already names a symbol", newName)
		}
		owner := symbol.FullName
		if index := strings.LastIndex(owner, "::"); index >= 0 {
			owner = owner[:index]
		}
		otherOwner := other.FullName
		if index := strings.LastIndex(otherOwner, "::"); index >= 0 {
			otherOwner = otherOwner[:index]
		}
		if other.ID != symbol.ID && other.Local && other.Name == newName {
			if owner == otherOwner || symbol.Kind == other.Kind && (symbol.Kind == semantic.SymbolTypeParam || symbol.Kind == semantic.SymbolVariable) &&
				(strings.HasPrefix(owner, otherOwner+"::") || strings.HasPrefix(otherOwner, owner+"::")) {
				return nil, fmt.Errorf("%q already names a local symbol", newName)
			}
		}
	}
	result := &protocol.WorkspaceEdit{Changes: make(map[protocol.DocumentURI][]protocol.TextEdit)}
	for _, reference := range ctx.SemanticGraph.OccurrencesFor(occurrence.SymbolID, true) {
		if location, ok := occurrenceLocation(ctx, reference); ok {
			result.Changes[location.URI] = append(result.Changes[location.URI], protocol.TextEdit{Range: location.Range, NewText: newName})
		}
	}
	return result, nil
}

func completeLocalIndex(graph *semantic.Program, path, content, name string) bool {
	for offset := 0; offset < len(content); {
		if content[offset] == '/' && offset+1 < len(content) && content[offset+1] == '/' {
			for offset < len(content) && content[offset] != '\n' {
				offset++
			}
			continue
		}
		if content[offset] == '"' || content[offset] == '`' {
			quote := content[offset]
			offset++
			for offset < len(content) && content[offset] != quote {
				if content[offset] == '\\' && quote == '"' {
					offset++
				}
				offset++
			}
			offset++
			continue
		}
		if !identifierStart(content[offset]) {
			offset++
			continue
		}
		start := offset
		for offset < len(content) && identifierPart(content[offset]) {
			offset++
		}
		if content[start:offset] == name {
			if _, ok := graph.OccurrenceAt(path, start); !ok {
				return false
			}
		}
	}
	return true
}

func identifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func identifierPart(value byte) bool {
	return identifierStart(value) || value >= '0' && value <= '9'
}

func validRename(name string) bool {
	switch name {
	case "package", "import", "class", "trait", "impl", "func", "let", "const", "if", "else", "for", "of", "in", "while", "loop", "return", "break", "continue", "unsafe", "with", "true", "false", "nil", "null", "self", "Self", "out", "defer", "match", "enum", "declare", "external", "public", "private", "protected":
		return false
	}
	if name == "" || !identifierStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !identifierPart(name[i]) {
			return false
		}
	}
	_, err := parser.Parser.ParseString("rename.gecko", "package rename\nfunc check(): void { let "+name+": int32 = 0 }\n")
	return err == nil
}

func (s *Server) handleReferences(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.ReferenceParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	doc, ok := s.documents.Get(params.TextDocument.URI)
	if !ok {
		return reply(ctx, []protocol.Location{}, nil)
	}
	locations := localReferences(doc.Analysis, uriToPath(string(doc.URI)), doc.Content, params.Position, params.Context.IncludeDeclaration)
	if locations == nil {
		var err error
		locations, _, _, err = s.workspaceReferences(ctx, doc, params.Position, params.Context.IncludeDeclaration, false)
		if err != nil {
			return reply(ctx, nil, err)
		}
	}
	if locations == nil {
		locations = []protocol.Location{}
	}
	return reply(ctx, locations, nil)
}

func (s *Server) handlePrepareRename(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.PrepareRenameParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	doc, ok := s.documents.Get(params.TextDocument.URI)
	if !ok {
		return reply(ctx, nil, nil)
	}
	occurrence, symbol, ok := symbolAt(doc.Analysis, uriToPath(string(doc.URI)), doc.Content, params.Position)
	if !ok {
		return reply(ctx, nil, nil)
	}
	if symbol.Local {
		if !completeLocalIndex(doc.Analysis.SemanticGraph, uriToPath(string(doc.URI)), doc.Content, symbol.Name) {
			return reply(ctx, nil, nil)
		}
	} else {
		if symbol.Kind != semantic.SymbolFunction && symbol.Kind != semantic.SymbolClass && symbol.Kind != semantic.SymbolTrait && symbol.Kind != semantic.SymbolField && symbol.Kind != semantic.SymbolEnum && symbol.Kind != semantic.SymbolEnumCase {
			return reply(ctx, nil, nil)
		}
		locations, _, _, err := s.workspaceReferences(ctx, doc, params.Position, true, true)
		if err != nil || len(locations) == 0 {
			return reply(ctx, nil, nil)
		}
	}
	location, ok := occurrenceLocation(doc.Analysis, occurrence)
	if !ok {
		return reply(ctx, nil, nil)
	}
	return reply(ctx, location.Range, nil)
}

func (s *Server) handleRename(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.RenameParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	doc, ok := s.documents.Get(params.TextDocument.URI)
	if !ok {
		return reply(ctx, nil, nil)
	}
	_, symbol, ok := symbolAt(doc.Analysis, uriToPath(string(doc.URI)), doc.Content, params.Position)
	if !ok {
		return reply(ctx, nil, nil)
	}
	var edit *protocol.WorkspaceEdit
	var err error
	if symbol.Local {
		edit, err = renameLocal(doc.Analysis, uriToPath(string(doc.URI)), doc.Content, params.Position, params.NewName)
	} else {
		edit, err = s.renameWorkspaceSymbol(ctx, doc, params.Position, params.NewName)
	}
	return reply(ctx, edit, err)
}
