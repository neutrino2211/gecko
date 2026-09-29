package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

func (s *Server) allForeignDeclarationsAction(ctx context.Context, doc *Document, diagnostics []protocol.Diagnostic) (*protocol.CodeAction, error) {
	path := filepath.Clean(uriToPath(string(doc.URI)))
	file, err := parser.Parser.ParseString(path, doc.Content)
	if err != nil {
		return nil, nil
	}
	module := foreignMigrationModule(doc.Content, file)
	parts, ok := foreignMigrationParts(doc.Content, module, file)
	if !ok || len(parts) == 0 {
		return nil, nil
	}
	selected := make([]protocol.Diagnostic, 0)
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
			selected = append(selected, diagnostic)
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	return s.foreignMigrationAction(ctx, doc, file, module, parts, selected, "Convert all declarations in file to foreign C module")
}

func (s *Server) directForeignDeclarationActions(ctx context.Context, doc *Document, diagnostics []protocol.Diagnostic) ([]protocol.CodeAction, error) {
	path := filepath.Clean(uriToPath(string(doc.URI)))
	file, err := parser.Parser.ParseString(path, doc.Content)
	if err != nil {
		return nil, nil
	}
	module := foreignMigrationModule(doc.Content, file)
	parts, ok := foreignMigrationParts(doc.Content, module, file)
	if !ok {
		return nil, nil
	}
	actions := make([]protocol.CodeAction, 0)
	for _, diagnostic := range diagnostics {
		if !strings.Contains(diagnostic.Message, "`declare external` is deprecated") {
			continue
		}
		for _, part := range parts {
			if part.target.name == "" || sourcePosition(doc.Content, part.span.start).Line != diagnostic.Range.Start.Line {
				continue
			}
			selected := foreignRequiredParts(parts, part)
			action, err := s.foreignMigrationAction(ctx, doc, file, module, selected, []protocol.Diagnostic{diagnostic}, "Convert declaration to foreign C module")
			if err != nil {
				return actions, err
			}
			if action != nil {
				actions = append(actions, *action)
			}
			break
		}
	}
	return actions, nil
}

func foreignRequiredParts(parts []foreignPart, target foreignPart) []foreignPart {
	selected := make([]foreignPart, 0)
	for _, part := range parts {
		if part.span == target.span {
			selected = append(selected, part)
			continue
		}
		if !strings.HasPrefix(part.member, "type ") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(part.member, "type "), " opaque")
		if hasIdentifier(target.member, name) {
			selected = append(selected, part)
		}
	}
	return selected
}

func (s *Server) foreignMigrationAction(ctx context.Context, doc *Document, file *tokens.File, module string, parts []foreignPart, diagnostics []protocol.Diagnostic, title string) (*protocol.CodeAction, error) {
	changes := map[protocol.DocumentURI][]protocol.TextEdit{
		doc.URI: foreignMigrationEdits(doc.Content, module, parts),
	}
	if err := s.addForeignReferenceEdits(ctx, doc, file, parts, module, changes); err != nil {
		return nil, err
	}
	if err := validateForeignEdits(changes, s.documents.Snapshot()); err != nil {
		return nil, err
	}
	return &protocol.CodeAction{
		Title:       title,
		Kind:        protocol.QuickFix,
		Diagnostics: diagnostics,
		Edit:        &protocol.WorkspaceEdit{Changes: changes},
	}, nil
}

