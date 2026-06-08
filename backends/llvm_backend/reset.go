package llvmbackend

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/ast"
)

// ResetState clears package-level backend state between compilations.
func ResetState() {
	CurrentBackend = nil
	FuncCalls = make(map[string]*ir.InstCall)
	Methods = make(map[string]*ast.Method)
	LLVMExecutionContext = nil

	LLVMScopeDataMap = &LLVMScopeData{}
	LLVMProgramValues = &LLVMValuesMap{}
	LLVMStructMap = make(map[string]*LLVMStructInfo)
	LLVMOpaqueTypeMap = make(map[string]*types.StructType)
	LLVMEnumMap = make(map[string]*LLVMEnumInfo)
	TraitDefinitionOrigins = make(map[string]string)
	TraitParents = make(map[string]string)
}
