// Package tokens contains the definitions for all gecko symbols/tokens
package tokens

import (
	"github.com/alecthomas/participle/lexer"
	"github.com/neutrino2211/gecko/config"
)

type baseToken struct {
	Pos   lexer.Position
	RefID string
}

// Attribute represents a compile-time attribute like @packed or @section(".text")
type Attribute struct {
	baseToken
	Name  string `parser:"'@' @Ident"`
	Value string `parser:"[ '(' @String ')' ]"`
}

// File tokens

type File struct {
	PackageName string   `parser:"['package' @Ident]"`
	Entries     []*Entry `parser:"@@*"`
	Imports     []*File
	Config      *config.CompileCfg
	Name        string
	Path        string
	Content     string
}

type CImport struct {
	baseToken
	Header      string `parser:"'cimport' @String"`
	WithObject  string `parser:"[ 'withobject' @String"`
	WithLibrary string `parser:"| 'withlibrary' @String ]"`
}

type Import struct {
	baseToken
	Package string   `parser:"'import' @Ident"`
	Objects []string `parser:"['use' '{' [ @Ident { ',' @Ident } ] '}']"`
}

type Entry struct {
	baseToken
	Return         *Expression     `parser:"'return' @@"`
	VoidReturn     *bool           `parser:"| @'return'"`
	Break          *bool           `parser:"| @'break'"`
	Continue       *bool           `parser:"| @'continue'"`
	Assignment     *Assignment     `parser:"| @@"`
	ElseIf         *ElseIf         `parser:"| @@"`
	Else           *Else           `parser:"| @@"`
	If             *If             `parser:"| @@"`
	Intrinsic      *Intrinsic      `parser:"| @@"`
	FuncCall       *FuncCall       `parser:"| @@"`
	Class          *Class          `parser:"| @@"`
	Trait          *Trait          `parser:"| @@"`
	Field          *Field          `parser:"| @@"`
	Method         *Method         `parser:"| @@"`
	Implementation *Implementation `parser:"| @@"`
	Enum           *Enum           `parser:"| @@"`
	Loop           *Loop           `parser:"| @@"`
	CImport        *CImport        `parser:"| @@"`
	Declaration    *Declaration    `parser:"| @@"`
	Import         *Import         `parser:"| @@"`
	Asm            *Asm            `parser:"| @@"`
}

// Generic type parameters

type TypeParam struct {
	baseToken
	Name  string `parser:"@Ident"`
	Trait string `parser:"[ 'is' @Ident ]"`
}

// Class tokens

type Class struct {
	baseToken
	DocComment      []string           `parser:"{ @DocComment }"`
	Attributes      []*Attribute       `parser:"{ @@ }"`
	Visibility      string             `parser:"[ @'private' | @'public' | @'protected' ]"`
	ExternalName    string             `parser:"[ 'external' @String ]"`
	Name            string             `parser:"'class' @Ident"`
	TypeParams      []*TypeParam       `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Fields          []*ClassBlockField `parser:"'{' { @@ } '}'"`
	Implementations []*Implementation
}

type ClassBlockField struct {
	baseToken
	Field  *Field  `parser:"@@"`
	Method *Method `parser:"| @@"`
}

type ClassField struct {
	baseToken
	Field
}

type ClassMethod struct {
	baseToken
	Method
}

// Conditionals

type If struct {
	baseToken
	Expression *Expression `parser:"'if' @@"`
	Value      []*Entry    `parser:"'{' { @@ } '}'"`
	ElseIf     *ElseIf     `parser:"[ @@ "`
	Else       *Else       `parser:"| @@ ]"`
}

type ElseIf struct {
	baseToken
	Expression *Expression `parser:"'else' 'if' @@"`
	Value      []*Entry    `parser:"'{' { @@ } '}'"`
	ElseIf     *ElseIf     `parser:"[ @@ "`
	Else       *Else       `parser:"| @@ ]"`
}

type Else struct {
	baseToken
	Value []*Entry `parser:"'else' '{' { @@ } '}'"`
}

// Expressions

type Expression struct {
	baseToken
	LogicalOr *LogicalOr `parser:"@@"`
}

type LogicalOr struct {
	baseToken
	LogicalAnd *LogicalAnd `parser:"@@"`
	Op         string      `parser:"[ @LogicalOr"`
	Next       *LogicalOr  `parser:"  @@ ]"`
}

type LogicalAnd struct {
	baseToken
	Equality *Equality   `parser:"@@"`
	Op       string      `parser:"[ @LogicalAnd"`
	Next     *LogicalAnd `parser:"  @@ ]"`
}

type Equality struct {
	baseToken
	Comparison *Comparison `parser:"@@"`
	Op         string      `parser:"[ @( '!' '=' | '=' '=' )"`
	Next       *Equality   `parser:"  @@ ]"`
}

type Comparison struct {
	baseToken
	Addition *Addition   `parser:"@@"`
	Op       string      `parser:"[ @( '>' '=' | '<' '=' | '>' | '<' )"`
	Next     *Comparison `parser:"  @@ ]"`
}

type Addition struct {
	baseToken
	Multiplication *Multiplication `parser:"@@"`
	Op             string          `parser:"[ @( '-' | '+' | '|' | '&' | '^' | '>' '>' '>' | '<' '<' '<' | '>' '>' | '<' '<')"`
	Next           *Addition       `parser:"  @@ ]"`
}

type Multiplication struct {
	baseToken
	Unary *Unary          `parser:"@@"`
	Op    string          `parser:"[ @( '/' | '*' )"`
	Next  *Multiplication `parser:"  @@ ]"`
}

type Unary struct {
	baseToken
	Op      string   `parser:"  ( @( '!' | '-' | '+' )"`
	Unary   *Unary   `parser:"    @@ )"`
	Primary *Primary `parser:"| @@"`
	Cast    *Cast    `parser:"[ @@ ]"`
}

// Cast represents a type cast expression using the 'as' keyword
// Example: 0xB8000 as *uint16, or ptr as uint64
type Cast struct {
	baseToken
	Type *TypeRef `parser:"'as' @@"`
}

type Primary struct {
	baseToken
	Literal *Literal `parser:"@@"`
	// IsPointer     *bool       `parser:"[ '&' ]"`
	// FuncCall      *FuncCall   `parser:"( @@"`
	// Bool          string      `parser:" | ( @'true' | @'false' )"`
	// Nil           *bool       `parser:" | @'nil'"`
	// String        string      `parser:" | @String"`
	// Symbol        string      `parser:" | @Ident"`
	// Number        string      `parser:" | @Number"`
	SubExpression *Expression `parser:" | '(' @@ ')'"`
}

// Misc TODO: Sort

type Enum struct {
	baseToken
	Name  string   `parser:"'enum' @Ident"`
	Cases []string `parser:"'{' { @Ident } '}'"`
}

type Trait struct {
	baseToken
	DocComment []string               `parser:"{ @DocComment }"`
	Name       string                 `parser:"'trait' @Ident"`
	TypeParams []*TypeParam           `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Fields     []*ImplementationField `parser:"'{' { @@ } '}'"`
}

