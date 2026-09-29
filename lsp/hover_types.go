package main

import (
	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/tokens"
)

func findHoverInFile(file *tokens.File, name string) *HoverInfo {
	if file == nil {
		return nil
	}
	for _, entry := range file.Entries {
		if info := findTypeHoverInEntry(entry, name); info != nil {
			return info
		}
	}
	for _, entry := range file.Entries {
		if info := findInEntry(entry, name, nil); info != nil {
			return info
		}
	}
	return nil
}

func findTypeHoverInEntry(entry *tokens.Entry, name string) *HoverInfo {
	if entry == nil {
		return nil
	}
	if class := entry.Class; class != nil && class.Name == name {
		return &HoverInfo{
			Name: name, Type: analysis.FormatClassType(class),
			DocComment: tokens.DocCommentText(class.DocComment),
		}
	}
	if trait := entry.Trait; trait != nil && trait.Name == name {
		return &HoverInfo{
			Name: name, Type: analysis.FormatTraitType(trait),
			DocComment: tokens.DocCommentText(trait.DocComment),
		}
	}
	if enum := entry.Enum; enum != nil && enum.Name == name {
		return &HoverInfo{Name: name, Type: "enum " + name}
	}
	if declaration := entry.Declaration; declaration != nil {
		if external := declaration.ExternalType; external != nil && external.Name == name {
			return &HoverInfo{Name: name, Type: "declare external type " + name}
		}
	}
	if foreign := entry.Foreign; foreign != nil {
		for _, member := range foreign.Members {
			if member.Type != nil && member.Type.Name == name {
				return &HoverInfo{Name: name, Type: "type " + name + " opaque"}
			}
		}
	}
	return nil
}
