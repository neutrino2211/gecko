package main

import (
	"sort"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

func organizeImportAction(content, filePath string, file *tokens.File) *protocol.CodeAction {
	if file == nil {
		return nil
	}
	var numbers []int
	for _, entry := range file.Entries {
		if entry != nil && entry.Import != nil {
			numbers = append(numbers, entry.Import.Pos.Line-1)
		}
	}
	if len(numbers) < 2 {
		return nil
	}
	sort.Ints(numbers)
	lines := strings.SplitAfter(content, "\n")
	for index, number := range numbers {
		if number < 0 || number >= len(lines) || index > 0 && number != numbers[index-1]+1 {
			return nil
		}
		line := strings.TrimSpace(lines[number])
		if !strings.HasPrefix(line, "import ") || strings.Contains(line, "//") || strings.Count(line, "{") != strings.Count(line, "}") {
			return nil
		}
	}
	ordered := make([]string, len(numbers))
	for index, number := range numbers {
		ordered[index] = lines[number]
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return strings.TrimSpace(ordered[i]) < strings.TrimSpace(ordered[j])
	})
	changed := false
	for index, number := range numbers {
		if ordered[index] != lines[number] {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	start := 0
	for _, line := range lines[:numbers[0]] {
		start += len(line)
	}
	end := start
	for _, line := range lines[numbers[0] : numbers[len(numbers)-1]+1] {
		end += len(line)
	}
	return &protocol.CodeAction{
		Title: "Sort contiguous imports",
		Kind:  protocol.SourceOrganizeImports,
		Edit: &protocol.WorkspaceEdit{Changes: map[protocol.DocumentURI][]protocol.TextEdit{
			pathToURI(filePath): {{
				Range:   protocol.Range{Start: sourcePosition(content, start), End: sourcePosition(content, end)},
				NewText: strings.Join(ordered, ""),
			}},
		}},
	}
}
