package main

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

type foreignSpan struct {
	start  int
	end    int
	indent string
}

type foreignTarget struct {
	name string
	span foreignSpan
}

type foreignPart struct {
	span    foreignSpan
	member  string
	wrapper string
	target  foreignTarget
}

func foreignDeclarationSpan(content string, declaration *tokens.Declaration) (foreignSpan, bool) {
	if declaration == nil {
		return foreignSpan{}, false
	}
	start, end := declaration.Pos.Offset, declaration.EndPos.Offset
	if start < 0 || end <= start || end > len(content) {
		return foreignSpan{}, false
	}
	lineStart := strings.LastIndexByte(content[:start], '\n') + 1
	indent := content[lineStart:start]
	if strings.Trim(indent, " \t") != "" {
		return foreignSpan{}, false
	}
	return foreignSpan{start: lineStart, end: end, indent: indent}, true
}

func foreignDeclarationParts(content string, span foreignSpan, declaration *tokens.Declaration, module string) (string, string, bool) {
	statement := strings.TrimSpace(content[span.start:span.end])
	if !strings.HasPrefix(statement, "declare external ") || strings.Contains(statement, "//") {
		return "", "", false
	}
	if declaration.ExternalType != nil {
		name := declaration.ExternalType.Name
		if statement != "declare external type "+name {
			return "", "", false
		}
		return "type " + name + " opaque", "", true
	}
	method := declaration.Method
	if method == nil || method.Visibility != "external" || len(method.TypeParams) > 0 || len(method.Attributes) > 0 || method.Throws != nil || method.Where != nil {
		return "", "", false
	}
	signature := strings.TrimPrefix(statement, "declare external ")
	if method.Variardic {
		signature = strings.TrimPrefix(signature, "variardic ")
		openParen := strings.IndexByte(signature, '(')
		closeParen := strings.LastIndexByte(signature, ')')
		if openParen < 0 || closeParen <= openParen {
			return "", "", false
		}
		separator := ", ..."
		if strings.TrimSpace(signature[openParen+1:closeParen]) == "" {
			separator = "..."
		}
		signature = signature[:closeParen] + separator + signature[closeParen:]
	}
	if !strings.HasPrefix(signature, "func "+method.Name+"(") || strings.ContainsAny(signature, "{}") {
		return "", "", false
	}
	for _, arg := range method.Arguments {
		if arg == nil || arg.Name == "" || arg.Variadic || arg.Default != nil || arg.Type == nil {
			return "", "", false
		}
	}
	if method.IsVariadic() {
		return signature, "", true
	}
	args := make([]string, 0, len(method.Arguments))
	for _, arg := range method.Arguments {
		if arg.Out {
			return signature, "", true
		}
		args = append(args, arg.Name)
	}
	call := fmt.Sprintf("%s.%s(%s)", module, method.Name, strings.Join(args, ", "))
	if method.Type != nil && (method.Type.Type != "void" || method.Type.Pointer) {
		call = "return " + call
	}
	return signature, fmt.Sprintf("public %s {\n%s    %s\n%s}", signature, span.indent, call, span.indent), true
}

func foreignBlock(indent, module string, members []string, wrapper string) string {
	var result strings.Builder
	result.WriteString(indent)
	result.WriteString("foreign \"c\" ")
	result.WriteString(module)
	result.WriteString(" {\n")
	for _, member := range members {
		for _, line := range strings.Split(member, "\n") {
			result.WriteString(indent)
			result.WriteString("    ")
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}
	result.WriteString(indent)
	result.WriteByte('}')
	if wrapper != "" {
		result.WriteString("\n\n")
		result.WriteString(indent)
		result.WriteString(wrapper)
	}
	return result.String()
}

func foreignTextEdit(content string, span foreignSpan, replacement string) protocol.TextEdit {
	if strings.Contains(content, "\r\n") {
		replacement = strings.ReplaceAll(replacement, "\n", "\r\n")
	}
	return protocol.TextEdit{
		Range: protocol.Range{
			Start: sourcePosition(content, span.start),
			End:   sourcePosition(content, span.end),
		},
		NewText: replacement,
	}
}

func foreignMigrationParts(content, module string, file *tokens.File) ([]foreignPart, bool) {
	parts := make([]foreignPart, 0)
	for _, entry := range file.Entries {
		if entry.Declaration == nil {
			continue
		}
		span, ok := foreignDeclarationSpan(content, entry.Declaration)
		if !ok {
			return nil, false
		}
		member, wrapper, ok := foreignDeclarationParts(content, span, entry.Declaration, module)
		if !ok {
			return nil, false
		}
		part := foreignPart{span: span, member: member, wrapper: wrapper}
		if entry.Declaration.Method != nil && wrapper == "" {
			part.target = foreignTarget{name: entry.Declaration.Method.Name, span: span}
		}
		parts = append(parts, part)
	}
	return parts, true
}

func foreignMigrationEdits(content, module string, parts []foreignPart) []protocol.TextEdit {
	edits := make([]protocol.TextEdit, 0)
	for index := 0; index < len(parts); {
		first := parts[index]
		end := first.span.end
		members := []string{first.member}
		wrappers := make([]string, 0)
		if first.wrapper != "" {
			wrappers = append(wrappers, first.wrapper)
		}
		index++
		for index < len(parts) && strings.TrimSpace(content[end:parts[index].span.start]) == "" && parts[index].span.indent == first.span.indent {
			members = append(members, parts[index].member)
			if parts[index].wrapper != "" {
				wrappers = append(wrappers, parts[index].wrapper)
			}
			end = parts[index].span.end
			index++
		}
		span := foreignSpan{start: first.span.start, end: end, indent: first.span.indent}
		replacement := foreignBlock(span.indent, module, members, strings.Join(wrappers, "\n\n"+span.indent))
		edits = append(edits, foreignTextEdit(content, span, replacement))
	}
	return edits
}
