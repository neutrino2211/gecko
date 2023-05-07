package codegen

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
