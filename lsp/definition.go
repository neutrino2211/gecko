// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"strings"

	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetDefinitionLocation returns the location of a symbol's definition. When ctx
// is non-nil it provides the single shared semantic graph used by the compiler.
func GetDefinitionLocation(ctx *analysis.AnalysisContext, content string, line, col int, uri string) *protocol.Location {
	file := ctxFile(ctx, content)
	if file == nil {
		return nil
	}

	word := getWordAt(content, line, col)
	if word == "" {
		return nil
	}
	if ctx != nil {
		position := utf16Position(content, protocol.Position{Line: uint32(line), Character: uint32(col)})
		if location := semanticDefinition(ctx, ctx.FilePath, content, position); location != nil {
			return location
		}
	}
	if ctx != nil {
		if imported := importedSymbolFile(ctx, file, content, line, col, word); imported != nil {
			if location := findDefinitionInFile(imported, word); location != nil {
				return location
			}
		}
	}
	if receiver := memberQualifier(content, line, col); receiver != "" {
		return findMemberDefinition(ctx, file, receiver, word, line+1, uri)
	}

	// First check for local variables within method bodies
	for _, entry := range file.Entries {
		if loc := findLocalDefinition(entry, word, line+1, col+1, uri); loc != nil {
			return loc
		}
	}

	// Search for top-level definitions
	for _, entry := range file.Entries {
		if loc := findDefinitionInEntry(entry, word, uri); loc != nil {
			return loc
		}
	}

	return nil
}

func findMemberDefinition(ctx *analysis.AnalysisContext, file *tokens.File, receiver, name string, line int, uri string) *protocol.Location {
	typeName := receiverVariableType(ctx, file, receiver, line)
	if typeName == "" {
		return nil
	}
	base := parseGenericType(typeName).BaseName
	if parts := strings.SplitN(base, ".", 2); len(parts) == 2 && ctx != nil {
		for _, imported := range ctx.FilesForModule(parts[0]) {
			if location := findTypeMemberInFile(imported, parts[1], name, string(pathToURI(imported.Path))); location != nil {
				return location
			}
		}
	}
	return findTypeMemberInFile(file, base, name, uri)
}

func findTypeMemberInFile(file *tokens.File, typeName, name, uri string) *protocol.Location {
	for _, entry := range file.Entries {
		if entry.Class != nil && entry.Class.Name == typeName {
			if location := findDefinitionInEntry(entry, name, uri); location != nil {
				return location
			}
		}
		if entry.Implementation != nil && isImplForClass(entry.Implementation, typeName) {
			if location := findDefinitionInEntry(entry, name, uri); location != nil {
				return location
			}
		}
	}
	for _, entry := range file.Entries {
		if entry.Implementation != nil && entry.Implementation.GetFor() == typeName {
			if location := findTraitMember(file, entry.Implementation.GetName(), name, uri, make(map[string]bool)); location != nil {
				return location
			}
		}
	}
	return nil
}

func findTraitMember(file *tokens.File, traitName, name, uri string, visited map[string]bool) *protocol.Location {
	if visited[traitName] {
		return nil
	}
	visited[traitName] = true
	for _, entry := range file.Entries {
		if entry.Trait == nil || entry.Trait.Name != traitName {
			continue
		}
		if location := findDefinitionInEntry(entry, name, uri); location != nil {
			return location
		}
		for _, parent := range entry.Trait.AllParents() {
			if location := findTraitMember(file, parent, name, uri, visited); location != nil {
				return location
			}
		}
	}
	return nil
}

func findDefinitionInFile(file *tokens.File, name string) *protocol.Location {
	uri := string(pathToURI(file.Path))
	for _, entry := range file.Entries {
		if location := findDefinitionInEntry(entry, name, uri); location != nil {
			return location
		}
	}
	return nil
}

func findLocalDefinition(entry *tokens.Entry, name string, line, _ int, uri string) *protocol.Location {
	// Check if we're in a method
	if entry.Method != nil {
		method := entry.Method
		if method.Pos.Line <= line && line <= method.EndPos.Line {
			if loc := findDefinitionInEntries(method.Value, name, line, uri); loc != nil {
				return loc
			}
			// Check function arguments
			for _, arg := range method.Arguments {
				if arg.Name == name {
					return &protocol.Location{
						URI: protocol.DocumentURI(uri),
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1)},
							End:   protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1 + len(arg.Name))},
						},
					}
				}
			}

		}
	}

	// Check in class methods
	if entry.Class != nil {
		for _, field := range entry.Class.Fields {
			if field.Method != nil && field.Method.Pos.Line <= line && line <= field.Method.EndPos.Line {
				if loc := findDefinitionInEntries(field.Method.Value, name, line, uri); loc != nil {
					return loc
				}
				for _, arg := range field.Method.Arguments {
					if arg.Name == name {
						return &protocol.Location{
							URI: protocol.DocumentURI(uri),
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1 + len(arg.Name))},
							},
						}
					}
				}
			}
		}
	}

	return nil
}

