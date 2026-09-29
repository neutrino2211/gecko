package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *Server) handleCodeAction(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("codeAction unmarshal error: %v", err)
		return reply(ctx, nil, err)
	}

	uri := params.TextDocument.URI
	log.Printf("CodeAction request for %s, range %v", uri, params.Range)

	doc, ok := s.documents.Get(uri)
	if !ok {
		log.Printf("Document not found for codeAction: %s", uri)
		return reply(ctx, []protocol.CodeAction{}, nil)
	}

	actions := GetCodeActions(doc.Content, uriToPath(string(uri)), params.Range, params.Context.Diagnostics)
	if direct, err := s.directForeignDeclarationActions(ctx, doc, params.Context.Diagnostics); err != nil {
		log.Printf("direct foreign migration unavailable: %v", err)
	} else {
		actions = append(actions, direct...)
	}
	if action, err := s.allForeignDeclarationsAction(ctx, doc, params.Context.Diagnostics); err != nil {
		log.Printf("foreign migration unavailable: %v", err)
	} else if action != nil {
		actions = append(actions, *action)
	}
	actions = codeActionsForKinds(actions, params.Context.Only)
	log.Printf("Found %d code actions", len(actions))

	return reply(ctx, actions, nil)
}

func codeActionsForKinds(actions []protocol.CodeAction, only []protocol.CodeActionKind) []protocol.CodeAction {
	if len(only) == 0 {
		return actions
	}

	filtered := make([]protocol.CodeAction, 0, len(actions))
	for _, action := range actions {
		for _, kind := range only {
			if action.Kind == kind || strings.HasPrefix(string(action.Kind), string(kind)+".") {
				filtered = append(filtered, action)
				break
			}
		}
	}
	return filtered
}
