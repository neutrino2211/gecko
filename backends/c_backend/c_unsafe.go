// spec: spec/types.md, spec/attributes.md, spec/pointers.md, spec/operators.md

package cbackend

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// UnsafeHandlerCoverage is the global, program-wide map from a handler type
// name to the intrinsics it guards. It is populated from `@attach_handler(...)`
// attributes on `impl UnsafeHandler for X` blocks (see NewImplementation) and
// read when lowering intrinsics inside `@unsafe with` blocks.
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
		if ok && info != nil && info.InUnsafe {
			return true
		}
		current = current.Parent
	}
	return false
}

func requireUnsafe(scope *ast.Ast, pos lexer.Position, what string) {
	if scopeInUnsafe(scope) {
		return
	}
	scope.ErrorScope.NewCompileTimeError(
		"Unsafe Required",
		what+" is only allowed inside @unsafe functions or @unsafe { ... } blocks",
		pos,
	)
}

// unsafeIntrinsics is the allowlist of intrinsics that may only be used inside
// an @unsafe region. These perform raw memory operations (stores, typed/untyped
// pointer arithmetic, bulk copy/zero) that are not governed by Gecko's type
// system. Safe introspection intrinsics (size_of, is_null, builtin ops, ...) are
// intentionally absent.
var unsafeIntrinsics = map[string]bool{
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
	requireUnsafe(scope, pos, "@"+name)
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

// NewUnsafeBlock lowers `@unsafe { ... }` (and `@unsafe with H { ... }`) by
// temporarily marking the scope unsafe, activating any handler instances from
// the `with` clause, processing the body (whose intrinsics are guarded by the
// active handlers), then detaching the handlers.
//
// The block may bind a Result<T, E> via its binder name, reachable from two
// syntactic forms (the binder is set programmatically, not parsed):
//   - expression  `let r = @unsafe with H { ... }`  -> declares `r`
//   - reassignment `r = @unsafe with H { ... }`       -> the binder is a fresh
//     temp that the caller copies into the existing `r`.
//
// A bare `@unsafe with H { ... }` statement (no binder) just runs the guarded
// body for its side effects (e.g. a catch that traps) and yields no Result.
//
// On success the Result holds Ok(trailing); on the first failing guard the
// intrinsic is skipped and the Result holds Err(handler variant).
func (impl *CBackendImplementation) NewUnsafeBlock(scope *ast.Ast, block *tokens.UnsafeBlock) {
	if block == nil {
		return
	}
	info := CGetScopeInformation(scope)
	prev := info.InUnsafe
	info.InUnsafe = true

	resultName := tokens.UnsafeBlockBindNames[block]
	hasResult := resultName != ""
	unsafeBlockCounter++
	id := unsafeBlockCounter
	failFlag := ""
	failIdx := ""
	if hasResult {
		failFlag = fmt.Sprintf("__gecko_unsafe_fail%d", id)
		failIdx = fmt.Sprintf("__gecko_unsafe_fidx%d", id)
	}

	pushed := 0
	for idx, h := range block.Handlers {
		if h == nil {
			continue
		}
		hExpr := h.ToExpression()
		typeName := strings.TrimSpace(impl.inferExpressionType(hExpr, scope))
		handlerC := strings.TrimSpace(impl.ExpressionToCString(hExpr, scope))
		var varName string
		if isSimpleIdent(handlerC) {
			// The handler is a bare variable; reuse it directly so any state
			// its catch sets persists after the block.
			varName = handlerC
		} else {
			unsafeHandlerCounter++
			varName = fmt.Sprintf("__gecko_unsafe_h%d", unsafeHandlerCounter)
			info.Code += "    " + typeName + " " + varName + " = " + handlerC + ";\n"
		}
		info.ActiveUnsafeHandlers = append(info.ActiveUnsafeHandlers, &UnsafeHandlerInstance{
			VarName:  varName,
			TypeName: typeName,
			Index:    idx,
			FailFlag: failFlag,
			FailIdx:  failIdx,
		})
		pushed++
	}

	if hasResult {
		info.Code += "    int " + failFlag + " = 0;\n"
		info.Code += "    int " + failIdx + " = -1;\n"
	}

	// The trailing expression (if any) is the block's value, captured into the
	// Result instead of being emitted as a bare statement.
	last := len(block.Body) - 1
	emitBody := block.Body
	var trailingC string
	var trailingType *tokens.TypeRef
	if hasResult && last >= 0 && block.Body[last].ExprStmt != nil {
		trailingType = impl.GetTypeOfExpression(block.Body[last].ExprStmt, scope)
		trailingC = impl.ExpressionToCString(block.Body[last].ExprStmt, scope)
		emitBody = block.Body[:last]
	}

	for _, entry := range emitBody {
		impl.processEntry(scope, entry)
	}

	if hasResult {
		enumName := tokens.UnsafeBlockErrorNames[block]
		registerUnsafeErrorEnum(scope, info, enumName, info.ActiveUnsafeHandlers[len(info.ActiveUnsafeHandlers)-pushed:])

		if trailingType == nil {
			trailingType = &tokens.TypeRef{Type: "void"}
		}
		resType := TypeRefToCType(&tokens.TypeRef{
			Type:     "Result",
			TypeArgs: []*tokens.TypeRef{trailingType, &tokens.TypeRef{Type: enumName}},
		}, scope)
		isVoid := trailingType.Type == "void"

		// Method names are module-qualified (e.g. result__Result__...__ok) when the
		// Result class originates from a module, so build the prefix from the
		// generic class origin rather than guessing.
		methodPrefix := resType
		if origin := Generics.GenericClassOrigins["Result"]; origin != "" {
			methodPrefix = origin + "__" + resType
		}

		if isVoid {
			info.Code += "    " + resType + " " + resultName + ";\n"
		} else {
			info.Code += "    " + resType + " " + resultName + " = " + methodPrefix + "__ok(" + trailingC + ");\n"
		}
		// Register the bound name so later method calls (ok_r.is_error(),
		// ok_r.unwrap()) resolve against the synthesized Result type.
		resultVar := ast.Variable{Name: resultName, Parent: scope}
		scope.Variables[resultName] = resultVar
		(*CProgramValues)[resultVar.GetFullName()] = &CValueInformation{
			CType: resType,
			GeckoType: &tokens.TypeRef{
				Type:     "Result",
				TypeArgs: []*tokens.TypeRef{trailingType, &tokens.TypeRef{Type: enumName}},
			},
		}
		// On failure, overwrite with the failing handler's error variant.
		info.Code += "    if (" + failFlag + ") {\n"
		info.Code += "        " + enumName + " __errv;\n"
		info.Code += "        switch (" + failIdx + ") {\n"
		for idx := range block.Handlers {
			if block.Handlers[idx] == nil {
				continue
			}
			variant := unsafeHandlerVariant(enumName, idx)
			info.Code += "            case " + strconv.Itoa(idx) + ": __errv = " + variant + "; break;\n"
		}
		info.Code += "        }\n"
		if isVoid {
			info.Code += "        " + resultName + ".error = __errv; " + resultName + ".is_ok = 0;\n"
		} else {
			info.Code += "        " + resultName + " = " + methodPrefix + "__err(__errv);\n"
		}
		info.Code += "    }\n"
	}

	info.ActiveUnsafeHandlers = info.ActiveUnsafeHandlers[:len(info.ActiveUnsafeHandlers)-pushed]
	info.InUnsafe = prev
}

// markMethodScopeUnsafe sets InUnsafe when the method carries @unsafe.
func markMethodScopeUnsafe(m *tokens.Method, info *CScopeInformation) {
	if m != nil && info != nil && tokens.HasAttribute(m.Attributes, "unsafe") {
		info.InUnsafe = true
	}
}

// unsafePtrArg returns the C expression for the pointer an intrinsic operates on
// (used to populate UnsafeOp.ptr). Returns "0" for control-flow intrinsics.
func unsafePtrArg(impl *CBackendImplementation, scope *ast.Ast, i *tokens.Intrinsic) string {
	switch i.Name {
	case "trap", "unreachable":
		return "0"
	default:
		if len(i.Args) > 0 {
			return impl.ExpressionToCString(i.Args[0], scope)
		}
		return "0"
	}
}

// unsafeSizeArg returns the C expression for the byte/element count an intrinsic
// covers (used to populate UnsafeOp.size).
func unsafeSizeArg(impl *CBackendImplementation, scope *ast.Ast, i *tokens.Intrinsic) string {
	switch i.Name {
	case "copy":
		if len(i.Args) > 2 {
			return impl.ExpressionToCString(i.Args[2], scope)
		}
	case "ptr_add", "ptr_sub":
		if len(i.Args) > 1 {
			return impl.ExpressionToCString(i.Args[1], scope)
		}
	}
	return "0"
}

// unsafeOpCTypeName resolves the C type name for the `UnsafeOp` struct the same
// way the backend names it for a `use`-imported type, so the guard wrapper's
// variable declaration matches the signatures of the handler's guard/catch.
func unsafeOpCTypeName(scope *ast.Ast) string {
	return TypeRefToCType(&tokens.TypeRef{Type: "UnsafeOp"}, scope)
}

// wrapUnsafeGuards wraps an intrinsic's generated C code with guard/catch calls
// for every active handler (in the current `@unsafe with` scope) that covers the
// intrinsic. Each handler's guard result is captured into its own variable; the
// group passes only if every handler passes (all results ANDed together). For
// each failing handler, catch runs and the intrinsic is skipped when the group
// fails. Returns the code unchanged when no covering handler is active.
func (impl *CBackendImplementation) wrapUnsafeGuards(scope *ast.Ast, i *tokens.Intrinsic, intrinsicCode string) string {
	info := CGetScopeInformation(scope)
	if len(info.ActiveUnsafeHandlers) == 0 {
		return intrinsicCode
	}

	var checks strings.Builder
	var results []string
	for idx, h := range info.ActiveUnsafeHandlers {
		if !handlerCovers(h.TypeName, i.Name) {
			continue
		}
		guard := h.TypeName + "__UnsafeHandler__guard"
		catch := h.TypeName + "__UnsafeHandler__catch"
		resVar := fmt.Sprintf("__unsafe_ok%d", idx)
		// Capture this handler's result, then run its catch if it failed.
		checks.WriteString("        int " + resVar + " = " + guard + "(&" + h.VarName + ", __op);\n")
		checks.WriteString("        if (!" + resVar + ") { " + catch + "(&" + h.VarName + ", __op);")
		if h.FailFlag != "" {
			checks.WriteString(" " + h.FailFlag + " = 1; if (" + h.FailIdx + " < 0) { " + h.FailIdx + " = " + strconv.Itoa(h.Index) + "; }")
		}
		checks.WriteString(" }\n")
		results = append(results, resVar)
	}
	if len(results) == 0 {
		return intrinsicCode
	}

	// Group decision: the operation is permitted only if all handlers passed.
	group := strings.Join(results, " && ")

	opType := unsafeOpCTypeName(scope)
	ptr := unsafePtrArg(impl, scope, i)
	size := unsafeSizeArg(impl, scope, i)
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString("        " + opType + " __op = { .ptr = (void*)(" + ptr + "), .size = (" + size + ") };\n")
	sb.WriteString(checks.String())
	sb.WriteString("        int __ok = " + group + ";\n")
	sb.WriteString("        if (__ok) { " + intrinsicCode + "; }\n")
	sb.WriteString("    }")
	return sb.String()
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
	if f == nil {
		return false
	}
	if f.Mutability == "const" {
		return true
	}
	if f.Type != nil && f.Type.Const && !f.Type.Pointer {
		return true
	}
	if f.Type != nil && f.Type.Size != nil && f.Type.Size.Type != nil && f.Type.Size.Type.Const && !f.Type.Pointer {
		return true
	}
	return false
}

func argBindingIsConst(t *tokens.TypeRef) bool {
	return t != nil && t.Const && !t.Pointer
}

// emitBindingConstPrefix is true when the C declaration needs an extra `const`
// from binding mutability. Type-level `readonly` is already emitted by TypeRefToCType.
func emitBindingConstPrefix(f *tokens.Field) bool {
	return f != nil && f.Mutability == "const"
}
