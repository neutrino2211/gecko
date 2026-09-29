package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type resolvedToken struct {
	rng       protocol.Range
	kind      uint32
	modifiers uint32
}

const (
	semanticKeyword uint32 = iota + 10
	semanticComment
	semanticString
	semanticNumber
	semanticOperator
	semanticType
)

var geckoSyntaxKeywords = map[string]bool{
	"as": true, "break": true, "case": true, "class": true, "const": true,
	"continue": true, "declare": true, "defer": true, "else": true, "enum": true,
	"external": true, "false": true, "for": true, "foreign": true, "func": true,
	"if": true, "impl": true, "import": true, "in": true, "is": true,
	"let": true, "loop": true, "match": true, "nil": true, "null": true,
	"of": true, "or": true, "out": true, "package": true, "private": true,
	"protected": true, "public": true, "readonly": true, "return": true,
	"self": true, "Self": true, "static": true, "trait": true, "true": true,
	"try": true, "type": true, "unsafe": true, "use": true, "where": true,
	"while": true, "with": true, "withheader": true, "withlib": true,
	"withobject": true,
}

var geckoBuiltinTypes = map[string]bool{
	"bool": true, "char": true, "float32": true, "float64": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"isize": true, "string": true, "uint": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "usize": true,
	"void": true,
}

func semanticTokenLegend() protocol.SemanticTokensLegend {
	return protocol.SemanticTokensLegend{
		TokenTypes: []protocol.SemanticTokenTypes{
			protocol.SemanticTokenClass,
			protocol.SemanticTokenFunction,
			protocol.SemanticTokenMethod,
			protocol.SemanticTokenProperty,
			protocol.SemanticTokenVariable,
			protocol.SemanticTokenInterface,
			protocol.SemanticTokenNamespace,
			protocol.SemanticTokenTypeParameter,
			protocol.SemanticTokenEnum,
			protocol.SemanticTokenEnumMember,
			protocol.SemanticTokenKeyword,
			protocol.SemanticTokenComment,
			protocol.SemanticTokenString,
			protocol.SemanticTokenNumber,
			protocol.SemanticTokenOperator,
			protocol.SemanticTokenType,
		},
		TokenModifiers: []protocol.SemanticTokenModifiers{
			protocol.SemanticTokenModifierDeclaration,
			protocol.SemanticTokenModifierDocumentation,
		},
	}
}

func semanticTokenKind(kind semantic.SymbolKind) (uint32, bool) {
	switch kind {
	case semantic.SymbolClass:
		return 0, true
	case semantic.SymbolFunction:
		return 1, true
	case semantic.SymbolMethod:
		return 2, true
	case semantic.SymbolField:
		return 3, true
	case semantic.SymbolVariable:
		return 4, true
	case semantic.SymbolTrait:
		return 5, true
	case semantic.SymbolModule:
		return 6, true
	case semantic.SymbolTypeParam:
		return 7, true
	case semantic.SymbolEnum:
		return 8, true
	case semantic.SymbolEnumCase:
		return 9, true
	default:
		return 0, false
	}
}

