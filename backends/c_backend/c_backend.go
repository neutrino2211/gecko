package cbackend

import (
	"fmt"
	"strings"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/tokens"
)

func (impls *CBackendImplementation) NewReturn(scope *ast.Ast) {
	info := GetCScope(scope)

	info.text += "return;"
}

func (impl *CBackendImplementation) NewReturnLiteral(scope *ast.Ast, literal *tokens.Expression) {
	info := GetCScope(scope)

	val := impl.ExpressionToCString(literal, scope, &tokens.TypeRef{})

	info.text += "return " + val + ";"
}

func (*CBackendImplementation) Declaration(scope *ast.Ast, decl *tokens.Declaration) {

}

func (*CBackendImplementation) ParseExpression(scope *ast.Ast, exp *tokens.Expression) {

}

func (impls *CBackendImplementation) ProcessEntries(scope *ast.Ast, entries []*tokens.Entry) {
	impls.Backend.ProcessEntries(entries, scope)
}

func (impls *CBackendImplementation) NewDeclaration(scope *ast.Ast, decl *tokens.Declaration) {
	if decl.Field != nil {
		impls.NewVariable(scope, decl.Field)
	} else if decl.Method != nil {
		impls.NewMethod(scope, decl.Method)
	}
}

func (impls *CBackendImplementation) NewClass(scope *ast.Ast, c *tokens.Class) {
	classAst := &ast.Ast{
		Scope:  c.Name,
		Parent: scope,
	}

	classAst.Init(scope.ErrorScope)
	scope.Classes[c.Name] = classAst

	for _, f := range c.Fields {
		if f.Method != nil {
			impls.NewMethod(classAst, f.Method)
		}

		if f.Field != nil {
			impls.NewVariable(classAst, f.Field)
		}
	}
}

func (impl *CBackendImplementation) NewImplementation(scope *ast.Ast, i *tokens.Implementation) {
	// TODO:

	if i.For != "" {
		// impl.LLVMImplementationForClass(scope, i)
	} else {
		// impl.LLVMImplementationForArch(scope, i)
	}
}

func (impl *CBackendImplementation) NewTrait(scope *ast.Ast, t *tokens.Trait) {

}

func (impls *CBackendImplementation) FuncCall(scope *ast.Ast, f *tokens.FuncCall) {
	mth := scope.ResolveMethod(f.Function)

	if mth.IsNil() {
		scope.ErrorScope.NewCompileTimeError("Function resolution error", fmt.Sprintf("Unable to resolve the function \"%s\"", f.Function), f.Pos)
		return
	}

	mthUnwrapped := mth.Unwrap()

	var info *CFileScope = nil

	if mthUnwrapped.Scope != nil {
		info = GetCScope(mthUnwrapped.Scope)
	} else {
		info = GetCScope(scope)
	}

	args := make([]string, 0)

	info.text += mthUnwrapped.GetFullName() + "("

	for _, a := range f.Arguments {
		tr := &tokens.TypeRef{}

		args = append(args, impls.ExpressionToCString(a.Value, scope, tr))
	}

	info.text += strings.Join(args, ", ")

	info.text += ");\n"

	FuncCalls[scope.FullScopeName()+"#"+f.Function] = mthUnwrapped.GetFullName()
}

