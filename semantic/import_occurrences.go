package semantic

import (
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/tokens"
)

func (a *analyzer) recordImportObjectUses(imp *tokens.Import) {
	if imp == nil || len(imp.Objects) == 0 || a.currentFile == nil {
		return
	}
	content := a.currentFile.Content
	start := imp.Pos.Offset
	if start < 0 || start >= len(content) {
		return
	}
	end := len(content)
	if lineEnd := strings.IndexByte(content[start:], '\n'); lineEnd >= 0 {
		end = start + lineEnd
	}
	open := strings.IndexByte(content[start:end], '{')
	if open < 0 {
		return
	}
	start += open + 1
	if close := strings.IndexByte(content[start:end], '}'); close >= 0 {
		end = start + close
	}
	module := imp.ModuleName()
	for _, name := range imp.Objects {
		offset := importObjectOffset(content, start, end, name)
		if offset < 0 {
			continue
		}
		if id := a.importObjectSymbolID(module, name); id != 0 {
			a.program.addOccurrence(Occurrence{
				SymbolID: id, FilePath: filepath.Clean(a.currentFile.Path),
				Start: offset, End: offset + len(name),
			})
		}
		start = offset + len(name)
	}
}

func importObjectOffset(content string, start, end int, name string) int {
	for start < end {
		index := strings.Index(content[start:end], name)
		if index < 0 {
			return -1
		}
		offset := start + index
		if (offset == 0 || !identifierByte(content[offset-1])) && (offset+len(name) == len(content) || !identifierByte(content[offset+len(name)])) {
			return offset
		}
		start = offset + len(name)
	}
	return -1
}

func (a *analyzer) importObjectSymbolID(module, name string) int64 {
	return a.importObjectSymbolIDFrom(a.currentFile, module, name)
}

func (a *analyzer) importObjectSymbolIDFrom(file *tokens.File, module, name string) int64 {
	if file == nil {
		return 0
	}
	var found int64
	for _, imported := range file.Imports {
		if imported == nil || imported.Name != module {
			continue
		}
		for id, symbol := range a.program.Symbols {
			if symbol.FullName == module+"::"+name && filepath.Clean(symbol.Pos.Filename) == filepath.Clean(imported.Path) {
				if found != 0 && found != id {
					return 0
				}
				found = id
			}
		}
	}
	return found
}

func (a *analyzer) explicitlyImportedSymbolID(file *tokens.File, name string, kind SymbolKind) int64 {
	if file == nil {
		return 0
	}
	var found int64
	for _, entry := range file.Entries {
		if entry == nil || entry.Import == nil {
			continue
		}
		for _, importedName := range entry.Import.Objects {
			if importedName != name {
				continue
			}
			id := a.importObjectSymbolIDFrom(file, entry.Import.ModuleName(), name)
			if symbol := a.program.SymbolByID(id); symbol != nil && symbol.Kind == kind {
				if found != 0 && found != id {
					return 0
				}
				found = id
			}
		}
	}
	return found
}
