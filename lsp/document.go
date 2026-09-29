// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/participle/v2"
	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/frontend"
	"go.lsp.dev/protocol"
)

type Document struct {
	URI     protocol.DocumentURI
	Content string
	Version int

	// Analysis holds the shared semantic graph for this document. It is the
	// single source of truth for types, reused by every language feature so the
	// editor and the compiler stay on the same page.
	Analysis *analysis.AnalysisContext
}

type DocumentStore struct {
	frontendSession    *frontend.Session
	mu                 sync.RWMutex
	docs               map[protocol.DocumentURI]*Document
	dependencies       map[protocol.DocumentURI]*dependencyClosure
	dependencyRevision uint64
}

func NewDocumentStore() *DocumentStore {
	return &DocumentStore{
		frontendSession: frontend.NewSession(),
		docs:            make(map[protocol.DocumentURI]*Document),
		dependencies:    make(map[protocol.DocumentURI]*dependencyClosure),
	}
}

func (ds *DocumentStore) Open(uri protocol.DocumentURI, content string, version int32) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.docs[uri] = &Document{
		URI:     uri,
		Content: content,
		Version: int(version),
	}
	ds.dependencies = make(map[protocol.DocumentURI]*dependencyClosure)
	ds.dependencyRevision++
}

func (ds *DocumentStore) Update(uri protocol.DocumentURI, content string, version int32) bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if doc, ok := ds.docs[uri]; ok {
		if int(version) <= doc.Version {
			return false
		}
		ds.docs[uri] = &Document{URI: uri, Content: content, Version: int(version)}
	} else {
		ds.docs[uri] = &Document{
			URI:     uri,
			Content: content,
			Version: int(version),
		}
		ds.dependencies = make(map[protocol.DocumentURI]*dependencyClosure)
	}
	delete(ds.dependencies, uri)
	ds.dependencyRevision++
	return true
}

func (ds *DocumentStore) Get(uri protocol.DocumentURI) (*Document, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	doc, ok := ds.docs[uri]
	if !ok {
		return nil, false
	}
	copy := *doc
	return &copy, true
}

func (d *Document) RebuildAnalysis(openFiles map[string]string) {
	d.RebuildAnalysisWithSession(openFiles, frontend.NewSession())
}

func (d *Document) RebuildAnalysisWithSession(openFiles map[string]string, session *frontend.Session) {
	defer func() {
		if d.Analysis != nil {
			d.Analysis.SourceContent = d.Content
		}
	}()
	path := uriToPath(string(d.URI))
	readSource := func(path string) ([]byte, error) {
		if content, ok := openFiles[filepath.Clean(path)]; ok {
			return []byte(content), nil
		}
		return os.ReadFile(path)
	}
	ctx, err := analysis.NewAnalysisContextWithSession(path, d.Content, readSource, session)
	if err == nil {
		d.Analysis = ctx
		return
	}
	if recovered := recoverUnfinishedDeclaration(path, d.Content, readSource, err, session); recovered != nil {
		d.Analysis = recovered
		return
	}
	if recovered := recoverEditedAnalysis(path, d.Content, readSource, err, session); recovered != nil {
		d.Analysis = recovered
		return
	}
	prefix := d.Content
	var parseErr participle.Error
	if errors.As(err, &parseErr) {
		if offset := parseErr.Position().Offset; offset > 0 && offset <= len(prefix) {
			prefix = prefix[:offset]
		}
	}
	for {
		end := strings.LastIndexByte(prefix, '\n')
		if end < 0 {
			break
		}
		prefix = prefix[:end]
		if strings.TrimSpace(prefix) == "" {
			break
		}
		ctx, err = analysis.NewAnalysisContextWithSession(path, prefix, readSource, session)
		if err == nil {
			d.Analysis = ctx
			return
		}
	}
	d.Analysis = nil
}

func recoverEditedAnalysis(path, content string, readSource func(string) ([]byte, error), parseError error, session *frontend.Session) *analysis.AnalysisContext {
	masked := []byte(content)
	for attempts := 0; attempts <= strings.Count(content, "\n"); attempts++ {
		var parseErr participle.Error
		if !errors.As(parseError, &parseErr) {
			return nil
		}
		position := parseErr.Position()
		if position.Filename != "" && filepath.Clean(position.Filename) != filepath.Clean(path) {
			return nil
		}
		if position.Offset >= len(masked) {
			if closed := closeOpenBraces(string(masked)); closed != string(masked) {
				ctx, err := analysis.NewAnalysisContextWithSession(path, closed, readSource, session)
				if err == nil {
					return ctx
				}
			}
		}
		if !maskErrorLine(masked, position.Offset) {
			return nil
		}
		ctx, err := analysis.NewAnalysisContextWithSession(path, string(masked), readSource, session)
		if err == nil {
			return ctx
		}
		parseError = err
	}
	return nil
}

func maskErrorLine(content []byte, offset int) bool {
	if offset < 0 || offset > len(content) {
		return false
	}
	if offset == len(content) && offset > 0 {
		offset--
	}
	start := offset
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	end := offset
	for end < len(content) && content[end] != '\n' {
		end++
	}
	if strings.TrimSpace(string(content[start:end])) == "" {
		if start == 0 {
			return false
		}
		return maskErrorLine(content, start-1)
	}
	if strings.HasPrefix(strings.TrimSpace(string(content[start:end])), "package ") {
		return false
	}
	for index := start; index < end; index++ {
		if content[index] != '\r' {
			content[index] = ' '
		}
	}
	return true
}

func closeOpenBraces(content string) string {
	depth := 0
	for offset := 0; offset < len(content); offset++ {
		if content[offset] == '/' && offset+1 < len(content) && content[offset+1] == '/' {
			for offset < len(content) && content[offset] != '\n' {
				offset++
			}
			continue
		}
		if content[offset] == '"' || content[offset] == '`' {
			quote := content[offset]
			offset++
			for offset < len(content) && content[offset] != quote {
				if content[offset] == '\\' && quote == '"' {
					offset++
				}
				offset++
			}
			continue
		}
		if content[offset] == '{' {
			depth++
		} else if content[offset] == '}' && depth > 0 {
			depth--
		}
	}
	if depth == 0 {
		return content
	}
	return content + strings.Repeat("}", depth)
}

func (ds *DocumentStore) SetAnalysis(doc *Document) bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	current, ok := ds.docs[doc.URI]
	if !ok || current.Version != doc.Version || current.Content != doc.Content {
		return false
	}
	current.Analysis = doc.Analysis
	return true
}

func (ds *DocumentStore) Save(uri protocol.DocumentURI, content string) bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	current, ok := ds.docs[uri]
	if !ok || current.Content == content {
		return false
	}
	ds.docs[uri] = &Document{URI: uri, Content: content, Version: current.Version}
	delete(ds.dependencies, uri)
	ds.dependencyRevision++
	return true
}

func (ds *DocumentStore) Snapshot() map[string]string {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	contents := make(map[string]string, len(ds.docs))
	for uri, doc := range ds.docs {
		contents[filepath.Clean(uriToPath(string(uri)))] = doc.Content
	}
	return contents
}

func (ds *DocumentStore) Close(uri protocol.DocumentURI) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	delete(ds.docs, uri)
	delete(ds.dependencies, uri)
	ds.dependencyRevision++
}