func (s *Server) addForeignReferenceEdits(ctx context.Context, doc *Document, file *tokens.File, parts []foreignPart, module string, changes map[protocol.DocumentURI][]protocol.TextEdit) error {
	path := filepath.Clean(uriToPath(string(doc.URI)))
	targets := make(map[symbolDeclaration]foreignTarget)
	if doc.Analysis == nil || doc.Analysis.SemanticGraph == nil {
		return fmt.Errorf("semantic references are unavailable for %s", path)
	}
	for _, part := range parts {
		if part.target.name == "" {
			continue
		}
		for _, occurrence := range doc.Analysis.SemanticGraph.OccurrencesInFile(path) {
			if occurrence.Declaration && occurrence.Start >= part.span.start && occurrence.End <= part.span.end && doc.Content[occurrence.Start:occurrence.End] == part.target.name {
				targets[symbolDeclaration{path: path, offset: occurrence.Start}] = part.target
				break
			}
		}
	}
	for _, part := range parts {
		if part.target.name != "" {
			found := false
			for _, target := range targets {
				found = found || target.span == part.span
			}
			if !found {
				return fmt.Errorf("cannot resolve declaration %s", part.target.name)
			}
		}
	}
	if len(targets) == 0 {
		return nil
	}
	openFiles := s.documents.Snapshot()
	readSource := func(candidate string) ([]byte, error) {
		if content, ok := openFiles[filepath.Clean(candidate)]; ok {
			return []byte(content), nil
		}
		return os.ReadFile(candidate)
	}
	root, err := projectSourceRoot(path, readSource)
	if err != nil {
		return err
	}
	paths, err := s.projectSourcePaths(ctx, root)
	if err != nil {
		return err
	}
	paths = append(paths, path)
	for candidate := range openFiles {
		if filepath.Ext(candidate) == ".gecko" && withinDirectory(root, candidate) {
			paths = append(paths, candidate)
		}
	}
	seen := make(map[string]bool)
	for _, candidate := range paths {
		candidate = filepath.Clean(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := readSource(candidate)
		if err != nil {
			return fmt.Errorf("reading %s: %w", candidate, err)
		}
		content := string(data)
		if !foreignTargetNamesPresent(content, targets) {
			continue
		}
		graph := doc.Analysis.SemanticGraph
		currentFile := file
		if candidate != path {
			context, analyzeErr := analysis.NewAnalysisContextWithReader(candidate, content, readSource)
			if analyzeErr != nil {
				return fmt.Errorf("analyzing %s: %w", candidate, analyzeErr)
			}
			graph = context.SemanticGraph
			currentFile = context.MainFile
		}
		if err := appendForeignUses(candidate, content, path, module, graph, currentFile, targets, changes); err != nil {
			return err
		}
	}
	return nil
}

func foreignTargetNamesPresent(content string, targets map[symbolDeclaration]foreignTarget) bool {
	for _, target := range targets {
		if hasIdentifier(content, target.name) {
			return true
		}
	}
	return false
}

func appendForeignUses(path, content, originPath, module string, graph *semantic.Program, file *tokens.File, targets map[symbolDeclaration]foreignTarget, changes map[protocol.DocumentURI][]protocol.TextEdit) error {
	uri := pathToURI(path)
	qualifier := ""
	if path != originPath {
		for _, imported := range file.Imports {
			if filepath.Clean(imported.Path) == originPath {
				qualifier = imported.Alias
				if qualifier == "" {
					qualifier = imported.Name
				}
				break
			}
		}
	}
	for _, occurrence := range graph.OccurrencesInFile(path) {
		if occurrence.Declaration {
			continue
		}
		key, ok := declarationFor(graph, occurrence.SymbolID)
		if !ok {
			continue
		}
		target, ok := targets[key]
		if !ok || occurrence.Start < 0 || occurrence.End > len(content) || content[occurrence.Start:occurrence.End] != target.name {
			continue
		}
		if foreignImportAt(file, occurrence.Start) != nil {
			continue
		}
		prefix := module + "."
		if path != originPath && !foreignQualifiedUse(content, occurrence.Start) {
			if qualifier == "" {
				return fmt.Errorf("cannot qualify %s in %s", target.name, path)
			}
			prefix = qualifier + "." + prefix
		}
		changes[uri] = append(changes[uri], protocol.TextEdit{
			Range:   protocol.Range{Start: sourcePosition(content, occurrence.Start), End: sourcePosition(content, occurrence.End)},
			NewText: prefix + target.name,
		})
	}
	if path != originPath {
		appendForeignImportEdits(path, content, file, qualifier, targets, changes)
	}
	return nil
}

func foreignQualifiedUse(content string, offset int) bool {
	for offset > 0 && (content[offset-1] == ' ' || content[offset-1] == '\t') {
		offset--
	}
	return offset > 0 && content[offset-1] == '.'
}

func foreignImportAt(file *tokens.File, offset int) *tokens.Import {
	for _, entry := range file.Entries {
		if entry.Import != nil && entry.Import.Pos.Offset <= offset && offset < entry.Import.EndPos.Offset {
			return entry.Import
		}
	}
	return nil
}

func appendForeignImportEdits(path, content string, file *tokens.File, qualifier string, targets map[symbolDeclaration]foreignTarget, changes map[protocol.DocumentURI][]protocol.TextEdit) {
	for _, entry := range file.Entries {
		imp := entry.Import
		if imp == nil || imp.ModuleName() != qualifier || len(imp.Objects) == 0 {
			continue
		}
		remaining := make([]string, 0, len(imp.Objects))
		for _, name := range imp.Objects {
			migrated := false
			for _, target := range targets {
				migrated = migrated || name == target.name
			}
			if !migrated {
				remaining = append(remaining, name)
			}
		}
		if len(remaining) == len(imp.Objects) {
			continue
		}
		text := "import " + imp.Package()
		if imp.Alias != "" {
			text += " as " + imp.Alias
		}
		if len(remaining) > 0 {
			text += " use {" + strings.Join(remaining, ", ") + "}"
		}
		changes[pathToURI(path)] = append(changes[pathToURI(path)], protocol.TextEdit{
			Range:   protocol.Range{Start: sourcePosition(content, imp.Pos.Offset), End: sourcePosition(content, imp.EndPos.Offset)},
			NewText: text,
		})
	}
}

func validateForeignEdits(changes map[protocol.DocumentURI][]protocol.TextEdit, openFiles map[string]string) error {
	for uri, edits := range changes {
		path := filepath.Clean(uriToPath(string(uri)))
		content, ok := openFiles[path]
		if !ok {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			content = string(data)
		}
		sort.Slice(edits, func(i, j int) bool {
			return sourceOffset(content, edits[i].Range.Start) > sourceOffset(content, edits[j].Range.Start)
		})
		previous := len(content)
		for _, edit := range edits {
			start := sourceOffset(content, edit.Range.Start)
			end := sourceOffset(content, edit.Range.End)
			if start < 0 || end > previous || start > end {
				return fmt.Errorf("overlapping migration edits in %s", path)
			}
			content = content[:start] + edit.NewText + content[end:]
			previous = start
		}
		if _, err := parser.Parser.ParseString(path, content); err != nil {
			return fmt.Errorf("parsing migrated %s: %w", path, err)
		}
	}
	return nil
}
