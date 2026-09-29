package frontend

import (
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

func (r *Result) View() *View {
	return r.clone(nil, false)
}

func (r *Result) CloneForBackend(cfg *config.CompileCfg) *View {
	return r.clone(cfg, true)
}

func (r *Result) clone(cfg *config.CompileCfg, backend bool) *View {
	if r == nil {
		return nil
	}
	copy := &View{Path: r.path, Content: r.content, ParseError: r.parseError, state: newCompileState()}
	cloner := tokens.NewSyntaxCloner()
	copy.File = cloner.File(r.file)
	for path, file := range r.state.fileCache {
		copy.state.fileCache[path] = cloner.File(file)
	}
	for path, file := range r.state.importCache {
		copy.state.importCache[path] = cloner.File(file)
	}
	for _, node := range cloner.Nodes() {
		if file, ok := node.(*tokens.File); ok {
			if backend {
				file.Config = cfg
			} else if file.Config != nil {
				configCopy := *file.Config
				configCopy.Project = cloneProject(configCopy.Project)
				configCopy.Ctx = nil
				configCopy.CFlags = append([]string(nil), configCopy.CFlags...)
				configCopy.CLFlags = append([]string(nil), configCopy.CLFlags...)
				configCopy.CObjects = append([]string(nil), configCopy.CObjects...)
				file.Config = &configCopy
			}
		}
	}
	copy.Program = r.program.CloneForSyntax(copy.File, cloner.Nodes())
	copy.Scopes = cloneErrorScopes(r.scopes)
	return copy
}

func cloneErrorScopes(scopes []*errors.ErrorScope) []*errors.ErrorScope {
	cloned := make([]*errors.ErrorScope, len(scopes))
	for index, scope := range scopes {
		if scope == nil {
			continue
		}
		copy := *scope
		if scope.Source != nil {
			source := *scope.Source
			copy.Source = &source
		}
		cloneMessages := func(messages []*errors.CompileTimeMessage) []*errors.CompileTimeMessage {
			result := make([]*errors.CompileTimeMessage, len(messages))
			for messageIndex, message := range messages {
				if message == nil {
					continue
				}
				clonedMessage := *message
				clonedMessage.Scope = &copy
				clonedMessage.Notes = append([]string(nil), message.Notes...)
				clonedMessage.Related = append([]errors.RelatedLocation(nil), message.Related...)
				clonedMessage.Fixes = append([]errors.SuggestedFix(nil), message.Fixes...)
				result[messageIndex] = &clonedMessage
			}
			return result
		}
		copy.CompileTimeErrors = cloneMessages(scope.CompileTimeErrors)
		copy.CompileTimeWarnings = cloneMessages(scope.CompileTimeWarnings)
		cloned[index] = &copy
	}
	return cloned
}
