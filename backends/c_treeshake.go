// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"strings"

	cbackend "github.com/neutrino2211/gecko/backends/c_backend"
)

func isCIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func extractTrailingIdentifier(s string) string {
	if s == "" {
		return ""
	}
	i := len(s) - 1
	for i >= 0 && !isCIdentChar(s[i]) {
		i--
	}
	if i < 0 {
		return ""
	}
	end := i
	for i >= 0 && isCIdentChar(s[i]) {
		i--
	}
	return s[i+1 : end+1]
}

func extractFunctionNameFromSignature(sig string) string {
	trimmed := strings.TrimSpace(sig)
	if trimmed == "" {
		return ""
	}
	open := strings.Index(trimmed, "(")
	if open < 0 {
		return ""
	}

	// Handles `ret (name)(...)` declarations.
	if open+1 < len(trimmed) && isCIdentChar(trimmed[open+1]) {
		closeParen := strings.Index(trimmed[open+1:], ")")
		if closeParen >= 0 {
			closeParen += open + 1
			if closeParen+1 < len(trimmed) && trimmed[closeParen+1] == '(' {
				name := strings.TrimSpace(trimmed[open+1 : closeParen])
				if name != "" {
					return name
				}
			}
		}
	}

	before := strings.TrimSpace(trimmed[:open])
	// Handles both `ret name(` and `ret (name)(` forms.
	if strings.HasSuffix(before, ")") {
		before = strings.TrimSuffix(before, ")")
	}
	return extractTrailingIdentifier(before)
}

func extractFunctionBody(def string) string {
	start := strings.Index(def, "{")
	end := strings.LastIndex(def, "}")
	if start < 0 || end <= start {
		return ""
	}
	return def[start+1 : end]
}

func containsIdentifier(body string, ident string) bool {
	if body == "" || ident == "" {
		return false
	}
	searchFrom := 0
	for {
		idx := strings.Index(body[searchFrom:], ident)
		if idx < 0 {
			return false
		}
		idx += searchFrom
		beforeOK := idx == 0 || !isCIdentChar(body[idx-1])
		afterPos := idx + len(ident)
		afterOK := afterPos >= len(body) || !isCIdentChar(body[afterPos])
		if beforeOK && afterOK {
			return true
		}
		searchFrom = idx + len(ident)
		if searchFrom >= len(body) {
			return false
		}
	}
}

func pruneUnreachableCFunctions(info *cbackend.CScopeInformation) {
	if info == nil || len(info.Functions) == 0 {
		return
	}

	funcDefs := make(map[string]string, len(info.Functions))
	funcOrder := make([]string, 0, len(info.Functions))
	for _, def := range info.Functions {
		name := extractFunctionNameFromSignature(def)
		if name == "" {
			continue
		}
		if _, seen := funcDefs[name]; !seen {
			funcOrder = append(funcOrder, name)
		}
		funcDefs[name] = def
	}

	if len(funcDefs) == 0 {
		return
	}

	roots := make(map[string]bool)
	for _, root := range info.ExternalRootSymbols {
		if _, ok := funcDefs[root]; ok {
			roots[root] = true
		}
	}
	if _, ok := funcDefs["main"]; ok {
		roots["main"] = true
	}
	if _, ok := funcDefs["__main"]; ok {
		roots["__main"] = true
	}
	// Library-style transpilation may have no executable roots.
	if len(roots) == 0 {
		return
	}

	edges := make(map[string]map[string]bool, len(funcDefs))
	for name, def := range funcDefs {
		body := extractFunctionBody(def)
		callees := make(map[string]bool)
		for candidate := range funcDefs {
			if candidate == name {
				continue
			}
			if containsIdentifier(body, candidate) {
				callees[candidate] = true
			}
		}
		edges[name] = callees
	}

	reachable := make(map[string]bool)
	queue := make([]string, 0, len(roots))
	for root := range roots {
		reachable[root] = true
		queue = append(queue, root)
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for callee := range edges[current] {
			if !reachable[callee] {
				reachable[callee] = true
				queue = append(queue, callee)
			}
		}
	}

	prunedFunctions := make([]string, 0, len(info.Functions))
	for _, name := range funcOrder {
		if reachable[name] {
			prunedFunctions = append(prunedFunctions, funcDefs[name])
		}
	}
	info.Functions = prunedFunctions

	// Remove forward declarations for internal functions that were pruned.
	prunedDeclarations := make([]string, 0, len(info.Declarations))
	for _, decl := range info.Declarations {
		declName := extractFunctionNameFromSignature(decl)
		if declName == "" {
			prunedDeclarations = append(prunedDeclarations, decl)
			continue
		}
		if _, isGeneratedFunc := funcDefs[declName]; isGeneratedFunc && !reachable[declName] {
			continue
		}
		prunedDeclarations = append(prunedDeclarations, decl)
	}
	info.Declarations = prunedDeclarations
}