type Implementation struct {
	baseToken
	Visibility string                 `parser:"[ @'private' | @'public' | @'protected' ]"`
	Default    bool                   `parser:"[ @'default' ]"`
	Name       string                 `parser:"'impl' @Ident"`
	TypeArgs   []*TypeRef             `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	For        string                 `parser:"['for' @Ident]"`
	Fields     []*ImplementationField `parser:"[ '{' { @@ } '}' ]"`
}

type Field struct {
	baseToken
	DocComment []string     `parser:"{ @DocComment }"`
	Attributes []*Attribute `parser:"{ @@ }"`
	Visibility string       `parser:"[ @'private' | @'public' | @'protected' | @'external' ]"`
	Mutability string       `parser:"(@'let' | @'const')"`
	Name       string       `parser:"@Ident"`
	Type       *TypeRef     `parser:"[ ':' @@ ]"`
	Value      *Expression  `parser:"[ '=' @@ ]"`
}

type Assignment struct {
	baseToken
	Name  string      `parser:"@Ident"`
	Field string      `parser:"[ '.' @Ident ]"`
	Index *Expression `parser:"[ '[' @@ ']' ]"`
	Value *Expression `parser:"'=' @@"`
}

type Declaration struct {
	baseToken
	Method       *Method       `parser:"( 'declare' @@ "`
	Field        *Field        `parser:"| 'declare' @@"`
	ExternalType *ExternalType `parser:"| 'declare' @@)"`
}

// ExternalType declares an opaque type that exists in C land
// Example: declare external type FILE
type ExternalType struct {
	baseToken
	Name string `parser:"'external' 'type' @Ident"`
}

type ImplementationField struct {
	baseToken
	Name      string   `parser:"'func' @Ident"`
	Arguments []*Value `parser:"[ '(' [ @@ { ',' @@ } ] ')' ]"`
	Type      *TypeRef `parser:"[ ':' @@ ]"`
	Value     []*Entry `parser:"[ '{' @@* '}' ]"`
}

type Method struct {
	baseToken
	DocComment []string     `parser:"{ @DocComment }"`
	Attributes []*Attribute `parser:"{ @@ }"`
	Visibility string       `parser:"[ @'private' | @'public' | @'protected' | @'external' ]"`
	Variardic  bool         `parser:"[ @'variardic' ]"`
	Name       string       `parser:"'func' @Ident"`
	TypeParams []*TypeParam `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Arguments  []*Value     `parser:"'(' [ @@ { ',' @@ } ] ')'"`
	Type       *TypeRef     `parser:"[ ':' @@ ]"`
	Value      []*Entry     `parser:"[ '{' @@* '}' ]"`
}

type Value struct {
	baseToken
	Variadic bool        `parser:"[ @'...' ]"`
	Name     string      `parser:"@Ident"`
	Type     *TypeRef    `parser:"[ ':' @@ ]"`
	Default  *Expression `parser:"[ '=' @@ ]"`
}

