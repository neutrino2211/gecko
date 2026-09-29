package main

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/neutrino2211/gecko/analysis"
	"github.com/neutrino2211/gecko/frontend"
)

func recoverUnfinishedDeclaration(path, content string, readSource func(string) ([]byte, error), parseError error, session *frontend.Session) *analysis.AnalysisContext {
	var parseErr participle.Error
	if !errors.As(parseError, &parseErr) {
		return nil
	}
	position := parseErr.Position()
	if position.Filename != "" && filepath.Clean(position.Filename) != filepath.Clean(path) {
		return nil
	}
	starts := topLevelDeclarationStarts(content)
	for index := len(starts) - 2; index >= 0; index-- {
		start, end := starts[index], starts[index+1]
		if start >= position.Offset {
			continue
		}
		segment := content[start:end]
		if !hasUnclosedParenthesis(segment) && !hasUnclosedGenericHeader(segment) && closeOpenBraces(segment) == segment {
			continue
		}
		masked := []byte(content)
		for offset := start; offset < end; offset++ {
			if masked[offset] != '\n' && masked[offset] != '\r' {
				masked[offset] = ' '
			}
		}
		ctx, err := analysis.NewAnalysisContextWithSession(path, string(masked), readSource, session)
		if err == nil {
			return ctx
		}
		return nil
	}
	return nil
}

func topLevelDeclarationStarts(content string) []int {
	var starts []int
	for offset := 0; offset < len(content); {
		end := strings.IndexByte(content[offset:], '\n')
		if end < 0 {
			end = len(content)
		} else {
			end += offset
		}
		line := strings.TrimSuffix(content[offset:end], "\r")
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && isDeclarationStart(line) {
			starts = append(starts, offset)
		}
		offset = end + 1
	}
	return starts
}

func isDeclarationStart(line string) bool {
	for _, prefix := range []string{"func ", "class ", "trait ", "enum ", "impl ", "type ", "public ", "external ", "declare ", "import ", "package ", "@"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func hasUnclosedParenthesis(content string) bool {
	return hasUnclosedDelimiter(content, '(', ')', false)
}

func hasUnclosedGenericHeader(content string) bool {
	return hasUnclosedDelimiter(content, '<', '>', true)
}

func hasUnclosedDelimiter(content string, open, close byte, headerOnly bool) bool {
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
		if headerOnly && content[offset] == '{' {
			return depth > 0
		}
		if content[offset] == open {
			depth++
		} else if content[offset] == close && depth > 0 {
			depth--
		}
	}
	return depth > 0
}
