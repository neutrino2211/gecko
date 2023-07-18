package ast

import (
	"github.com/llir/llvm/ir/types"
	"github.com/neutrino2211/gecko/codegen"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/go-option"
)

func loadPrimitives(ast *Ast) {
	for _, p := range Primitives {
		ast.Classes[p.Class.Scope] = p.Class

		if ast.LocalContext != nil { // In a function, provide LLIR type context
			ast.LocalContext.Types[p.Class.FullScopeName()] = &p.Type
		}
	}
}

type Ast struct {
	Scope            string
	Imports          []string
	Methods          map[string]*Method
	Variables        map[string]Variable
	Classes          map[string]*Ast
	Traits           map[string]*[]*Method
	Parent           *Ast
	ErrorScope       *errors.ErrorScope
	Config           *config.CompileCfg
	ExecutionContext *codegen.ExecutionContext
	ProgramContext   *codegen.ModuleContext
	LocalContext     *codegen.LocalContext
	ChildContexts    map[string]*codegen.LocalContext
}

func (a *Ast) Init(errorScope *errors.ErrorScope, executionContext *codegen.ExecutionContext) {
	a.Methods = make(map[string]*Method)
	a.Variables = make(map[string]Variable)
	a.Classes = make(map[string]*Ast)
	a.Traits = make(map[string]*[]*Method)
	a.Imports = []string{}
	a.ErrorScope = errorScope
	a.Config = &config.CompileCfg{}

	a.ExecutionContext = executionContext
	a.ProgramContext = executionContext.Context
	a.LocalContext = nil
	a.ChildContexts = make(map[string]*codegen.LocalContext)

	loadPrimitives(a)
}

func (a *Ast) FullScopeName() string {
	r := a.Scope
	parent := a.Parent

	for parent != nil {
		r = parent.Scope + "." + r
		parent = parent.Parent
	}

	return r
}

func (a *Ast) ResolveSymbolAsVariable(symbol string) *option.Optional[*Variable] {
	scope := a
	symbolVariable, ok := scope.Variables[symbol]

	for !ok {
		if scope.Parent == nil {
			return option.None[*Variable]()
		}

		scope = scope.Parent
		symbolVariable, ok = scope.Variables[symbol]
	}

	return option.Some(&symbolVariable)
}

func (a *Ast) ResolveMethod(mth string) *option.Optional[*Method] {
	scope := a
	mthMethod, ok := scope.Methods[mth]

	for !ok {
		if scope.Parent == nil {
			return option.None[*Method]()
		}

		scope = scope.Parent
		mthMethod, ok = scope.Methods[mth]
	}

	return option.Some(mthMethod)
}

func (a *Ast) ResolveClass(class string) *option.Optional[*Ast] {
	scope := a
	clsClass, ok := scope.Classes[class]

	for !ok {
		if scope.Parent == nil {
			return option.None[*Ast]()
		}

		scope = scope.Parent
		clsClass, ok = scope.Classes[class]
	}

	return option.Some(clsClass)
}

func (a *Ast) ResolveTrait(trait string) *option.Optional[*[]*Method] {
	scope := a
	trTrait, ok := scope.Traits[trait]

	for !ok {
		if scope.Parent == nil {
			return option.None[*[]*Method]()
		}

		scope = scope.Parent
		trTrait, ok = scope.Traits[trait]
	}

	return option.Some(trTrait)
}

func (a *Ast) ResolveFuncContext(funcName string) *option.Optional[*codegen.LocalContext] {
	scope := a
	fnCtx, ok := scope.ChildContexts[funcName]

	for !ok {
		if scope.Parent == nil {
			return option.None[*codegen.LocalContext]()
		}

		scope = scope.Parent
		fnCtx, ok = scope.ChildContexts[funcName]
	}

	return option.Some(fnCtx)
}

func (a *Ast) ResolveLLIRType(typ string) *option.Optional[*types.Type] {
	t, ok := a.LocalContext.Types[typ]

	if !ok {
		return option.None[*types.Type]()
	}

	return option.Some(t)
}

func (a *Ast) ToCString() string {
	r := ""
	// solve
	return r
}
