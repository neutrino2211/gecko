package tokens

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/codegen"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
)

func assignEntriesToAst(entries []*Entry, scope *ast.Ast) {
	for _, entry := range entries {
		if entry.Method != nil {
			entry.Method.ToAstMethod(scope)
		} else if entry.Field != nil {
			scope.Variables[entry.Field.Name] = *entry.Field.ToAstVariable(scope)
		} else if entry.Class != nil {
			entry.Class.ToAst(scope)
		} else if entry.Implementation != nil {
			if entry.Implementation.For != "" {
				entry.Implementation.ForClass(scope)
			} else {
				entry.Implementation.ForArch(scope)
			}
		} else if entry.Trait != nil {
			entry.Trait.AssignToScope(scope)
		} else if entry.Declaration != nil {
			if entry.Declaration.Field != nil {
				variableOpt := entry.Declaration.ToAstVariable(scope)
				scope.Variables[entry.Declaration.Field.Name] = *variableOpt.UnwrapOrElse(func(err error) *ast.Variable {
					scope.ErrorScope.NewCompileTimeError(
						"Parse Error",
						"Unable to parse the variable named '"+entry.Declaration.Field.Name+"'",
						entry.Pos,
					)
					return &ast.Variable{}
				})
			} else if entry.Declaration.Method != nil {
				methodOpt := entry.Declaration.ToAstMethod(scope)
				scope.Methods[entry.Declaration.Method.Name] = methodOpt.UnwrapOrElse(func(err error) *ast.Method {
					scope.ErrorScope.NewCompileTimeError(
						"Parse Error",
						"Unable to parse the method named '"+entry.Declaration.Method.Name+"'",
						entry.Pos,
					)

					return &ast.Method{}
				})
			}
		} else if entry.FuncCall != nil {
			entry.FuncCall.AddToLLIR(scope)
		} else if entry.If != nil {
			entry.If.ToLLIRBlock(scope)
		} else if entry.Return != nil {
			returnLiteral(scope, entry.Return)
		} else if entry.VoidReturn != nil {
			returnVoid(scope)
		}
	}
}

func assignArgumentsToMethodArguments(args []*Value, mth *ast.Method) {
	for _, v := range args {
		var def value.Value = nil

		if v.Default != nil {
			def = v.Default.ToLLIRValue(mth.Parent, v.Type)
		}

		if mth.Scope != nil {
			v.Type.Check(mth.Scope)
		}

		mth.Arguments = append(mth.Arguments, ast.Variable{
			Name:      v.Name,
			Type:      v.Type.ToCString(mth.Parent),
			IsPointer: v.Type.Pointer,
			Value:     def,
		})
	}
}

func (f *File) ToAst(config *config.CompileCfg) *ast.Ast {
	file := &ast.Ast{
		Scope: f.PackageName,
	}

	file.Init(errors.NewErrorScope(f.Name, f.Path, f.Content), codegen.NewExecutionContext())
	file.Config = config

	assignEntriesToAst(f.Entries, file)

	return file
}

func (m *Method) ToAstMethod(scope *ast.Ast) *ast.Method {
	methodScope := ast.Ast{
		Scope:  m.Name,
		Parent: scope,
	}

	fnParams := make([]*ir.Param, 0)

	for _, a := range m.Arguments {
		ty := a.Type.GetLLIRType(scope)

		fnParams = append(fnParams, ir.NewParam(a.Name, ty))
	}

	methodScope.Init(scope.ErrorScope, scope.ExecutionContext)

	returnType := "void"
	irType := ast.VoidType.Type

	if m.Type != nil {
		m.Type.Check(scope)

		returnType = m.Type.ToCString(scope)
		irType = m.Type.GetLLIRType(scope)
	}

	irFunc := ir.NewFunc(m.Name, irType, fnParams...)
	irFunc.CallingConv = codegen.CallingConventions[scope.Config.Arch][scope.Config.Platform]
	if m.Variardic {
		irFunc.Sig.Variadic = true
	}

	methodScope.Config = scope.Config
	methodScope.LocalContext = codegen.NewLocalContext(irFunc)
	methodScope.LoadPrimitives()

	if len(m.Value) > 0 {
		methodScope.LocalContext.MainBlock = methodScope.LocalContext.Func.NewBlock(irFunc.Name() + "$main")
	}

	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      map[bool]*ast.Ast{true: nil, false: &methodScope}[len(m.Value) == 0],
		Arguments:  make([]ast.Variable, 0),
		Visibility: m.Visibility,
		Parent:     scope,
		Type:       returnType,
	}
	scope.Methods[m.Name] = astMth

	astMth.Context = methodScope.LocalContext

	scope.ChildContexts[astMth.GetFullName()] = methodScope.LocalContext
	scope.ProgramContext.Module.Funcs = append(scope.ProgramContext.Module.Funcs, methodScope.LocalContext.Func)

	// Add arguments as variables

	for _, v := range m.Arguments {
		methodScope.Variables[v.Name] = ast.Variable{
			IsPointer:  v.Type.Pointer,
			IsConst:    v.Type.Const,
			IsExternal: false,
			IsArgument: true,
			Name:       v.Name,
			Type:       v.Type.Type,
			Parent:     &methodScope,
			Value:      ir.NewParam(v.Name, v.Type.GetLLIRType(&methodScope)),
		}
	}

	assignEntriesToAst(m.Value, &methodScope)

	assignArgumentsToMethodArguments(m.Arguments, astMth)

	// If no return is specified, inject a void return
	if methodScope.LocalContext.MainBlock != nil && methodScope.LocalContext.MainBlock.Term == nil {
		t := true
		m.Value = append(m.Value, &Entry{VoidReturn: &t})
	}

	// if len(m.Value) > 0 {
	// 	methodScope.LocalContext.MainBlock.NewRet(constant.NewInt(types.I1, 0))
	// }

	return astMth
}

func (t *baseToken) GetID() string {
	s := rand.NewSource(time.Now().UnixNano() + rand.Int63())
	r := rand.New(s)
	if t.RefID == "" {
		i := 0
		for i < 32 {
			t.RefID += strconv.Itoa(r.Intn(9))
			i++
		}
	}

	return t.RefID
}
