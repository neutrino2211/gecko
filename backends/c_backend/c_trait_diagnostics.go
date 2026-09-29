package cbackend

import (
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	geckoerrors "github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

func traitMemberNameSpan(source string, pos lexer.Position, name string) (lexer.Position, int) {
	if pos.Offset < 0 || pos.Offset >= len(source) {
		return pos, 0
	}
	line := source[pos.Offset:]
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end]
	}
	keyword := strings.Index(line, "func")
	if keyword < 0 {
		return pos, 0
	}
	nameStart := keyword + len("func")
	for nameStart < len(line) && (line[nameStart] == ' ' || line[nameStart] == '\t') {
		nameStart++
	}
	if !strings.HasPrefix(line[nameStart:], name) {
		return pos, 0
	}
	pos.Offset += nameStart
	pos.Column += nameStart
	return pos, pos.Offset + len(name)
}

func decorateTraitOverrideDiagnostic(scope *ast.Ast, message *geckoerrors.CompileTimeMessage, parent, child *tokens.ImplementationField, reason string) {
	if scope.ErrorScope.Source != nil {
		message.Pos, message.EndOffset = traitMemberNameSpan(*scope.ErrorScope.Source, child.Pos, child.Name)
	}
	parentPath := parent.Pos.Filename
	if parentPath == "" {
		parentPath = scope.ErrorScope.SourceName
	}
	message.Related = append(message.Related, geckoerrors.RelatedLocation{
		Span:    geckoerrors.SourceSpan{File: parentPath, Start: parent.Pos.Offset, End: parent.Pos.Offset + len("func")},
		Message: "Inherited method declared here",
	})
	if !strings.HasPrefix(reason, "return type mismatch") || scope.ErrorScope.Source == nil || parent.Type == nil || child.Type == nil {
		return
	}
	if typeRefSignature(parent.Type) != parent.Type.Type || typeRefSignature(child.Type) != child.Type.Type {
		return
	}
	start := child.Type.Pos.Offset
	end := start + len(child.Type.Type)
	if start < 0 || end > len(*scope.ErrorScope.Source) || (*scope.ErrorScope.Source)[start:end] != child.Type.Type {
		return
	}
	message.Fixes = append(message.Fixes, geckoerrors.SuggestedFix{
		Title:   "Change return type to " + parent.Type.Type,
		Span:    geckoerrors.SourceSpan{File: scope.ErrorScope.SourceName, Start: start, End: end},
		NewText: parent.Type.Type,
	})
}
