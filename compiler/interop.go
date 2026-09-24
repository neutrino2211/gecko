// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md, spec/attributes.md

package compiler

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
	"github.com/neutrino2211/gecko/utils"
)

// LastNativeLibraries holds native libraries requested by cimport/foreign clauses
// in the most recent compilation unit (including import closure).
var LastNativeLibraries []string

// LastNativeObjects holds native object inputs requested by cimport/foreign clauses
// in the most recent compilation unit (including import closure).
var LastNativeObjects []string

func unquoteIfQuoted(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

type legacyDeprecationHits struct {
	declarePos   *lexer.Position
	cimportPos   *lexer.Position
	variardicPos *lexer.Position
}

func notePos(dst **lexer.Position, pos lexer.Position) {
	if *dst != nil {
		return
	}
	p := pos
	*dst = &p
}

func scanLegacyDeprecations(entries []*tokens.Entry, hits *legacyDeprecationHits) {
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.Declaration != nil {
			notePos(&hits.declarePos, entry.Declaration.Pos)
			if entry.Declaration.Method != nil && entry.Declaration.Method.Variardic {
				notePos(&hits.variardicPos, entry.Declaration.Method.Pos)
			}
		}
		if entry.CImport != nil {
			notePos(&hits.cimportPos, entry.CImport.Pos)
		}
		if entry.Method != nil && entry.Method.Variardic {
			notePos(&hits.variardicPos, entry.Method.Pos)
		}
		if entry.Class != nil {
			for _, field := range entry.Class.Fields {
				if field != nil && field.Method != nil && field.Method.Variardic {
					notePos(&hits.variardicPos, field.Method.Pos)
				}
			}
		}
	}
}

func emitLegacyInteropDeprecationWarnings(sourceFile *tokens.File) {
	visited := make(map[string]bool)
	var walk func(file *tokens.File)
	walk = func(file *tokens.File) {
		if file == nil {
			return
		}
		key := file.Path
		if key == "" {
			key = file.Name
		}
		if key == "" {
			key = fmt.Sprintf("%p", file)
		}
		if visited[key] {
			return
		}
		visited[key] = true

		hits := &legacyDeprecationHits{}
		scanLegacyDeprecations(file.Entries, hits)
		if hits.declarePos != nil || hits.cimportPos != nil || hits.variardicPos != nil {
			scope := errors.NewErrorScope("deprecation", file.Path, file.Content)
			if hits.declarePos != nil {
				scope.NewCompileTimeWarning(
					"Deprecated Syntax",
					"`declare external` is deprecated; use `foreign \"c\" <module> ... { ... }`",
					*hits.declarePos,
				)
			}
			if hits.cimportPos != nil {
				scope.NewCompileTimeWarning(
					"Deprecated Syntax",
					"`cimport` is deprecated; use `foreign \"c\" <module> withheader ...`",
					*hits.cimportPos,
				)
			}
			if hits.variardicPos != nil {
				scope.NewCompileTimeWarning(
					"Deprecated Syntax",
					"`variardic` is deprecated; use trailing `...` in function parameter lists",
					*hits.variardicPos,
				)
			}
		}

		for _, imported := range file.Imports {
			walk(imported)
		}
	}
	walk(sourceFile)
}

func collectNativeLinkMetadata(sourceFile *tokens.File) {
	var libs []string
	var objects []string

	visited := make(map[string]bool)
	var walk func(file *tokens.File)
	walk = func(file *tokens.File) {
		if file == nil {
			return
		}
		key := file.Path
		if key == "" {
			key = file.Name
		}
		if key == "" {
			key = fmt.Sprintf("%p", file)
		}
		if visited[key] {
			return
		}
		visited[key] = true

		for _, entry := range file.Entries {
			if entry == nil {
				continue
			}
			if entry.CImport != nil {
				for _, libRaw := range entry.CImport.GetWithLibraries() {
					lib := strings.TrimSpace(unquoteIfQuoted(libRaw))
					if lib != "" {
						libs = append(libs, lib)
					}
				}
				for _, objRaw := range entry.CImport.GetWithObjects() {
					obj := strings.TrimSpace(unquoteIfQuoted(objRaw))
					if obj == "" {
						continue
					}
					if !filepath.IsAbs(obj) && file.Path != "" {
						obj = filepath.Join(filepath.Dir(file.Path), obj)
					}
					objects = append(objects, obj)
				}
			}
			if entry.Foreign != nil {
				for _, libRaw := range entry.Foreign.GetWithLibraries() {
					lib := strings.TrimSpace(unquoteIfQuoted(libRaw))
					if lib != "" {
						libs = append(libs, lib)
					}
				}
				for _, objRaw := range entry.Foreign.GetWithObjects() {
					obj := strings.TrimSpace(unquoteIfQuoted(objRaw))
					if obj == "" {
						continue
					}
					if !filepath.IsAbs(obj) && file.Path != "" {
						obj = filepath.Join(filepath.Dir(file.Path), obj)
					}
					objects = append(objects, obj)
				}
			}
		}

		for _, imported := range file.Imports {
			walk(imported)
		}
	}
	walk(sourceFile)

	LastNativeLibraries = utils.DedupeStrings(libs)
	LastNativeObjects = utils.DedupeStrings(objects)
	// Backward-compatibility for existing command helpers.
	cbackend.LastCImportLibraries = LastNativeLibraries
	cbackend.LastCImportObjects = LastNativeObjects
}
