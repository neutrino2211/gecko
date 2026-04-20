package main

import (
	"sync"

	"go.lsp.dev/protocol"
)

type Document struct {
	URI     protocol.DocumentURI
	Content string
	Version int
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

func (ds *DocumentStore) Close(uri protocol.DocumentURI) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	delete(ds.docs, uri)
}
