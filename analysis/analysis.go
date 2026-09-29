package analysis

import (
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/frontend"
	"github.com/neutrino2211/gecko/semantic"
	"github.com/neutrino2211/gecko/tokens"
)

type AnalysisContext struct {
	MainFile      *tokens.File
	ImportedFiles map[string]*tokens.File
	FilePath      string
	SourceContent string
	SemanticGraph *semantic.Program
	Frontend      *frontend.Result
	filesByPath   map[string]*tokens.File
	moduleFiles   map[string][]*tokens.File
}

func NewAnalysisContext(path, content string) (*AnalysisContext, error) {
	return NewAnalysisContextWithReader(path, content, os.ReadFile)
}

func NewAnalysisContextWithReader(path, content string, readSource func(string) ([]byte, error)) (*AnalysisContext, error) {
	return NewAnalysisContextWithSession(path, content, readSource, frontend.NewSession())
}

func NewAnalysisContextWithSession(path, content string, readSource func(string) ([]byte, error), session *frontend.Session) (*AnalysisContext, error) {
	project, err := config.LoadProjectConfigWithReader(filepath.Dir(path), readSource)
	if err != nil && !stderrors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	result := session.Analyze(path, content, &config.CompileCfg{Project: project, ReadSource: readSource})
	if result.ParseError() != nil {
		return nil, result.ParseError()
	}
	return FromFrontend(result), nil
}

func FromFrontend(result *frontend.Result) *AnalysisContext {
	view := result.View()
	if view == nil || view.File == nil {
		return nil
	}
	ctx := &AnalysisContext{MainFile: view.File, FilePath: view.File.Path, SourceContent: view.File.Content, SemanticGraph: view.Program, Frontend: result, ImportedFiles: make(map[string]*tokens.File), filesByPath: make(map[string]*tokens.File), moduleFiles: make(map[string][]*tokens.File)}
	seen := make(map[*tokens.File]bool)
	var index func(*tokens.File)
	index = func(file *tokens.File) {
		if file == nil || seen[file] {
			return
		}
		seen[file] = true
		ctx.filesByPath[filepath.Clean(file.Path)] = file
		for _, imported := range file.Imports {
			index(imported)
		}
	}
	index(view.File)
	for _, file := range view.File.Imports {
		ctx.moduleFiles[file.Name] = append(ctx.moduleFiles[file.Name], file)
		if ctx.ImportedFiles[file.Name] == nil {
			ctx.ImportedFiles[file.Name] = file
		}
	}
	return ctx
}

func (ctx *AnalysisContext) FilesForModule(module string) []*tokens.File {
	if ctx == nil {
		return nil
	}
	return append([]*tokens.File(nil), ctx.moduleFiles[module]...)
}

func (ctx *AnalysisContext) FileForSymbol(id int64) *tokens.File {
	if ctx == nil || ctx.SemanticGraph == nil {
		return nil
	}
	for _, occurrence := range ctx.SemanticGraph.OccurrencesFor(id, true) {
		if occurrence.Declaration {
			return ctx.filesByPath[filepath.Clean(occurrence.FilePath)]
		}
	}
	return nil
}
