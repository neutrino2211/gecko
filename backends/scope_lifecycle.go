// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// ScopeLifecycleHook provides callbacks for scope entry and exit events.
// This enables features like defer that need to emit cleanup code at scope exit.
type ScopeLifecycleHook interface {
	// OnScopeEnter is called before processing entries in a scope.
	OnScopeEnter(scope *ast.Ast, entries []*tokens.Entry)

	// OnScopeExit is called after processing all entries in a scope.
	// This is where deferred actions should be emitted.
	OnScopeExit(scope *ast.Ast, entries []*tokens.Entry)
}

// ScopeLifecycleRegistry manages registered scope lifecycle hooks.
type ScopeLifecycleRegistry struct {
	hooks []ScopeLifecycleHook
}

// GlobalScopeLifecycle is the global registry for scope lifecycle hooks.
var GlobalScopeLifecycle = &ScopeLifecycleRegistry{}

// Register adds a hook to the registry.
func (r *ScopeLifecycleRegistry) Register(hook ScopeLifecycleHook) {
	r.hooks = append(r.hooks, hook)
}

// NotifyScopeEnter calls all registered hooks for scope entry.
func (r *ScopeLifecycleRegistry) NotifyScopeEnter(scope *ast.Ast, entries []*tokens.Entry) {
	for _, hook := range r.hooks {
		hook.OnScopeEnter(scope, entries)
	}
}

// NotifyScopeExit calls all registered hooks for scope exit.
func (r *ScopeLifecycleRegistry) NotifyScopeExit(scope *ast.Ast, entries []*tokens.Entry) {
	for _, hook := range r.hooks {
		hook.OnScopeExit(scope, entries)
	}
}

// Reset clears all registered hooks.
func (r *ScopeLifecycleRegistry) Reset() {
	r.hooks = nil
}

// DeferHook is a ScopeLifecycleHook implementation that handles defer statements.
// It collects defer expressions during scope processing and emits them at scope exit.
type DeferHook struct {
	// EmitDefer is called for each deferred expression that should be emitted at scope exit.
	EmitDefer func(scope *ast.Ast, deferStmt *tokens.Defer)
}

func (h *DeferHook) OnScopeEnter(scope *ast.Ast, entries []*tokens.Entry) {
	// No action needed on scope enter for defer
}

func (h *DeferHook) OnScopeExit(scope *ast.Ast, entries []*tokens.Entry) {
	// Collect defer statements from the scope's entries
	for _, entry := range entries {
		if entry != nil && entry.Defer != nil && h.EmitDefer != nil {
			h.EmitDefer(scope, entry.Defer)
		}
	}
}
