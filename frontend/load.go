package frontend

import (
	"fmt"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/tokens"
)

func (s *compileState) loadFile(path string, cfg *config.CompileCfg) (*tokens.File, error) {
	key := normalizePath(path)
	if file := s.fileCache[key]; file != nil {
		return file, nil
	}
	content, err := readSource(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	source, err := s.parse(path, string(content))
	s.parsed[key] = source
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	source.File.Config = cfg
	s.fileCache[key] = source.File
	return source.File, nil
}
