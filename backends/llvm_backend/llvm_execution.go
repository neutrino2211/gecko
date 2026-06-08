// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package llvmbackend

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/go-option"
)

type ModuleContext struct {
	Module            *ir.Module
	GlobalDefinitions map[string]*ir.Global
	Methods           map[string]*ir.Func
	ExternalRootFuncs []*ir.Func
}

type LocalContext struct {
	ModuleContext *ModuleContext
	Func          *ir.Func
	MainBlock     *ir.Block
	Branches      map[string]*ir.Block
	Types         map[string]*types.Type
	LoopBreak     []*ir.Block
	LoopContinue  []*ir.Block
}

type ExecutionContext struct {
	Context *ModuleContext
	Steps   []*Step
}

func (m *ModuleContext) Init() {
	m.Module = ir.NewModule()
	m.GlobalDefinitions = make(map[string]*ir.Global)
	m.Methods = make(map[string]*ir.Func)
	m.ExternalRootFuncs = make([]*ir.Func, 0)
}

func (l *LocalContext) Init(fn *ir.Func) {
	if l.Func == nil {
		l.Func = fn
	}
	if l.Branches == nil {
		l.Branches = make(map[string]*ir.Block)
	}
	if l.Types == nil {
		l.Types = make(map[string]*types.Type)
	}
	if l.LoopBreak == nil {
		l.LoopBreak = make([]*ir.Block, 0)
	}
	if l.LoopContinue == nil {
		l.LoopContinue = make([]*ir.Block, 0)
	}
}

func (e *ExecutionContext) Init() {
	e.Context = &ModuleContext{}
	e.Context.Init()
	e.Steps = make([]*Step, 0)
}

func (e *ExecutionContext) FindLLIRMethod(name string) *option.Optional[*ir.Func] {
	fn, ok := e.Context.Methods[name]

	if ok {
		return option.Some(fn)
	}

	return option.None[*ir.Func]()
}

func NewExecutionContext() *ExecutionContext {
	ctx := &ExecutionContext{}
	ctx.Init()
	return ctx
}

func NewLocalContext(fn *ir.Func) *LocalContext {
	ctx := &LocalContext{}
	ctx.Init(fn)
	return ctx
}

func (m *ModuleContext) RegisterExternalRoot(fn *ir.Func) {
	if fn == nil {
		return
	}
	for _, existing := range m.ExternalRootFuncs {
		if existing == fn || existing.Name() == fn.Name() {
			return
		}
	}
	m.ExternalRootFuncs = append(m.ExternalRootFuncs, fn)
}

func (m *ModuleContext) EmitExternalRootAnchors() {
	if m == nil || m.Module == nil || len(m.ExternalRootFuncs) == 0 {
		return
	}

	for _, g := range m.Module.Globals {
		if g.Name() == "llvm.used" {
			return
		}
	}

	elems := make([]constant.Constant, 0, len(m.ExternalRootFuncs))
	for _, fn := range m.ExternalRootFuncs {
		elems = append(elems, constant.NewBitCast(fn, types.I8Ptr))
	}
	arrType := types.NewArray(uint64(len(elems)), types.I8Ptr)
	anchor := m.Module.NewGlobalDef("llvm.used", constant.NewArray(arrType, elems...))
	anchor.Linkage = enum.LinkageAppending
	anchor.Section = "llvm.metadata"
}

func (l *LocalContext) PushLoopTargets(breakTarget *ir.Block, continueTarget *ir.Block) {
	l.LoopBreak = append(l.LoopBreak, breakTarget)
	l.LoopContinue = append(l.LoopContinue, continueTarget)
}

func (l *LocalContext) PopLoopTargets() {
	if len(l.LoopBreak) > 0 {
		l.LoopBreak = l.LoopBreak[:len(l.LoopBreak)-1]
	}
	if len(l.LoopContinue) > 0 {
		l.LoopContinue = l.LoopContinue[:len(l.LoopContinue)-1]
	}
}

func (l *LocalContext) CurrentLoopBreakTarget() *ir.Block {
	if len(l.LoopBreak) == 0 {
		return nil
	}
	return l.LoopBreak[len(l.LoopBreak)-1]
}

func (l *LocalContext) CurrentLoopContinueTarget() *ir.Block {
	if len(l.LoopContinue) == 0 {
		return nil
	}
	return l.LoopContinue[len(l.LoopContinue)-1]
}
