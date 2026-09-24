// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

var CurrentBackend interfaces.BackendInterface = nil

var Methods map[string]*ast.Method

const stdTryDiagnosticsInstallerMethod = "__gecko_install_try_error_handler"

// Init initializes scope info for an AST
func (info *CScopeInformation) InitForAst(a *ast.Ast) {
	info.Init()
	loadPrimitives(a)
}

func loadPrimitives(a *ast.Ast) {
	// Register primitive types as classes in the AST
	for typeName := range GeckoToCType {
		a.Classes[typeName] = &ast.Ast{
			Scope: typeName,
		}
	}
}

func isStdErrorsSource(path string) bool {
	if path == "" {
		return false
	}
	normalized := filepath.ToSlash(path)
	return normalized == "stdlib/errors.gecko" || strings.HasSuffix(normalized, "/stdlib/errors.gecko")
}

func findStdTryDiagnosticsInstaller(root *ast.Ast) string {
	if root == nil {
		return ""
	}

	visited := make(map[*ast.Ast]bool)
	var walk func(scope *ast.Ast) string
	walk = func(scope *ast.Ast) string {
		if scope == nil || visited[scope] {
			return ""
		}
		visited[scope] = true

		if isStdErrorsSource(scope.GetSourceFile()) {
			if m, ok := scope.Methods[stdTryDiagnosticsInstallerMethod]; ok {
				return m.GetFullName()
			}
		}

		for _, child := range scope.Children {
			if installer := walk(child); installer != "" {
				return installer
			}
		}
		return ""
	}

	return walk(root)
}

func scopeFindMethodByNameOrQualifiedSuffix(scope *ast.Ast, target string) (string, bool) {
	if scope == nil || target == "" {
		return "", false
	}
	visited := make(map[*ast.Ast]bool)
	queue := []*ast.Ast{scope}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == nil || visited[current] {
			continue
		}
		visited[current] = true
		for existing := range current.Methods {
			if existing == target || strings.HasSuffix(existing, "__"+target) {
				return existing, true
			}
		}
		for _, child := range current.Children {
			if child != nil && !visited[child] {
				queue = append(queue, child)
			}
		}
	}
	return "", false
}

// inferExpressionType infers the C type of an expression for temp variable declaration
func (impl *CBackendImplementation) inferExpressionType(expr *tokens.Expression, scope *ast.Ast) string {
	exprType := impl.GetTypeOfExpression(expr, scope)
	if exprType != nil {
		return TypeRefToCType(exprType, scope)
	}
	return ""
}

// Declaration handles declaration tokens
func (*CBackendImplementation) Declaration(scope *ast.Ast, decl *tokens.Declaration) {
	// Handled by NewDeclaration
}

// ParseExpression parses an expression
func (*CBackendImplementation) ParseExpression(scope *ast.Ast, exp *tokens.Expression) {
	// No-op, expressions are processed inline
}

// ProcessEntries processes all entries in a scope
func (impls *CBackendImplementation) ProcessEntries(scope *ast.Ast, entries []*tokens.Entry) {
	impls.Backend.ProcessEntries(entries, scope)
}

// ExprStatement handles expression statements (e.g., i++, method calls without side effects)
func (impl *CBackendImplementation) ExprStatement(scope *ast.Ast, expr *tokens.Expression) {
	info := CGetScopeInformation(scope)
	exprStr := impl.ExpressionToCString(expr, scope)
	info.Code += fmt.Sprintf("    %s;\n", exprStr)
}

// processEntry handles a single entry in a block (for control flow bodies)
func (impl *CBackendImplementation) processEntry(scope *ast.Ast, entry *tokens.Entry) {
	if entry.Return != nil {
		impl.NewReturnLiteral(scope, entry.Return)
	} else if entry.VoidReturn != nil {
		impl.NewReturn(scope)
	} else if entry.Break != nil {
		impl.NewBreak(scope)
	} else if entry.Continue != nil {
		impl.NewContinue(scope)
	} else if entry.Intrinsic != nil {
		impl.IntrinsicStatement(scope, entry.Intrinsic)
	} else if entry.MethodCall != nil {
		impl.MethodCall(scope, entry.MethodCall)
	} else if entry.FuncCall != nil {
		impl.FuncCall(scope, entry.FuncCall)
	} else if entry.Field != nil {
		impl.NewVariable(scope, entry.Field)
	} else if entry.Destructuring != nil {
		impl.DestructuringDeclaration(scope, entry.Destructuring)
	} else if entry.Assignment != nil {
		impl.NewAssignment(scope, entry.Assignment)
	} else if entry.If != nil {
		impl.NewIf(scope, entry.If)
	} else if entry.Match != nil {
		impl.NewMatch(scope, entry.Match)
	} else if entry.Defer != nil {
		impl.NewDefer(scope, entry.Defer)
	} else if entry.Loop != nil {
		impl.NewLoop(scope, entry.Loop)
	} else if entry.Asm != nil {
		impl.NewAsm(scope, entry.Asm)
	} else if entry.ExprStmt != nil {
		impl.ExprStatement(scope, entry.ExprStmt)
	} else if entry.IncDec != nil {
		impl.NewIncDec(scope, entry.IncDec)
	} else if entry.UnsafeBlock != nil {
		impl.NewUnsafeBlock(scope, entry.UnsafeBlock)
	}
}
