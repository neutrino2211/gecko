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
	Assignment     *Assignment     `parser:"| @@"`
	ElseIf         *ElseIf         `parser:"| @@"`
	Else           *Else           `parser:"| @@"`
	If             *If             `parser:"| @@"`
	FuncCall       *FuncCall       `parser:"| @@"`
	Method         *Method         `parser:"| @@"`
	Class          *Class          `parser:"| @@"`
	Implementation *Implementation `parser:"| @@"`
	Trait          *Trait          `parser:"| @@"`
	Enum           *Enum           `parser:"| @@"`
	Field          *Field          `parser:"| @@"`
	Loop           *Loop           `parser:"| @@"`
	CImport        *CImport        `parser:"| @@"`
	Declaration    *Declaration    `parser:"| @@"`
	Import         *Import         `parser:"| @@"`
}

// Class tokens

type Class struct {
	baseToken
	Visibility      string             `parser:"[ @'private' | @'public' | @'protected' ]"`
	ExternalName    string             `parser:"[ 'external' @String ]"`
	Name            string             `parser:"'class' @Ident"`
	Fields          []*ClassBlockField `parser:"'{' { @@ } '}'"`
	Implementations []*Implementation
}

type ClassBlockField struct {
	baseToken
	Method *Method `parser:"@@"`
	Field  *Field  `parser:"| @@"`
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
	Expression *Expression `parser:"'if' '(' @@ ')'"`
	Value      []*Entry    `parser:"'{' { @@ } '}'"`
	ElseIf     *ElseIf     `parser:"[ @@ "`
	Else       *Else       `parser:"| @@ ]"`
}

type ElseIf struct {
	baseToken
	Expression *Expression `parser:"'else' 'if' '(' @@ ')'"`
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
	Equality *Equality `parser:"@@"`
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
	Op       string      `parser:"[ @( '>' | '>' '=' | '<' | '<' '=' )"`
	Next     *Comparison `parser:"  @@ ]"`
}

type Addition struct {
	baseToken
	Multiplication *Multiplication `parser:"@@"`
	Op             string          `parser:"[ @( '-' | '+' | '|' | '&' | '>' '>' '>' | '<' '<' '<')"`
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
	Name   string                 `parser:"'trait' @Ident"`
	Type   *TypeRef               `parser:"'<' @@ '>'"`
	Fields []*ImplementationField `parser:"'{' { @@ } '}'"`
}

type Implementation struct {
	baseToken
	Visibility string                 `parser:"[ @'private' | @'public' | @'protected' ]"`
	Default    bool                   `parser:"[ @'default' ]"`
	Name       string                 `parser:"'impl' @Ident"`
	For        string                 `parser:"['for' @Ident]"`
	Fields     []*ImplementationField `parser:"[ '{' { @@ } '}' ]"`
}

type Field struct {
	baseToken
	Visibility string      `parser:"[ @'private' | @'public' | @'protected' | @'external' ]"`
	Mutability string      `parser:"@( 'const' | 'let' )"`
	Name       string      `parser:"@Ident"`
	Type       *TypeRef    `parser:"[ ':' @@ ]"`
	Value      *Expression `parser:"[ '=' @@ ]"`
}

type Assignment struct {
	baseToken
	Name  string      `parser:"@Ident"`
	Value *Expression `parser:"'=' @@"`
}

type Declaration struct {
	baseToken
	Method *Method `parser:"( 'declare' @@ "`
	Field  *Field  `parser:"| 'declare' @@)"`
}

type ImplementationField struct {
	baseToken
	Name      string   `parser:"'func' @Ident"`
	Arguments []*Value `parser:"[ '(' [ @@ { ',' @@ } ] ')' ]"`
	Type      *TypeRef `parser:"':' @@"`
	Value     []*Entry `parser:"[ '{' @@* '}' ]"`
}

type Method struct {
	baseToken
	Visibility string   `parser:"[ @'private' | @'public' | @'protected' | @'external' ]"`
	Variardic  bool     `parser:"[ @'variardic' ]"`
	Name       string   `parser:"'func' @Ident"`
	Arguments  []*Value `parser:"'(' [ @@ { ',' @@ } ] ')'"`
	Type       *TypeRef `parser:"[ ':' @@ ]"`
	Value      []*Entry `parser:"[ '{' @@* '}' ]"`
}

type Value struct {
	baseToken
	Variadic bool        `parser:"[ @'...' ]"`
	Name     string      `parser:"@Ident"`
	Type     *TypeRef    `parser:"':' @@"`
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
	Array   *TypeRef `parser:"( '[' @@ ']'"`
	Size    *SizeDef `parser:" | @@"`
	Type    string   `parser:" | @Ident)"`
	Trait   string   `parser:"[ 'is' @Ident ]"`
	Const   bool     `parser:"[ @'!' ]"`
	Pointer bool     `parser:"[ @'*']"`
}

type Literal struct {
	baseToken
	IsPointer  bool              `parser:"[ @'&' ]"`
	FuncCall   *FuncCall         `parser:"( @@"`
	Bool       string            `parser:" | @( 'true' | 'false' )"`
	String     string            `parser:" | @String"`
	Symbol     string            `parser:" | @Ident"`
	Number     string            `parser:" | @Number"`
	Object     []*ObjectKeyValue `parser:" | '{' [ @@ { ',' @@ } ] '}'"`
	Array      []*Literal        `parser:" | '[' [ @@ { ',' @@ } ] ']' )"`
	ArrayIndex *Literal          `parser:"[ '[' @@ ']' ]"`
}

type FuncCall struct {
	baseToken
	Function  string      `parser:"@Ident"`
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
	For           string      `parser:"'for'"`
	ForOf         *ForOfLoop  `parser:"( @@"`
	ForIn         *ForInLoop  `parser:" | @@"`
	ForExpression *Expression `parser:" | @@ )"`
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
