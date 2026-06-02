// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import "github.com/llir/llvm/ir/enum"

var CallingConventions map[string]map[string]enum.CallingConv = map[string]map[string]enum.CallingConv{
	"arm64": {
		"darwin":  enum.CallingConvC,
		"linux":   enum.CallingConvC,
		"windows": enum.CallingConvC,
	},
	"amd64": {
		"darwin":  enum.CallingConvC,
		"linux":   enum.CallingConvC,
		"windows": enum.CallingConvWin64,
	},
}
