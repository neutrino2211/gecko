// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package cbackend

import (
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

// CheckCast validates cast rules:
//   - Word qualifiers (readonly, volatile) may be added with `as`, stripped only with `as!`
//   - `as!` requires an @unsafe region
//
// Symbol rules (pointer ↔ integer, pointee changes) are enforced incrementally;
// today those remain expressible with plain `as` for compatibility with existing
// stdlib FFI patterns, but discarding word quals is not.
func (impl *CBackendImplementation) CheckCast(source *tokens.TypeRef, target *tokens.TypeRef, trusted bool, scope *ast.Ast, pos lexer.Position) {
	if target == nil || scope == nil || scope.ErrorScope == nil {
		return
	}

	if trusted {
		requireUnsafe(scope, pos, "as!")
		return
	}

	if source == nil {
		return
	}

	// Word qualifiers: may add with as, must not strip with as
	if source.Const && !target.Const {
		scope.ErrorScope.NewCompileTimeError(
			"Unsafe Cast Required",
			fmt.Sprintf("discarding readonly from '%s' requires as! inside @unsafe", FormatTypeRef(source)),
			pos,
		)
		return
	}
	if source.Volatile && !target.Volatile {
		scope.ErrorScope.NewCompileTimeError(
			"Unsafe Cast Required",
			fmt.Sprintf("discarding volatile from '%s' requires as! inside @unsafe", FormatTypeRef(source)),
			pos,
		)
		return
	}
}

// CheckReadonlyStore errors if storing through a readonly pointer/view.
func CheckReadonlyStore(ptrType *tokens.TypeRef, scope *ast.Ast, pos lexer.Position) {
	if ptrType == nil || !ptrType.Const {
		return
	}
	scope.ErrorScope.NewCompileTimeError(
		"Readonly Violation",
		fmt.Sprintf("cannot write through readonly type '%s'", FormatTypeRef(ptrType)),
		pos,
	)
}
