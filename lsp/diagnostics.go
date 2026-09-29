// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/scoping.md

package main

import (
	stderrors "errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/frontend"
	"github.com/urfave/cli/v2"
	"go.lsp.dev/protocol"
)

var compilerCheckMu sync.Mutex

func uriToPath(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return uri
	}
	return filepath.FromSlash(parsed.Path)
}

func pathToURI(path string) protocol.DocumentURI {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return protocol.DocumentURI((&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String())
}

func RunCompilerCheck(uri string, content string) ([]protocol.Diagnostic, error) {
	results, err := RunWorkspaceCheck(uri, content, nil)
	return results[protocol.DocumentURI(uri)], err
}

func RunWorkspaceCheck(uri string, content string, openFiles map[string]string) (map[protocol.DocumentURI][]protocol.Diagnostic, error) {
	return RunWorkspaceCheckWithTarget(uri, content, openFiles, "")
}

func RunWorkspaceCheckWithTarget(uri string, content string, openFiles map[string]string, targetOverride string) (map[protocol.DocumentURI][]protocol.Diagnostic, error) {
	return runWorkspaceCheckWithFrontend(uri, content, openFiles, targetOverride, nil)
}

func runWorkspaceCheckWithFrontend(uri string, content string, openFiles map[string]string, targetOverride string, prepared *frontend.Result, sessions ...*frontend.Session) (map[protocol.DocumentURI][]protocol.Diagnostic, error) {
	filePath := uriToPath(uri)
	contents := make(map[string]string, len(openFiles)+1)
	for path, text := range openFiles {
		contents[filepath.Clean(path)] = text
	}
	contents[filepath.Clean(filePath)] = content

	readSource := func(path string) ([]byte, error) {
		if text, ok := contents[filepath.Clean(path)]; ok {
			return []byte(text), nil
		}
		return os.ReadFile(path)
	}

	if filepath.Base(filePath) == "gecko.toml" {
		project, err := config.LoadProjectConfigFromFileWithReader(filePath, readSource)
		if err != nil {
			return configCheckResult(err, contents, readSource), nil
		}
		if _, err := config.ResolveCompileTarget(project, targetOverride, runtime.GOARCH, runtime.GOOS); err != nil {
			return configCheckResult(&config.ProjectConfigError{Path: filePath, Err: err}, contents, readSource), nil
		}
		return map[protocol.DocumentURI][]protocol.Diagnostic{}, nil
	}

	project, err := config.LoadProjectConfigWithReader(filepath.Dir(filePath), readSource)
	if err != nil && !stderrors.Is(err, os.ErrNotExist) {
		return configCheckResult(err, contents, readSource), nil
	}
	target, err := config.ResolveCompileTarget(project, targetOverride, runtime.GOARCH, runtime.GOOS)
	if err != nil {
		if project == nil {
			return nil, fmt.Errorf("selecting target: %w", err)
		}
		return configCheckResult(&config.ProjectConfigError{Path: project.ConfigPath, Err: err}, contents, readSource), nil
	}
	backend := "c"
	if project != nil {
		backend = project.Build.Backend
	}
	flags := flag.NewFlagSet("lsp", flag.ContinueOnError)
	flags.String("backend", backend, "")
	cfg := &config.CompileCfg{
		Arch:       target.Arch,
		Platform:   target.Platform,
		Vendor:     target.Vendor,
		TargetKey:  target.Key,
		CheckOnly:  true,
		Ctx:        cli.NewContext(&cli.App{}, flags, nil),
		Project:    project,
		ReadSource: readSource,
	}
	if !prepared.Matches(filePath, content, cfg) {
		if len(sessions) > 0 {
			prepared = sessions[0].Analyze(filePath, content, cfg)
		} else {
			prepared = frontend.Analyze(filePath, content, cfg)
		}
	}
	compilerCheckMu.Lock()
	compiler.ResetCompilationState()
	compilation := compiler.CompilePreparedWithDiagnostics(prepared, cfg)
	compilerCheckMu.Unlock()
	errors := compiler.GetAllErrors(compilation.Scopes)
	warnings := compiler.GetAllWarnings(compilation.Scopes)

	results := make(map[protocol.DocumentURI][]protocol.Diagnostic)
	add := func(message compiler.DiagnosticMessage, severity protocol.DiagnosticSeverity) {
		path := message.File
		if path == "" || !filepath.IsAbs(path) {
			path = filePath
		}
		path = filepath.Clean(path)
		uri := pathToURI(path)
		text, ok := contents[path]
		if !ok {
			if data, readErr := readSource(path); readErr == nil {
				text = string(data)
			}
		}
		diagnostic := protocol.Diagnostic{
			Range:    compilerDiagnosticRange(text, message),
			Severity: severity,
			Source:   "gecko",
			Message:  message.Message,
		}
		if message.Code != "" {
			diagnostic.Code = message.Code
		}
		for _, related := range message.Related {
			relatedPath := related.Span.File
			if relatedPath == "" || !filepath.IsAbs(relatedPath) {
				relatedPath = path
			}
			relatedPath = filepath.Clean(relatedPath)
			relatedText, ok := contents[relatedPath]
			if !ok {
				data, readErr := readSource(relatedPath)
				if readErr != nil {
					continue
				}
				relatedText = string(data)
			}
			if related.Span.Start < 0 || related.Span.End <= related.Span.Start || related.Span.End > len(relatedText) {
				continue
			}
			diagnostic.RelatedInformation = append(diagnostic.RelatedInformation, protocol.DiagnosticRelatedInformation{
				Location: protocol.Location{URI: pathToURI(relatedPath), Range: protocol.Range{
					Start: sourcePosition(relatedText, related.Span.Start),
					End:   sourcePosition(relatedText, related.Span.End),
				}},
				Message: related.Message,
			})
		}
		if len(message.Fixes) > 0 {
			diagnostic.Data = message.Fixes
		}
		results[uri] = append(results[uri], diagnostic)
	}
	for _, message := range errors {
		add(message, protocol.DiagnosticSeverityError)
	}
	for _, message := range warnings {
		add(message, protocol.DiagnosticSeverityWarning)
	}
	return results, nil
}

func configCheckResult(err error, contents map[string]string, readSource func(string) ([]byte, error)) map[protocol.DocumentURI][]protocol.Diagnostic {
	var configErr *config.ProjectConfigError
	if !stderrors.As(err, &configErr) {
		return map[protocol.DocumentURI][]protocol.Diagnostic{}
	}
	path := filepath.Clean(configErr.Path)
	content := contents[path]
	if content == "" {
		if data, readErr := readSource(path); readErr == nil {
			content = string(data)
		}
	}
	rng := diagnosticRange(content, 1, 1)
	var parseErr toml.ParseError
	if stderrors.As(err, &parseErr) {
		start := parseErr.Position.Start
		end := start + parseErr.Position.Len
		if end <= start {
			end = start + 1
		}
		rng = protocol.Range{Start: sourcePosition(content, start), End: sourcePosition(content, end)}
	}
	return map[protocol.DocumentURI][]protocol.Diagnostic{
		pathToURI(path): {{Range: rng, Severity: protocol.DiagnosticSeverityError, Source: "gecko.toml", Message: configErr.Error()}},
	}
}

func diagnosticRange(content string, line, column int) protocol.Range {
	lines := strings.Split(content, "\n")
	row := line - 1
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	lineText := strings.TrimSuffix(lines[row], "\r")
	start := column - 1
	if start < 0 {
		start = 0
	}
	if start > len(lineText) {
		start = len(lineText)
	}
	for start > 0 && start < len(lineText) && !utf8.RuneStart(lineText[start]) {
		start--
	}
	end := start
	if start < len(lineText) {
		_, size := utf8.DecodeRuneInString(lineText[start:])
		end += size
	}
	return protocol.Range{
		Start: protocol.Position{Line: uint32(row), Character: uint32(len(utf16.Encode([]rune(lineText[:start]))))},
		End:   protocol.Position{Line: uint32(row), Character: uint32(len(utf16.Encode([]rune(lineText[:end]))))},
	}
}

func compilerDiagnosticRange(content string, message compiler.DiagnosticMessage) protocol.Range {
	start := message.Offset
	if start < 0 || start >= len(content) || !utf8.RuneStart(content[start]) {
		return diagnosticRange(content, message.Line, message.Column)
	}
	before := content[:start]
	line := strings.Count(before, "\n") + 1
	column := start - strings.LastIndexByte(before, '\n')
	if line != message.Line || column != message.Column {
		return diagnosticRange(content, message.Line, message.Column)
	}
	if message.EndOffset > start && message.EndOffset <= len(content) && (message.EndOffset == len(content) || utf8.RuneStart(content[message.EndOffset])) {
		return protocol.Range{Start: sourcePosition(content, start), End: sourcePosition(content, message.EndOffset)}
	}
	end := start
	for end < len(content) {
		r, size := utf8.DecodeRuneInString(content[end:])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		end += size
	}
	if end == start {
		_, size := utf8.DecodeRuneInString(content[start:])
		end += size
	}
	return protocol.Range{Start: sourcePosition(content, start), End: sourcePosition(content, end)}
}

func byteColumn(content string, position protocol.Position) int {
	lines := strings.Split(content, "\n")
	if int(position.Line) >= len(lines) {
		return 0
	}
	line := strings.TrimSuffix(lines[position.Line], "\r")
	units := 0
	for index, r := range line {
		width := len(utf16.Encode([]rune{r}))
		if units+width > int(position.Character) {
			return index
		}
		units += width
	}
	return len(line)
}

func utf16Position(content string, position protocol.Position) protocol.Position {
	lines := strings.Split(content, "\n")
	if int(position.Line) >= len(lines) {
		return position
	}
	line := strings.TrimSuffix(lines[position.Line], "\r")
	column := int(position.Character)
	if column > len(line) {
		column = len(line)
	}
	for column > 0 && column < len(line) && !utf8.RuneStart(line[column]) {
		column--
	}
	position.Character = uint32(len(utf16.Encode([]rune(line[:column]))))
	return position
}