type Argument struct {
	baseToken
	Name    string      `parser:"[ @Ident ':' ]"`
	Value   *Expression `parser:"( @@"`
	SubCall *FuncCall   `parser:"| @@)"`
}

type SizeDef struct {
	baseToken
	Size string   `parser:"'[' @Number ']'"`
	Type *TypeRef `parser:"@@"`
}

type TypeRef struct {
	baseToken
	Array    *TypeRef   `parser:"( '[' @@ ']'"`
	Size     *SizeDef   `parser:" | @@"`
	FuncType *FuncType  `parser:" | @@"`
	Type     string     `parser:" | @Ident)"`
	TypeArgs []*TypeRef `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Trait    string     `parser:"[ 'is' @Ident ]"`
	Const    bool       `parser:"[ @'!' ]"`
	Volatile bool       `parser:"[ @'volatile' ]"`
	Pointer  bool       `parser:"[ @'*']"`
	NonNull  bool       `parser:"[ @'!' ]"` // T*! = non-nullable pointer
}

type FuncType struct {
	baseToken
	ParamTypes []*TypeRef `parser:"'func' '(' [ @@ { ',' @@ } ] ')'"`
	ReturnType *TypeRef   `parser:"[ ':' @@ ]"`
}

type Literal struct {
	baseToken
	IsPointer    bool              `parser:"[ @'&' ]"`
	Intrinsic    *Intrinsic        `parser:"( @@"`
	FuncCall     *FuncCall         `parser:" | @@"`
	Bool         string            `parser:" | @( 'true' | 'false' )"`
	String       string            `parser:" | @String"`
	StructType   string            `parser:" | ( @Ident"`
	StructFields []*ObjectKeyValue `parser:"     '{' [ @@ { ',' @@ } ] '}' )"`
	SymbolModule string            `parser:" | ( @Ident '.'"`
	Symbol       string            `parser:"     @Ident ) | @Ident"`
	Number       string            `parser:" | @Number"`
	Object       []*ObjectKeyValue `parser:" | '{' [ @@ { ',' @@ } ] '}'"`
	Array        []*Literal        `parser:" | '[' [ @@ { ',' @@ } ] ']' )"`
	ArrayIndex   *Literal          `parser:"[ '[' @@ ']' ]"`
	Chain        []*ChainAccess    `parser:"{ @@ }"`
}

// ChainAccess represents a chained field or method access: .field or .method()
type ChainAccess struct {
	baseToken
	Name       string      `parser:"'.' @Ident"`
	TypeArgs   []*TypeRef  `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	HasParens  bool        `parser:"[ @'('"`
	Args       []*Argument `parser:"  [ @@ { ',' @@ } ] ')' ]"`
}

// IsMethodCall returns true if this chain access is a method call (has parentheses)
func (c *ChainAccess) IsMethodCall() bool {
	return c.HasParens
}

// Intrinsic represents a compiler intrinsic call: @name(args) or @name<T>(args)
type Intrinsic struct {
	baseToken
	Name     string        `parser:"'@' @Ident"`
	TypeArgs []*TypeRef    `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Args     []*Expression `parser:"'(' [ @@ { ',' @@ } ] ')'"`
}

type FuncCall struct {
	baseToken
	// Static type call: Type<Args>::function() or Type::function()
	StaticType     string     `parser:"[ @Ident"`
	StaticTypeArgs []*TypeRef `parser:"  [ '<' @@ { ',' @@ } '>' ] '::' ]"`
	// Module/instance call: module.function()
	Module    string      `parser:"[ @Ident '.' ]"`
	Function  string      `parser:"@Ident"`
	TypeArgs  []*TypeRef  `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Arguments []*Argument `parser:"'(' [ @@ { ',' @@ } ] ')'"`
}

type Object struct {
	baseToken
}

type ObjectKeyValue struct {
	baseToken
	Key   string      `parser:"@Ident ':'"`
	Value *Expression `parser:"@@"`
}

type Loop struct {
	baseToken
	For           string      `parser:"( 'for'"`
	ForOf         *ForOfLoop  `parser:"  ( @@"`
	ForIn         *ForInLoop  `parser:"   | @@"`
	ForExpression *Expression `parser:"   | @@ )"`
	While         string      `parser:"| 'while'"`
	WhileExpr     *Expression `parser:"  @@ )"`
	Value         []*Entry    `parser:" '{' @@* '}' "`
}

type ForOfLoop struct {
	baseToken
	Variable    *Field      `parser:"@@ 'of'"`
	SourceArray *Expression `parser:"@@"`
}

type ForInLoop struct {
	baseToken
	Variable    *Field      `parser:"@@ 'in'"`
	SourceArray *Expression `parser:"@@"`
}

// Asm represents inline assembly code
type Asm struct {
	baseToken
	Code string `parser:"'asm' '{' @String '}'"`
}

// IsStructLiteral returns true if this Literal is a struct literal (TypeName { fields })
func (l *Literal) IsStructLiteral() bool {
	return l.StructType != "" && l.StructFields != nil
}
