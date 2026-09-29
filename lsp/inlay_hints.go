package main

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

type inlayHintParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Range        protocol.Range                  `json:"range"`
}

type inlayHint struct {
	Position protocol.Position `json:"position"`
	Label    string            `json:"label"`
	Kind     int               `json:"kind"`
}

func positionWithin(position protocol.Position, rng protocol.Range) bool {
	if position.Line < rng.Start.Line || position.Line > rng.End.Line {
		return false
	}
	if position.Line == rng.Start.Line && position.Character < rng.Start.Character {
		return false
	}
	return position.Line != rng.End.Line || position.Character <= rng.End.Character
}

func appendFieldHint(result *[]inlayHint, field *tokens.Field, graph *semantic.Program, content string, rng protocol.Range) {
	if field == nil || field.Type != nil || field.Value == nil {
		return
	}
	typ := graph.TypeOfExpression(field.Value)
	if typ == nil || typ.Type == "void" {
		return
	}
	start := symbolNameOffset(content, field.Pos, field.Name)
	if start < 0 {
		return
	}
	position := sourcePosition(content, start+len(field.Name))
	if !positionWithin(position, rng) {
		return
	}
	*result = append(*result, inlayHint{Position: position, Label: ": " + analysis.FormatTypeRef(typ), Kind: 1})
}

func appendElseIfHints(result *[]inlayHint, branch *tokens.ElseIf, graph *semantic.Program, content string, rng protocol.Range) {
	for branch != nil {
		appendEntryHints(result, branch.Value, graph, content, rng)
		if branch.Else != nil {
			appendEntryHints(result, branch.Else.Value, graph, content, rng)
		}
		branch = branch.ElseIf
	}
}

func appendEntryHints(result *[]inlayHint, entries []*tokens.Entry, graph *semantic.Program, content string, rng protocol.Range) {
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		appendFieldHint(result, entry.Field, graph, content, rng)
		if entry.If != nil {
			appendEntryHints(result, entry.If.Value, graph, content, rng)
			appendElseIfHints(result, entry.If.ElseIf, graph, content, rng)
			if entry.If.Else != nil {
				appendEntryHints(result, entry.If.Else.Value, graph, content, rng)
			}
		}
		if entry.Loop != nil {
			appendEntryHints(result, entry.Loop.Value, graph, content, rng)
		}
		if entry.UnsafeBlock != nil {
			appendEntryHints(result, entry.UnsafeBlock.Body, graph, content, rng)
		}
	}
}

func inlayHints(doc *Document, rng protocol.Range) []inlayHint {
	result := make([]inlayHint, 0)
	if doc == nil || doc.Analysis == nil || doc.Analysis.MainFile == nil || doc.Analysis.SemanticGraph == nil {
		return result
	}
	graph := doc.Analysis.SemanticGraph
	for _, entry := range doc.Analysis.MainFile.Entries {
		if entry == nil {
			continue
		}
		appendFieldHint(&result, entry.Field, graph, doc.Content, rng)
		if entry.Method != nil {
			appendEntryHints(&result, entry.Method.Value, graph, doc.Content, rng)
		}
		if entry.Class != nil {
			for _, member := range entry.Class.Fields {
				if member != nil && member.Method != nil {
					appendEntryHints(&result, member.Method.Value, graph, doc.Content, rng)
				}
			}
		}
		if entry.Implementation != nil {
			for _, member := range entry.Implementation.GetFields() {
				if member != nil {
					appendEntryHints(&result, member.Value, graph, doc.Content, rng)
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Position.Line != result[j].Position.Line {
			return result[i].Position.Line < result[j].Position.Line
		}
		return result[i].Position.Character < result[j].Position.Character
	})
	return result
}

func (s *Server) handleInlayHints(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params inlayHintParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	doc, ok := s.documents.Get(params.TextDocument.URI)
	if !ok {
		return reply(ctx, []inlayHint{}, nil)
	}
	return reply(ctx, inlayHints(doc, params.Range), nil)
}
