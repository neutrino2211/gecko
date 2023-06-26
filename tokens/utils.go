package tokens

import (
	"github.com/llir/llvm/ir"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/codegen"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
)

func assignEntriesToAst(entries []*Entry, scope *ast.Ast) {
	for _, entry := range entries {
		if entry.Method != nil {
			scope.Methods[entry.Method.Name] = *entry.Method.ToAstMethod(scope)
		} else if entry.Field != nil {
			scope.Variables[entry.Field.Name] = *entry.Field.ToAstVariable(scope)
		} else if entry.Class != nil {
			scope.Classes[entry.Class.Name] = entry.Class.ToAst(scope)
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
				scope.Methods[entry.Declaration.Method.Name] = *methodOpt.UnwrapOrElse(func(err error) *ast.Method {
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
		} else if entry.Return != nil {
			returnLiteral(scope, entry.Return)
		}
	}
}

func assignArgumentsToMethodArguments(args []*Value, mth *ast.Method) {
	for _, v := range args {
		def := ""

		if v.Default != nil {
			def = v.Default.ToCString(mth.Parent)
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

	astMth.Context = methodScope.LocalContext

	scope.ChildContexts[astMth.GetFullName()] = methodScope.LocalContext
	scope.ProgramContext.Module.Funcs = append(scope.ProgramContext.Module.Funcs, methodScope.LocalContext.Func)

	assignEntriesToAst(m.Value, &methodScope)

	assignArgumentsToMethodArguments(m.Arguments, astMth)

	// if len(m.Value) > 0 {
	// 	methodScope.LocalContext.MainBlock.NewRet(constant.NewInt(types.I1, 0))
	// }

	return astMth
}
