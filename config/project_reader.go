package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ProjectConfigError struct {
	Path string
	Err  error
}

func (e *ProjectConfigError) Error() string {
	return fmt.Sprintf("gecko.toml %s: %v", e.Path, e.Err)
}

func (e *ProjectConfigError) Unwrap() error {
	return e.Err
}

func FindProjectConfigWithReader(startDir string, readSource func(string) ([]byte, error)) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving project directory: %w", err)
	}
	for {
		path := filepath.Join(dir, "gecko.toml")
		if _, err := readSource(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", &ProjectConfigError{Path: path, Err: fmt.Errorf("reading config: %w", err)}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("gecko.toml not found from %s: %w", startDir, os.ErrNotExist)
}

func LoadProjectConfigWithReader(startDir string, readSource func(string) ([]byte, error)) (*ProjectConfig, error) {
	path, err := FindProjectConfigWithReader(startDir, readSource)
	if err != nil {
		return nil, err
	}
	return LoadProjectConfigFromFileWithReader(path, readSource)
}

func LoadProjectConfigFromFileWithReader(path string, readSource func(string) ([]byte, error)) (*ProjectConfig, error) {
	data, err := readSource(path)
	if err != nil {
		return nil, &ProjectConfigError{Path: path, Err: fmt.Errorf("reading config: %w", err)}
	}
	var project ProjectConfig
	if err := toml.Unmarshal(data, &project); err != nil {
		return nil, &ProjectConfigError{Path: path, Err: fmt.Errorf("parsing config: %w", err)}
	}
	project.ConfigPath = path
	project.ProjectRoot = filepath.Dir(path)
	if project.Build.Backend == "" {
		project.Build.Backend = "c"
	}
	return &project, nil
}
