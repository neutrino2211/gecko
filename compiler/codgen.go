package compiler

import (
	"github.com/llir/llvm/ir/value"
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/config"
	"github.com/neutrino2211/gecko/errors"
	"github.com/neutrino2211/gecko/tokens"
)

func assignEntriesToAst(entries []*tokens.Entry, scope *ast.Ast) {
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
			Type:      v.Type.GetLLIRType(mth.Scope),
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

	return astMth
}
