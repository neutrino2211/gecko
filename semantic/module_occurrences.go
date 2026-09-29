package semantic

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) indexImportModule(imp *tokens.Import) {
	if a.currentFile == nil || a.currentFile.Path == "" || imp == nil {
		return
	}
	name := imp.ModuleName()
	content := a.currentFile.Content
	start := imp.Pos.Offset
	if name == "" || start < 0 || start >= len(content) {
		return
	}
	end := len(content)
	if lineEnd := strings.IndexByte(content[start:], '\n'); lineEnd >= 0 {
		end = start + lineEnd
	}
	line := content[start:end]
	offset := -1
	if imp.Alias != "" {
		if index := strings.LastIndex(line, "as "+name); index >= 0 {
			offset = start + index + len("as ")
		}
	} else if path := imp.Package(); path != "" {
		if index := strings.Index(line, path); index >= 0 {
			offset = start + index + len(path) - len(name)
		}
	}
	if offset < 0 || offset+len(name) > len(content) || content[offset:offset+len(name)] != name {
		return
	}
	pos := imp.Pos
	pos.Offset = offset
	pos.Column += offset - start
	id := a.program.addSymbol(SymbolModule, name, a.fileKey(a.currentFile)+"::import::"+name, nil, pos)
	a.program.addOccurrence(Occurrence{
		SymbolID: id, FilePath: filepath.Clean(a.currentFile.Path),
		Start: offset, End: offset + len(name), Declaration: true,
	})
	if a.moduleSymbolIDs[a.currentFile] == nil {
		a.moduleSymbolIDs[a.currentFile] = make(map[string]int64)
	}
	a.moduleSymbolIDs[a.currentFile][name] = id
}

func (a *analyzer) recordModuleQualifier(module string, pos lexer.Position) {
	if module == "" || a.currentFile == nil {
		return
	}
	id := a.moduleSymbolIDs[a.currentFile][module]
	content := a.currentFile.Content
	start := pos.Offset
	end := start + len(module)
	if id == 0 || start < 0 || end >= len(content) || content[start:end] != module {
		return
	}
	for end < len(content) && (content[end] == ' ' || content[end] == '\t') {
		end++
	}
	if end >= len(content) || content[end] != '.' {
		return
	}
	a.program.addOccurrence(Occurrence{
		SymbolID: id, FilePath: filepath.Clean(a.currentFile.Path),
		Start: start, End: start + len(module),
	})
}
