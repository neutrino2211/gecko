package tokens

import (
	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/errors"
)

func assignEntriesToAst(entries []*Entry, scope *ast.Ast) {
	for _, entry := range entries {
		if entry.Method != nil {
			scope.Methods[entry.Method.Name] = *entry.Method.ToAstMethod(scope)
		} else if entry.Field != nil {
			scope.Variables[entry.Field.Name] = *entry.Field.ToAstVariable(scope)
		} else if entry.Declaration != nil {
			if entry.Declaration.Field != nil {
				scope.Variables[entry.Declaration.Field.Name] = *entry.Declaration.ToAstVariable(scope)
			} else if entry.Declaration.Method != nil {
				scope.Methods[entry.Declaration.Method.Name] = *entry.Declaration.ToAstMethod(scope)
			}
		}
	}
}

func assignArgumentsToMethodArguments(args []*Value, mth *ast.Method) {
	for _, v := range args {
		mth.Arguments = append(mth.Arguments, ast.MethodArgument{
			Name:      v.Name,
			Type:      v.Type.ToCString(mth.Parent),
			IsPointer: v.Type.Pointer,
		})
	}
}

func (f *File) ToAst() *ast.Ast {
	file := &ast.Ast{
		Scope:      f.PackageName,
		Methods:    make(map[string]ast.Method),
		Variables:  make(map[string]ast.Variable),
		Imports:    []string{},
		ErrorScope: errors.NewErrorScope(f.Name, f.Path, f.Content),
	}

	assignEntriesToAst(f.Entries, file)

	return file
}

func (m *Method) ToAstMethod(scope *ast.Ast) *ast.Method {
	methodScope := ast.Ast{
		Scope:      m.Name,
		Parent:     scope,
		Methods:    make(map[string]ast.Method),
		Variables:  make(map[string]ast.Variable),
		Imports:    []string{},
		ErrorScope: scope.ErrorScope,
	}

	returnType := "void"

	if m.Type != nil {
		returnType = m.Type.ToCString(scope)
	}

	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      map[bool]*ast.Ast{true: nil, false: &methodScope}[len(m.Value) == 0],
		Arguments:  make([]ast.MethodArgument, 0),
		Visibility: m.Visibility,
		Parent:     scope,
		Type:       returnType,
	}

	assignEntriesToAst(m.Value, &methodScope)

	assignArgumentsToMethodArguments(m.Arguments, astMth)

	return astMth
}
