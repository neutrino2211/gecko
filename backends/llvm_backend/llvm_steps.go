package llvmbackend

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/value"
)

type ConditionStep struct {
	Expression  *value.Value
	BranchTrue  *ir.Block
	BranchFalse *ir.Block
}

type AssignmentStep struct {
	Source      *value.Value
	Destination *value.Value
}

type DeclarationStep struct {
	Name  string
	Value *value.Value
}

type CallStep struct {
	Func   *ir.Func
	Params []*value.Value
}

type Step struct {
	Call        *CallStep
	Condition   *ConditionStep
	Assignment  *AssignmentStep
	Declaration *DeclarationStep
}
