package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func symbolNameOffset(content string, pos lexer.Position, name string) int {
	if name == "" || pos.Offset < 0 || pos.Offset >= len(content) {
		return -1
	}
	limit := pos.Offset + 512
	if limit > len(content) {
		limit = len(content)
	}
	for offset := pos.Offset; offset+len(name) <= limit; offset++ {
		if content[offset] == '/' && offset+1 < limit && content[offset+1] == '/' {
			for offset < limit && content[offset] != '\n' {
				offset++
			}
			continue
		}
		if content[offset] == '"' || content[offset] == '`' {
			quote := content[offset]
			offset++
			for offset < limit && content[offset] != quote {
				if content[offset] == '\\' && quote == '"' {
					offset++
				}
				offset++
			}
			continue
		}
		if content[offset:offset+len(name)] != name {
			continue
		}
		if offset > 0 && identifierPart(content[offset-1]) || offset+len(name) < len(content) && identifierPart(content[offset+len(name)]) {
			continue
		}
		return offset
	}
	return -1
}

func symbolBodyEnd(content string, afterName int) int {
	open := strings.IndexByte(content[afterName:], '{')
	if open < 0 {
		return afterName
	}
	depth := 0
	for offset := afterName + open; offset < len(content); offset++ {
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
			continue
		}
		if content[offset] == '{' {
			depth++
		} else if content[offset] == '}' {
			depth--
			if depth == 0 {
				return offset + 1
			}
		}
	}
	return afterName
}

func declarationSymbol(content string, pos lexer.Position, name string, kind protocol.SymbolKind, container bool) (protocol.DocumentSymbol, bool) {
	start := symbolNameOffset(content, pos, name)
	if start < 0 {
		return protocol.DocumentSymbol{}, false
	}
	selection := protocol.Range{Start: sourcePosition(content, start), End: sourcePosition(content, start+len(name))}
	rng := selection
	if container {
		end := symbolBodyEnd(content, start+len(name))
		if end > start+len(name) {
			rng = protocol.Range{Start: sourcePosition(content, pos.Offset), End: sourcePosition(content, end)}
		}
	}
	return protocol.DocumentSymbol{Name: name, Kind: kind, Range: rng, SelectionRange: selection}, true
}

func fieldDocumentSymbol(content string, field *tokens.Field, member bool) (protocol.DocumentSymbol, bool) {
	if field == nil {
		return protocol.DocumentSymbol{}, false
	}
	kind := protocol.SymbolKindVariable
	if member {
		kind = protocol.SymbolKindField
	}
	if field.Mutability == "const" {
		kind = protocol.SymbolKindConstant
	}
	return declarationSymbol(content, field.Pos, field.Name, kind, false)
}

func methodDocumentSymbol(content string, method *tokens.Method, member bool) (protocol.DocumentSymbol, bool) {
	if method == nil {
		return protocol.DocumentSymbol{}, false
	}
	kind := protocol.SymbolKindFunction
	if member {
		kind = protocol.SymbolKindMethod
	}
	return declarationSymbol(content, method.Pos, method.Name, kind, false)
}

func enumCaseSymbols(content string, symbol protocol.DocumentSymbol, cases []string) []protocol.DocumentSymbol {
	cursor := sourceOffset(content, symbol.SelectionRange.End)
	end := sourceOffset(content, symbol.Range.End)
	result := make([]protocol.DocumentSymbol, 0, len(cases))
	for _, name := range cases {
		start := symbolNameOffset(content, lexer.Position{Offset: cursor}, name)
		if start < 0 || start+len(name) > end {
			break
		}
		rng := protocol.Range{Start: sourcePosition(content, start), End: sourcePosition(content, start+len(name))}
		result = append(result, protocol.DocumentSymbol{Name: name, Kind: protocol.SymbolKindEnumMember, Range: rng, SelectionRange: rng})
		cursor = start + len(name)
	}
	return result
}

func implementationDocumentSymbol(content string, implementation *tokens.Implementation) (protocol.DocumentSymbol, bool) {
	if implementation == nil {
		return protocol.DocumentSymbol{}, false
	}
	name := implementation.GetName()
	symbol, ok := declarationSymbol(content, implementation.Pos, name, protocol.SymbolKindObject, true)
	if !ok {
		return protocol.DocumentSymbol{}, false
	}
	symbol.Name = "impl " + name
	if target := implementation.GetFor(); target != "" {
		symbol.Name += " for " + target
	}
	for _, member := range implementation.GetFields() {
		if member == nil {
			continue
		}
		if child, ok := declarationSymbol(content, member.Pos, member.Name, protocol.SymbolKindMethod, false); ok {
			symbol.Children = append(symbol.Children, child)
		}
	}
	return symbol, true
}

func documentSymbols(file *tokens.File, content string) []protocol.DocumentSymbol {
	result := make([]protocol.DocumentSymbol, 0)
	if file == nil {
		return result
	}
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		switch {
		case entry.Class != nil:
			class := entry.Class
			symbol, ok := declarationSymbol(content, class.Pos, class.Name, protocol.SymbolKindClass, true)
			if !ok {
				continue
			}
			for _, member := range class.Fields {
				if member == nil {
					continue
				}
				if child, ok := fieldDocumentSymbol(content, member.Field, true); ok {
					symbol.Children = append(symbol.Children, child)
				}
				if child, ok := methodDocumentSymbol(content, member.Method, true); ok {
					symbol.Children = append(symbol.Children, child)
				}
			}
			result = append(result, symbol)
		case entry.Trait != nil:
			trait := entry.Trait
			symbol, ok := declarationSymbol(content, trait.Pos, trait.Name, protocol.SymbolKindInterface, true)
			if !ok {
				continue
			}
			for _, member := range trait.Fields {
				if member == nil {
					continue
				}
				if child, ok := declarationSymbol(content, member.Pos, member.Name, protocol.SymbolKindMethod, false); ok {
					symbol.Children = append(symbol.Children, child)
				}
			}
			result = append(result, symbol)
		case entry.Enum != nil:
			if symbol, ok := declarationSymbol(content, entry.Enum.Pos, entry.Enum.Name, protocol.SymbolKindEnum, true); ok {
				symbol.Children = enumCaseSymbols(content, symbol, entry.Enum.Cases)
				result = append(result, symbol)
			}
		case entry.Implementation != nil:
			if symbol, ok := implementationDocumentSymbol(content, entry.Implementation); ok {
				result = append(result, symbol)
			}
		case entry.Method != nil:
			if symbol, ok := methodDocumentSymbol(content, entry.Method, false); ok {
				result = append(result, symbol)
			}
		case entry.Field != nil:
			if symbol, ok := fieldDocumentSymbol(content, entry.Field, false); ok {
				result = append(result, symbol)
			}
		case entry.Declaration != nil:
			if symbol, ok := methodDocumentSymbol(content, entry.Declaration.Method, false); ok {
				result = append(result, symbol)
			}
			if symbol, ok := fieldDocumentSymbol(content, entry.Declaration.Field, false); ok {
				result = append(result, symbol)
			}
		}
	}
	return result
}

func (s *Server) handleDocumentSymbol(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentSymbolParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	doc, ok := s.documents.Get(params.TextDocument.URI)
	if !ok || doc.Analysis == nil {
		return reply(ctx, []protocol.DocumentSymbol{}, nil)
	}
	return reply(ctx, documentSymbols(doc.Analysis.MainFile, doc.Content), nil)
}