func findDefinitionInEntries(entries []*tokens.Entry, name string, line int, uri string) *protocol.Location {
	var latest *protocol.Location
	for _, entry := range entries {
		if entry.Field != nil && entry.Field.Name == name && entry.Field.Pos.Line <= line {
			latest = &protocol.Location{
				URI: protocol.DocumentURI(uri),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1)},
					End:   protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1 + len(entry.Field.Name))},
				},
			}
		}

		// Recurse into if/else/loop blocks
		if entry.If != nil && entry.If.Pos.Line <= line && line <= entry.If.EndPos.Line {
			if entry.If.Else != nil && entry.If.Else.Pos.Line <= line {
				if loc := findDefinitionInEntries(entry.If.Else.Value, name, line, uri); loc != nil {
					return loc
				}
				continue
			}
			elseIf := entry.If.ElseIf
			for elseIf != nil {
				if elseIf.Pos.Line <= line && line <= elseIf.EndPos.Line {
					if loc := findDefinitionInEntries(elseIf.Value, name, line, uri); loc != nil {
						return loc
					}
					break
				}
				elseIf = elseIf.ElseIf
			}
			if elseIf == nil {
				if loc := findDefinitionInEntries(entry.If.Value, name, line, uri); loc != nil {
					return loc
				}
			}
		}

		if entry.Loop != nil && entry.Loop.Pos.Line <= line && line <= entry.Loop.EndPos.Line {
			if loc := findDefinitionInEntries(entry.Loop.Value, name, line, uri); loc != nil {
				return loc
			}
		}
	}
	return latest
}

func findDefinitionInEntry(entry *tokens.Entry, name string, uri string) *protocol.Location {
	if entry.Class != nil && entry.Class.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Class.Pos.Line - 1), Character: uint32(entry.Class.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Class.Pos.Line - 1), Character: uint32(entry.Class.Pos.Column - 1 + len(entry.Class.Name))},
			},
		}
	}

	// Check class fields and methods
	if entry.Class != nil {
		for _, field := range entry.Class.Fields {
			if field.Method != nil && field.Method.Name == name {
				return &protocol.Location{
					URI: protocol.DocumentURI(uri),
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(field.Method.Pos.Line - 1), Character: uint32(field.Method.Pos.Column - 1)},
						End:   protocol.Position{Line: uint32(field.Method.Pos.Line - 1), Character: uint32(field.Method.Pos.Column - 1 + len(field.Method.Name))},
					},
				}
			}
			if field.Field != nil && field.Field.Name == name {
				return &protocol.Location{
					URI: protocol.DocumentURI(uri),
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(field.Field.Pos.Line - 1), Character: uint32(field.Field.Pos.Column - 1)},
						End:   protocol.Position{Line: uint32(field.Field.Pos.Line - 1), Character: uint32(field.Field.Pos.Column - 1 + len(field.Field.Name))},
					},
				}
			}
		}
	}

	if entry.Trait != nil && entry.Trait.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Trait.Pos.Line - 1), Character: uint32(entry.Trait.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Trait.Pos.Line - 1), Character: uint32(entry.Trait.Pos.Column - 1 + len(entry.Trait.Name))},
			},
		}
	}

	if entry.Trait != nil {
		for _, field := range entry.Trait.Fields {
			if field.Name == name {
				return &protocol.Location{
					URI: protocol.DocumentURI(uri),
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(field.Pos.Line - 1), Character: uint32(field.Pos.Column - 1)},
						End:   protocol.Position{Line: uint32(field.Pos.Line - 1), Character: uint32(field.Pos.Column - 1 + len(field.Name))},
					},
				}
			}
		}
	}

	if entry.Method != nil && entry.Method.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Method.Pos.Line - 1), Character: uint32(entry.Method.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Method.Pos.Line - 1), Character: uint32(entry.Method.Pos.Column - 1 + len(entry.Method.Name))},
			},
		}
	}

	if entry.Field != nil && entry.Field.Name == name {
		return &protocol.Location{
			URI: protocol.DocumentURI(uri),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1)},
				End:   protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1 + len(entry.Field.Name))},
			},
		}
	}

	if entry.Declaration != nil {
		if entry.Declaration.Method != nil && entry.Declaration.Method.Name == name {
			return &protocol.Location{
				URI: protocol.DocumentURI(uri),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(entry.Declaration.Method.Pos.Line - 1), Character: uint32(entry.Declaration.Method.Pos.Column - 1)},
					End:   protocol.Position{Line: uint32(entry.Declaration.Method.Pos.Line - 1), Character: uint32(entry.Declaration.Method.Pos.Column - 1 + len(entry.Declaration.Method.Name))},
				},
			}
		}
	}

	if entry.Implementation != nil {
		for _, field := range entry.Implementation.GetFields() {
			if field.Name == name {
				return &protocol.Location{
					URI: protocol.DocumentURI(uri),
					Range: protocol.Range{
						Start: protocol.Position{Line: uint32(field.Pos.Line - 1), Character: uint32(field.Pos.Column - 1)},
						End:   protocol.Position{Line: uint32(field.Pos.Line - 1), Character: uint32(field.Pos.Column - 1 + len(field.Name))},
					},
				}
			}
		}
	}

	return nil
}
