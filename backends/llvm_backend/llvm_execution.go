package llvmbackend

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/go-option"
)

type ModuleContext struct {
	Module            *ir.Module
	GlobalDefinitions map[string]*ir.Global
	Methods           map[string]*ir.Func
}

type LocalContext struct {
	ModuleContext *ModuleContext
	Func          *ir.Func
	MainBlock     *ir.Block
	Branches      map[string]*ir.Block
	Types         map[string]*types.Type
}

type ExecutionContext struct {
	Context *ModuleContext
	Steps   []*Step
}

func (m *ModuleContext) Init() {
	m.Module = ir.NewModule()
	m.GlobalDefinitions = make(map[string]*ir.Global)
	m.Methods = make(map[string]*ir.Func)
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
