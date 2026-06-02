// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

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
