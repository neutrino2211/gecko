// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
	"go.lsp.dev/protocol"
)

// GetDefinitionLocation returns the location of a symbol's definition
func GetDefinitionLocation(content string, line, col int, uri string) *protocol.Location {
	file, err := parser.Parser.ParseString("", content)
	if err != nil {
		return nil
	}
	file.ComputeRanges()

	word := getWordAt(content, line, col)
	if word == "" {
		return nil
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

func findLocalDefinition(entry *tokens.Entry, name string, line, _ int, uri string) *protocol.Location {
	// Check if we're in a method
	if entry.Method != nil {
		method := entry.Method
		if method.Pos.Line <= line && line <= method.EndPos.Line {
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

			// Search in method body
			if loc := findDefinitionInEntries(method.Value, name, uri); loc != nil {
				return loc
			}
		}
	}

	// Check in class methods
	if entry.Class != nil {
		for _, field := range entry.Class.Fields {
			if field.Method != nil && field.Method.Pos.Line <= line && line <= field.Method.EndPos.Line {
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
				if loc := findDefinitionInEntries(field.Method.Value, name, uri); loc != nil {
					return loc
				}
			}
		}
	}

	return nil
}

func findDefinitionInEntries(entries []*tokens.Entry, name string, uri string) *protocol.Location {
	for _, entry := range entries {
		if entry.Field != nil && entry.Field.Name == name {
			return &protocol.Location{
				URI: protocol.DocumentURI(uri),
				Range: protocol.Range{
					Start: protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1)},
					End:   protocol.Position{Line: uint32(entry.Field.Pos.Line - 1), Character: uint32(entry.Field.Pos.Column - 1 + len(entry.Field.Name))},
				},
			}
		}

		// Recurse into if/else/loop blocks
		if entry.If != nil {
			if loc := findDefinitionInEntries(entry.If.Value, name, uri); loc != nil {
				return loc
			}
			elseIf := entry.If.ElseIf
			for elseIf != nil {
				if loc := findDefinitionInEntries(elseIf.Value, name, uri); loc != nil {
					return loc
				}
				elseIf = elseIf.ElseIf
			}
			if entry.If.Else != nil {
				if loc := findDefinitionInEntries(entry.If.Else.Value, name, uri); loc != nil {
					return loc
				}
			}
		}

		if entry.Loop != nil {
			if loc := findDefinitionInEntries(entry.Loop.Value, name, uri); loc != nil {
				return loc
			}
		}
	}
	return nil
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
