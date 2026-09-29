package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type sourceDirectoryState struct {
	realPath string
	modified time.Time
}

type sourceCacheEntry struct {
	paths       []string
	directories map[string]sourceDirectoryState
}

func (entry *sourceCacheEntry) valid(ctx context.Context) bool {
	for directory, state := range entry.directories {
		if ctx.Err() != nil {
			return false
		}
		realPath, err := filepath.EvalSymlinks(directory)
		if err != nil || realPath != state.realPath {
			return false
		}
		info, err := os.Stat(directory)
		if err != nil || !info.ModTime().Equal(state.modified) {
			return false
		}
	}
	return true
}

func (s *Server) invalidateSourcePaths() {
	s.mu.Lock()
	s.sourcePaths = make(map[string]*sourceCacheEntry)
	s.sourceRevision++
	s.mu.Unlock()
}

func (s *Server) projectSourcePaths(ctx context.Context, root string) ([]string, error) {
	root = filepath.Clean(root)
	s.mu.Lock()
	cached := s.sourcePaths[root]
	revision := s.sourceRevision
	s.mu.Unlock()
	if cached != nil && cached.valid(ctx) {
		return cached.paths, nil
	}
	entry, err := scanSourcePaths(ctx, root)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if revision == s.sourceRevision {
		s.sourcePaths[root] = entry
	}
	s.mu.Unlock()
	return entry.paths, nil
}

func discoverSourcePaths(ctx context.Context, root string) ([]string, error) {
	entry, err := scanSourcePaths(ctx, root)
	if err != nil {
		return nil, err
	}
	return entry.paths, nil
}

func scanSourcePaths(ctx context.Context, root string) (*sourceCacheEntry, error) {
	paths := make(map[string]bool)
	result := &sourceCacheEntry{directories: make(map[string]sourceDirectoryState)}
	ancestors := make(map[string]bool)
	var walk func(string) error
	walk = func(directory string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		realPath, err := filepath.EvalSymlinks(directory)
		if err != nil {
			return fmt.Errorf("resolving source directory %s: %w", directory, err)
		}
		if ancestors[realPath] {
			return nil
		}
		info, err := os.Stat(directory)
		if err != nil {
			return fmt.Errorf("checking source directory %s: %w", directory, err)
		}
		result.directories[directory] = sourceDirectoryState{realPath: realPath, modified: info.ModTime()}
		ancestors[realPath] = true
		defer delete(ancestors, realPath)
		entries, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("reading source directory %s: %w", directory, err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			path := filepath.Join(directory, entry.Name())
			if entry.IsDir() {
				if err := walk(path); err != nil {
					return err
				}
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				info, err := os.Stat(path)
				if err != nil {
					continue
				}
				if info.IsDir() {
					if err := walk(path); err != nil {
						return err
					}
					continue
				}
			}
			if filepath.Ext(path) == ".gecko" {
				paths[filepath.Clean(path)] = true
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	result.paths = make([]string, 0, len(paths))
	for path := range paths {
		result.paths = append(result.paths, path)
	}
	sort.Strings(result.paths)
	return result, nil
}