func (impl *CBackendImplementation) NewMethod(scope *ast.Ast, m *tokens.Method) {
	methodScope := ast.Ast{
		Scope:  m.Name,
		Parent: scope,
	}
	methodScope.Init(scope.ErrorScope)

	info := GetCScope(scope)

	returnType := "void"

	if m.Type != nil {
		m.Type.Check(scope)

		returnType = m.Type.ToCString(scope)
	}

	info.text += returnType + " "

	if m.Visibility == "external" || methodScope.GetFullName() == "main__main" {
		info.text += m.Name
	} else {
		info.text += methodScope.GetFullName()
	}

	info.text += "("

	for _, a := range m.Arguments {
		info.text += a.Type.ToCString(scope) + " " + a.Name + ","
	}

	if m.Variardic && len(m.Value) != 0 {
		scope.ErrorScope.NewCompileTimeError("E_VARIARDIC", "Gecko functions can not be variardic", m.Pos)
	}

	if m.Variardic {
		info.text += "...,"
	}

	if len(m.Arguments) > 0 {
		info.text = info.text[:len(info.text)-1]
	}

	info.text += ")"

	mthInfo := GetCScope(&methodScope)

	astMth := &ast.Method{
		Name:       m.Name,
		Scope:      map[bool]*ast.Ast{true: nil, false: &methodScope}[len(m.Value) == 0],
		Arguments:  make([]ast.Variable, 0),
		Visibility: m.Visibility,
		Parent:     scope,
		Type:       returnType,
	}
	scope.Methods[m.Name] = astMth

	(*ScopeData)[astMth.GetFullName()] = mthInfo

	// Add arguments as variables

	for _, v := range m.Arguments {
		mVariable := ast.Variable{
			IsPointer:  v.Type.Pointer,
			IsConst:    v.Type.Const,
			IsExternal: false,
			IsArgument: true,
			Name:       v.Name,
			Parent:     &methodScope,
		}

		methodScope.Variables[v.Name] = mVariable

		(*ProgramValues)[mVariable.GetFullName()] = &CValueInformation{
			IsConst:   v.Type.Const,
			Value:     v.Name,
			GeckoType: v.Type,
		}
	}

	// If no return is specified, inject a void return
	if len(m.Value) > 0 && m.Value[len(m.Value)-1].Return == nil && m.Value[len(m.Value)-1].VoidReturn == nil {
		t := true
		m.Value = append(m.Value, &tokens.Entry{VoidReturn: &t})
	}

	impl.Backend.ProcessEntries(m.Value, &methodScope)

	if mthInfo.text != "" {
		info.text += "{\n"

		info.text += mthInfo.text

		info.text += "\n}\n"
	} else {
		info.text += ";\n"
	}

	// if len(m.Value) > 0 {
	// 	methodScope.LocalContext.MainBlock.NewRet(constant.NewInt(types.I1, 0))
	// }
	Methods[scope.FullScopeName()+"#"+m.Name] = astMth
}

func (impl *CBackendImplementation) NewVariable(scope *ast.Ast, f *tokens.Field) {
	if f.Type == nil {
		scope.ErrorScope.NewCompileTimeError("TODO: Infer variable type", "variable type inference not implemented", f.Pos)
		f.Type = &tokens.TypeRef{}
	}

	if f.Value == nil && f.Type.Const {
		scope.ErrorScope.NewCompileTimeError("Uninitialesed constant", "constant must be initialised with a value", f.Pos)
		f.Value = &tokens.Expression{}
	}

	info := GetCScope(scope)

	if f.Type.Const {
		info.text += "const"
	} else {
		info.text += f.Type.ToCString(scope)
	}

	fieldVariable := ast.Variable{
		Name:       f.Name,
		IsConst:    f.Type.Const,
		IsPointer:  f.Type.Pointer,
		IsExternal: false,
		Parent:     scope,
	}

	if f.Visibility == "external" {
		fieldVariable.IsExternal = true
	}

	info.text += " " + fieldVariable.GetFullName()

	val := impl.ExpressionToCString(f.Value, scope, f.Type)

	info.text += " = " + val + ";\n"

	(*ProgramValues)[fieldVariable.GetFullName()] = &CValueInformation{
		IsConst:   f.Type.Const,
		GeckoType: f.Type,
		Value:     val,
	}

	scope.Variables[f.Name] = fieldVariable
}

func GetCScope(scope *ast.Ast) *CFileScope {
	name := scope.GetFullName()

	s, ok := (*ScopeData)[name]

	if !ok {
		s = &CFileScope{}
		s.Init(name)
		(*ScopeData)[name] = s
		return s
	}

	return s
}
