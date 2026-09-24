// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	"sync"

	"github.com/neutrino2211/gecko/analysis"
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
	mu   sync.RWMutex
	docs map[protocol.DocumentURI]*Document
}

func NewDocumentStore() *DocumentStore {
	return &DocumentStore{
		docs: make(map[protocol.DocumentURI]*Document),
	}
}

func (ds *DocumentStore) Open(uri protocol.DocumentURI, content string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.docs[uri] = &Document{
		URI:     uri,
		Content: content,
		Version: 1,
	}
}

func (ds *DocumentStore) Update(uri protocol.DocumentURI, content string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if doc, ok := ds.docs[uri]; ok {
		doc.Content = content
		doc.Version++
	} else {
		ds.docs[uri] = &Document{
			URI:     uri,
			Content: content,
			Version: 1,
		}
	}
}

func (ds *DocumentStore) Get(uri protocol.DocumentURI) (*Document, bool) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	doc, ok := ds.docs[uri]
	return doc, ok
}

// RebuildAnalysis (re)computes the shared semantic graph for this document from
// its current content. On a parse/analysis failure (e.g. transiently broken
// source while typing) the previous graph is retained so editor features keep
// working against the last good analysis.
func (d *Document) RebuildAnalysis() {
	ctx, err := analysis.NewAnalysisContext(uriToPath(string(d.URI)), d.Content)
	if err != nil {
		return
	}
	d.Analysis = ctx
}

func (ds *DocumentStore) Close(uri protocol.DocumentURI) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	delete(ds.docs, uri)
}
