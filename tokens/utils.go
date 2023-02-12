package tokens

import (
	"github.com/alecthomas/repr"
	"github.com/neutrino2211/gecko/ast"
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
			entry.Implementation.ForClass(scope)
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
			repr.Println(entry.FuncCall)
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

func (f *File) ToAst() *ast.Ast {
	file := &ast.Ast{
		Scope: f.PackageName,
	}

	file.Init(errors.NewErrorScope(f.Name, f.Path, f.Content))

	assignEntriesToAst(f.Entries, file)

	return file
}

func (m *Method) ToAstMethod(scope *ast.Ast) *ast.Method {
	methodScope := ast.Ast{
		Scope:  m.Name,
		Parent: scope,
	}

	methodScope.Init(scope.ErrorScope)

	returnType := "void"

	if m.Type != nil {
		m.Type.Check(scope)

		returnType = m.Type.ToCString(scope)
	}

	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      map[bool]*ast.Ast{true: nil, false: &methodScope}[len(m.Value) == 0],
		Arguments:  make([]ast.Variable, 0),
		Visibility: m.Visibility,
		Parent:     scope,
		Type:       returnType,
	}

	assignEntriesToAst(m.Value, &methodScope)

	assignArgumentsToMethodArguments(m.Arguments, astMth)

	return astMth
}
