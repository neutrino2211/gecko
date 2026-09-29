// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md, spec/unsafe.md

package cbackend

import (
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/hooks"
	"github.com/neutrino2211/gecko/tokens"
)

var UnsafeHandlerCoverage = map[string][]string{}

// unsafeHandlerCounter generates unique C variable names for activated handlers.
var unsafeHandlerCounter = 0

// unsafeBlockCounter generates unique ids for @unsafe blocks used as expressions.
var unsafeBlockCounter = 0

// ResetUnsafeHandlerCoverage clears the global coverage map. Call once at the
// start of each compilation so stale entries from a previous file don't leak.
func ResetUnsafeHandlerCoverage() {
	UnsafeHandlerCoverage = map[string][]string{}
}

// normalizeHandlerTypeName strips a leading `module__` prefix so a handler type
// can be matched regardless of the module it is referenced from.
func normalizeHandlerTypeName(t string) string {
	if idx := strings.Index(t, "__"); idx >= 0 {
		return t[idx+2:]
	}
	return t
}

// isSimpleIdent reports whether s is a bare C identifier (no operators, calls,
// or field access), so it can be reused directly as a handler variable.
func isSimpleIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// stripAttributeString removes surrounding quotes from an attribute argument.
func stripAttributeString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// handlerCovers reports whether the given handler type guards the intrinsic.
func handlerCovers(typeName, intr string) bool {
	for _, c := range UnsafeHandlerCoverage[normalizeHandlerTypeName(typeName)] {
		if c == intr {
			return true
		}
	}
	return false
}

// scopeInUnsafe reports whether the current scope (or an ancestor) is an
// @unsafe function body or @unsafe { ... } block.
func scopeInUnsafe(scope *ast.Ast) bool {
	current := scope
	for current != nil {
		info, ok := (*CScopeDataMap)[current.GetFullName()]
		if ok && info != nil {
			if info.InUnsafe {
				return true
			}
			if info.FunctionBoundary {
				return false
			}
		}
		current = current.Parent
	}
	return false
}

func requireUnsafe(scope *ast.Ast, pos lexer.Position, what string) {
	requireUnsafeSpan(scope, pos, what, 0)
}

func requireUnsafeSpan(scope *ast.Ast, pos lexer.Position, what string, length int) {
	if scopeInUnsafe(scope) {
		return
	}
	message := scope.ErrorScope.NewCompileTimeError(
		"Unsafe Required",
		what+" is only allowed inside @unsafe functions or @unsafe { ... } blocks",
		pos,
	)
	if length > 0 {
		message.EndOffset = pos.Offset + length
	}
}

// unsafeIntrinsics is the allowlist of intrinsics that may only be used inside
// an @unsafe region. These perform raw memory operations (stores, typed/untyped
// pointer arithmetic, bulk copy/zero) that are not governed by Gecko's type
// system. Safe introspection intrinsics (size_of, is_null, builtin ops, ...) are
// intentionally absent.
var unsafeIntrinsics = map[string]bool{
	"deref":          true,
	"drop_in_place":  true,
	"alloc":          true,
	"free":           true,
	"write_volatile": true,
	"read_volatile":  true,
	"ptr_add":        true,
	"ptr_sub":        true,
	"copy":           true,
	"zero":           true,
	"trap":           true,
	"unreachable":    true,
}

// requireUnsafeIntrinsic errors when an allowlisted unsafe intrinsic is used
// outside an @unsafe region.
func requireUnsafeIntrinsic(name string, scope *ast.Ast, pos lexer.Position) {
	if !unsafeIntrinsics[name] {
		return
	}
	requireUnsafeSpan(scope, pos, "@"+name, len(name)+1)
}

// requireUnsafeCall errors when a function/method marked @unsafe is invoked
// outside an @unsafe region (Rust-style call-site requirement). The @unsafe flag
// is carried on ast.Method (populated from the source @unsafe attribute).
func requireUnsafeCall(method *ast.Method, scope *ast.Ast, pos lexer.Position) {
	if method == nil || !method.Unsafe {
		return
	}
	requireUnsafe(scope, pos, "call to @unsafe function '"+method.Name+"'")
}

