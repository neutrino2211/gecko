package frontend

import (
	"path/filepath"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/parser"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

type Result struct {
	path        string
	content     string
	file        *tokens.File
	program     *semantic.Program
	sources     map[string]*parser.Source
	scopes      []*errors.ErrorScope
	parseError  error
	state       *compileState
	inputs      map[string]sourceInput
	directories map[string]string
	project     *config.ProjectConfig
}

type View struct {
	Path       string
	Content    string
	File       *tokens.File
	Program    *semantic.Program
	Scopes     []*errors.ErrorScope
	ParseError error
	state      *compileState
}

func ReadSource(path string, cfg *config.CompileCfg) ([]byte, error) {
	return readSource(path, cfg)
}

func (s *compileState) newErrorScope(name, path, content string) *errors.ErrorScope {
	scope := &errors.ErrorScope{Name: name, SourceName: path, Source: &content}
	s.scopes = append(s.scopes, scope)
	return scope
}

func Analyze(path, content string, cfg *config.CompileCfg) *Result {
	return NewSession().Analyze(path, content, cfg)
}

func analyze(path, content string, cfg *config.CompileCfg, parse func(string, string) (*parser.Source, error)) *Result {
	inputs := map[string]sourceInput{normalizePath(path): {content: []byte(content)}}
	cfg, seal := snapshotConfig(cfg, inputs)
	defer func() {
		cfg.Ctx = nil
		seal()
	}()
	state := newCompileState()
	state.parse = parse
	source, err := state.parse(path, content)
	result := &Result{path: path, content: content, project: cloneProject(cfg.Project), file: source.File, sources: state.parsed, parseError: err, state: state, inputs: inputs}
	state.parsed[normalizePath(path)] = source
	defer func() { result.directories = snapshotDirectories(inputs) }()
	if source.File == nil {
		return result
	}
	source.File.Config = cfg
	state.fileCache[normalizePath(path)] = source.File
	scope := state.newErrorScope("frontend", path, content)
	resolveImports(source.File, filepath.Dir(path), cfg, scope, state)
	preloadDirectoryReferences(source.File, state)
	result.scopes = state.scopes
	if err == nil {
		result.program = semantic.Analyze(source.File)
	}
	return result
}

func (r *Result) ParseError() error { return r.parseError }

func (r *Result) LexicalTokens(path string) []parser.SourceToken {
	if r == nil {
		return nil
	}
	if source := r.sources[normalizePath(path)]; source != nil {
		return append([]parser.SourceToken(nil), source.Tokens...)
	}
	return nil
}

func (v *View) ResolveType(name string) (*tokens.File, bool) {
	return resolveTypeFromDirectoryImports(v.File, name, v.state)
}

func (v *View) ResolveMethod(name string) (*tokens.File, bool) {
	return resolveMethodFromDirectoryImports(v.File, name, v.state)
}

func (v *View) ResolveModuleType(module, name string) (*tokens.File, bool) {
	return resolveModuleTypeFromDirectoryImports(v.File, module, name, v.state)
}
