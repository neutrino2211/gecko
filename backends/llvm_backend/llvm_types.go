// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"

	"github.com/llir/llvm/ir/value"
)

type LLVMScopeInformation struct {
	ExecutionContext *ExecutionContext
	ProgramContext   *ModuleContext
	LocalContext     *LocalContext
	ChildContexts    map[string]*LocalContext
}

type LLVMValueInformation struct {
	Type       types.Type
	Value      value.Value
	GeckoType  *tokens.TypeRef
	IsVolatile bool
}

type LLVMScopeData map[string]*LLVMScopeInformation

type LLVMBackendImplementation struct {
	Backend interfaces.BackendInteface
}

type LLVMValuesMap map[string]*LLVMValueInformation

// LLVMStructInfo stores information about a struct/class type
type LLVMStructInfo struct {
	Type       *types.StructType // The LLVM struct type
	FieldNames []string          // Field names in order
	FieldTypes []*tokens.TypeRef // Gecko type references for each field
	IsPacked   bool              // Whether the struct is packed
}

// LLVMStructMap maps class names to their struct info
var LLVMStructMap = make(map[string]*LLVMStructInfo)

// TraitDefinitionOrigins stores the defining package for trait declarations.
// Maps trait name (e.g., "Iterator") to origin package (e.g., "traits").
var TraitDefinitionOrigins = make(map[string]string)
