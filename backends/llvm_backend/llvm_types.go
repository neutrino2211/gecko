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
	Type      types.Type
	Value     value.Value
	GeckoType *tokens.TypeRef
}

type LLVMScopeData map[string]*LLVMScopeInformation

type LLVMBackendImplementation struct {
	Backend interfaces.BackendInterface
}

type LLVMValuesMap map[string]*LLVMValueInformation
