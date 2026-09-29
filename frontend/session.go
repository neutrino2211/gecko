package frontend

import (
	"sync"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/tokens"
)

type cachedSource struct {
	content string
	source  *parser.Source
	err     error
}

type Session struct {
	mu               sync.Mutex
	sources          map[string]cachedSource
	results          map[string]*Result
	parses, analyses uint64
}

func NewSession() *Session {
	return &Session{sources: make(map[string]cachedSource), results: make(map[string]*Result)}
}

func (s *Session) Analyze(path, content string, cfg *config.CompileCfg) *Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := normalizePath(path)
	if result := s.results[key]; result.Matches(path, content, cfg) {
		return result
	}
	result := analyze(path, content, cfg, s.parse)
	s.analyses++
	s.results[key] = result
	return result
}

func (s *Session) Syntax(path, content string) (*parser.Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parse(path, content)
}

func (s *Session) parse(path, content string) (*parser.Source, error) {
	key := normalizePath(path)
	cached, ok := s.sources[key]
	if !ok || cached.content != content {
		source, err := parser.ParseSource(path, content)
		cached = cachedSource{content: content, source: source, err: err}
		s.sources[key] = cached
		s.parses++
	}
	return &parser.Source{File: tokens.NewSyntaxCloner().File(cached.source.File), Tokens: append([]parser.SourceToken(nil), cached.source.Tokens...)}, cached.err
}

func (s *Session) Counts() (parses, analyses uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parses, s.analyses
}