func semanticTokens(doc *Document) protocol.SemanticTokens {
	result := protocol.SemanticTokens{Data: make([]uint32, 0)}
	if doc == nil {
		return result
	}
	tokens := make([]resolvedToken, 0)
	seen := make(map[protocol.Range]bool)
	if doc.Analysis != nil && doc.Analysis.SemanticGraph != nil {
		graph := doc.Analysis.SemanticGraph
		path := uriToPath(string(doc.URI))
		for _, occurrence := range graph.OccurrencesInFile(path) {
			symbol := graph.SymbolByID(occurrence.SymbolID)
			if symbol == nil || occurrence.Start < 0 || occurrence.End > len(doc.Content) {
				continue
			}
			kind, ok := semanticTokenKind(symbol.Kind)
			if !ok {
				continue
			}
			rng := protocol.Range{Start: sourcePosition(doc.Content, occurrence.Start), End: sourcePosition(doc.Content, occurrence.End)}
			if seen[rng] || rng.Start.Line != rng.End.Line || rng.Start.Character == rng.End.Character {
				continue
			}
			seen[rng] = true
			modifiers := uint32(0)
			if occurrence.Declaration {
				modifiers = 1
			}
			tokens = append(tokens, resolvedToken{rng: rng, kind: kind, modifiers: modifiers})
		}
	}
	var lexical []parser.SourceToken
	if doc.Analysis != nil && doc.Analysis.Frontend != nil && doc.Analysis.MainFile.Content == doc.Content {
		lexical = doc.Analysis.Frontend.LexicalTokens(uriToPath(string(doc.URI)))
	}
	if lexical == nil {
		lexical = parser.LexSource(uriToPath(string(doc.URI)), doc.Content)
	}
	for _, token := range lexical {
		kind, modifiers, ok := lexicalTokenKind(token)
		if ok {
			appendLexicalToken(doc.Content, token, kind, modifiers, &tokens, seen)
		}
	}
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].rng.Start.Line != tokens[j].rng.Start.Line {
			return tokens[i].rng.Start.Line < tokens[j].rng.Start.Line
		}
		return tokens[i].rng.Start.Character < tokens[j].rng.Start.Character
	})
	var previousLine, previousCharacter uint32
	for _, token := range tokens {
		line := token.rng.Start.Line
		character := token.rng.Start.Character
		deltaLine := line - previousLine
		deltaCharacter := character
		if deltaLine == 0 {
			deltaCharacter -= previousCharacter
		}
		result.Data = append(result.Data, deltaLine, deltaCharacter, token.rng.End.Character-character, token.kind, token.modifiers)
		previousLine = line
		previousCharacter = character
	}
	return result
}

func lexicalTokenKind(token parser.SourceToken) (uint32, uint32, bool) {
	switch token.Kind {
	case "DocComment":
		return semanticComment, 2, true
	case "Comment":
		return semanticComment, 0, true
	case "String", "BacktickString":
		return semanticString, 0, true
	case "Number":
		return semanticNumber, 0, true
	case "Ident":
		if geckoSyntaxKeywords[token.Value] {
			return semanticKeyword, 0, true
		}
		if geckoBuiltinTypes[token.Value] {
			return semanticType, 0, true
		}
	case "Punct":
		if strings.ContainsAny(token.Value, "<>=!+-*/%&|^~?:@") {
			return semanticOperator, 0, true
		}
	case "Ellipsis", "RangeDot", "DoubleColon", "LogicalAnd", "LogicalOr",
		"PlusAssign", "MinusAssign", "StarAssign", "SlashAssign", "AndAssign",
		"OrAssign", "XorAssign", "ModuloAssign", "Inc", "Dec", "Arrow":
		return semanticOperator, 0, true
	}
	return 0, 0, false
}

func appendLexicalToken(content string, token parser.SourceToken, kind, modifiers uint32, tokens *[]resolvedToken, seen map[protocol.Range]bool) {
	start := token.Offset
	end := start + len(token.Value)
	if start < 0 || end > len(content) {
		return
	}
	for start < end {
		segmentEnd := end
		if newline := strings.IndexByte(content[start:end], '\n'); newline >= 0 {
			segmentEnd = start + newline
		}
		if segmentEnd > start {
			rng := protocol.Range{Start: sourcePosition(content, start), End: sourcePosition(content, segmentEnd)}
			if !seen[rng] {
				seen[rng] = true
				*tokens = append(*tokens, resolvedToken{rng: rng, kind: kind, modifiers: modifiers})
			}
		}
		start = segmentEnd + 1
	}
}

func (s *Server) handleSemanticTokens(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.SemanticTokensParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	doc, ok := s.documents.Get(params.TextDocument.URI)
	if !ok {
		return reply(ctx, protocol.SemanticTokens{Data: []uint32{}}, nil)
	}
	return reply(ctx, semanticTokens(doc), nil)
}