func (impl *CBackendImplementation) NewUnsafeBlock(scope *ast.Ast, block *tokens.UnsafeBlock) {
	if block == nil {
		return
	}
	if block.Setup != nil || len(block.Handlers) > 0 {
		impl.newUnsafeSetupBlock(scope, block)
		return
	}
	info := CGetScopeInformation(scope)
	previous := info.InUnsafe
	info.InUnsafe = true
	for _, entry := range block.Body {
		impl.processEntry(scope, entry)
	}
	info.InUnsafe = previous
}

// markMethodScopeUnsafe sets InUnsafe when the method carries @unsafe.
func markMethodScopeUnsafe(m *tokens.Method, info *CScopeInformation) {
	if m != nil && info != nil && tokens.HasAttribute(m.Attributes, "unsafe") {
		info.InUnsafe = true
	}
}

func visibleUnsafeHandlerHook(scope *ast.Ast) *hooks.RegisteredHook {
	if scope == nil {
		return nil
	}
	candidates := visibleHooks(scope, hooks.HookUnsafeHandler)
	if len(candidates) != 1 {
		return nil
	}
	return candidates[0]
}

func unsafeOperationType(hook *hooks.RegisteredHook, scope *ast.Ast) string {
	if hook == nil || len(hook.Methods) != 2 {
		return ""
	}
	trait := TraitDefinitions[hook.TraitName]
	if trait == nil {
		return ""
	}
	for _, field := range trait.Fields {
		if field.Name != hook.Methods[0] || len(field.Arguments) < 2 || field.Arguments[1].Type == nil {
			continue
		}
		return TypeRefToCType(field.Arguments[1].Type, scope)
	}
	return ""
}

// unsafeHandlerVariant returns the (globally unique) C enumerator name for the
// idx-th handler inside the given synthesized error enum.
func unsafeHandlerVariant(enumName string, idx int) string {
	return enumName + "_E" + strconv.Itoa(idx)
}

// registerUnsafeErrorEnum emits a synthesized C enum type whose variants are the
// handler ids (one per active handler), and registers it so `Result<T, E>` can
// be instantiated. The enum name comes from the analyzer (block.ErrorTypeName).
// The typedef is emitted on the ROOT scope so it lands in the global "Type definitions"
// section (function-scope type defs are not emitted to the final file).
func registerUnsafeErrorEnum(scope *ast.Ast, info *CScopeInformation, enumName string, handlers []*UnsafeHandlerInstance) {
	// The same error enum may be reused across reassignments of one Result
	// variable; only define it once (the emitter dedups StructDefs but a second
	// typedef would still be a C redefinition error).
	if _, ok := EnumToCType[enumName]; ok {
		return
	}
	var sb strings.Builder
	sb.WriteString("typedef enum {\n")
	for idx := range handlers {
		variant := unsafeHandlerVariant(enumName, idx)
		sb.WriteString("    " + variant + " = " + strconv.Itoa(idx) + ",\n")
	}
	sb.WriteString("} " + enumName + ";")
	rootInfo := CGetScopeInformation(scope.GetRoot())
	// StructDefs is what the emitter prints (topologically sorted); Types is only
	// a fallback used when StructDefs is empty, so it is intentionally not used here.
	rootInfo.StructDefs = append(rootInfo.StructDefs, &StructDefinition{
		Name: enumName,
		Code: sb.String(),
	})

	EnumToCType[enumName] = enumName
	enumAst := &ast.Ast{
		Scope:        enumName,
		Parent:       scope.GetRoot(),
		OriginModule: scope.GetRoot().Scope,
		SourceFile:   scope.GetSourceFile(),
	}
	enumAst.Init(scope.ErrorScope)
	scope.GetRoot().Classes[enumName] = enumAst
}

// bindingIsConst reports whether a field declares an immutable binding.
// Type-level `readonly` on a non-pointer value also forbids reassignment.
// `T readonly*` does not — the pointer may be rebound; only the pointee is readonly.
func bindingIsConst(f *tokens.Field) bool {
	return f.IsConstBinding()
}

func argBindingIsConst(t *tokens.TypeRef) bool {
	return t != nil && t.Const && !t.Pointer
}

// emitBindingConstPrefix is true when the C declaration needs an extra `const`
// from binding mutability. Type-level `readonly` is already emitted by TypeRefToCType.
func emitBindingConstPrefix(f *tokens.Field) bool {
	return f != nil && f.Mutability == "const"
}
