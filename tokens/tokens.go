// Package tokens contains the definitions for all gecko symbols/tokens
package tokens

import (
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/config"
)

type baseToken struct {
	Pos    lexer.Position
	EndPos lexer.Position
	RefID  string
}

// Attribute represents a compile-time attribute like @packed, @section(".text"), or @drop_hook(.drop)
type Attribute struct {
	baseToken
	Name string          `parser:"'@' @Ident"`
	Args []*AttributeArg `parser:"[ '(' [ @@ { ',' @@ } ] ')' ]"`
}

// AttributeArg represents an argument to an attribute - either a string or a method reference
type AttributeArg struct {
	baseToken
	String string `parser:"@String"`
	Method string `parser:"| '.' @Ident"`
}

// GetStringValue returns the first string argument value (for backwards compat with @section(".text"))
func (a *Attribute) GetStringValue() string {
	if len(a.Args) > 0 && a.Args[0].String != "" {
		v := a.Args[0].String
		// Remove quotes from the value
		if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
			v = v[1 : len(v)-1]
		}
		return v
	}
	return ""
}

// GetHookMethods returns all method references (for hook attributes like @drop_hook(.drop))
func (a *Attribute) GetHookMethods() []string {
	var methods []string
	for _, arg := range a.Args {
		if arg.Method != "" {
			methods = append(methods, arg.Method)
		}
	}
	return methods
}

// File tokens

// DirectoryImport represents a lazily-resolved directory import
type DirectoryImport struct {
	Path       string   // Full import path (e.g., "std.collections")
	DirPath    string   // Filesystem path to the directory
	UseObjects []string // Specific symbols to import (empty = all public)
}

type File struct {
	Attributes       []*Attribute `parser:"{ @@ }"`
	PackageName      string       `parser:"['package' @Ident]"`
	Entries          []*Entry     `parser:"@@*"`
	Imports          []*File
	DirectoryImports []*DirectoryImport // Lazily resolved directory imports
	Config           *config.CompileCfg
	Name             string
	Path             string
	Content          string
}

// GetBackend returns the backend specified by @backend attribute, or empty string if not specified
func (f *File) GetBackend() string {
	for _, attr := range f.Attributes {
		if attr.Name == "backend" {
			return attr.GetStringValue()
		}
	}
	return ""
}

type CImport struct {
	baseToken
	Header      string `parser:"'cimport' @String"`
	WithObject  string `parser:"[ 'withobject' @String"`
	WithLibrary string `parser:"| 'withlibrary' @String ]"`
}

type Import struct {
	baseToken
	Path    []string `parser:"'import' @Ident { '.' @Ident }"`
	Objects []string `parser:"['use' '{' [ @Ident { ',' @Ident } ] '}']"`
}

// Package returns the full dot-separated import path as a string
func (i *Import) Package() string {
	if len(i.Path) == 0 {
		return ""
	}
	result := i.Path[0]
	for _, p := range i.Path[1:] {
		result += "." + p
	}
	return result
}

// ModuleName returns the last component of the import path (the module name)
func (i *Import) ModuleName() string {
	if len(i.Path) == 0 {
		return ""
	}
	return i.Path[len(i.Path)-1]
}

type Entry struct {
	baseToken
	Return         *Expression     `parser:"'return' @@"`
	VoidReturn     *bool           `parser:"| @'return'"`
	Break          *bool           `parser:"| @'break'"`
	Continue       *bool           `parser:"| @'continue'"`
	// Implementation must come before Assignment to prevent 'impl' being parsed as identifier
	Implementation *Implementation `parser:"| @@"`
	Assignment     *Assignment     `parser:"| @@"`
	ElseIf         *ElseIf         `parser:"| @@"`
	Else           *Else           `parser:"| @@"`
	If             *If             `parser:"| @@"`
	// Declarations with optional attributes must come before Intrinsic
	// so @attr func/class/trait is parsed as declaration, not intrinsic
	Class          *Class          `parser:"| @@"`
	Trait          *Trait          `parser:"| @@"`
	Method         *Method         `parser:"| @@"`
	Field          *Field          `parser:"| @@"`
	Declaration    *Declaration    `parser:"| @@"`
	Enum           *Enum           `parser:"| @@"`
	// Intrinsic, MethodCall, and FuncCall must come after declarations
	// MethodCall handles chained calls like self.field.method()
	Intrinsic      *Intrinsic      `parser:"| @@"`
	MethodCall     *MethodCall     `parser:"| @@"`
	FuncCall       *FuncCall       `parser:"| @@"`
	Loop           *Loop           `parser:"| @@"`
	CImport        *CImport        `parser:"| @@"`
	Import         *Import         `parser:"| @@"`
	Asm            *Asm            `parser:"| @@"`
}

// Generic type parameters

type TypeParam struct {
	baseToken
	Name   string   `parser:"@Ident"`
	Trait  string   `parser:"[ 'is' @Ident"` // First trait (kept for backwards compatibility)
	Traits []string `parser:"    { '&' @Ident } ]"`
}

// AllTraits returns all trait constraints (Trait + Traits)
func (t *TypeParam) AllTraits() []string {
	if t.Trait == "" {
		return nil
	}
	return append([]string{t.Trait}, t.Traits...)
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
	OrExpr *OrExpression `parser:"@@"`
}

// OrExpression handles the 'or' keyword for default values: expr or default
type OrExpression struct {
	baseToken
	LogicalOr *LogicalOr    `parser:"@@"`
	Or        *OrExpression `parser:"[ 'or' @@ ]"`
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
	Op      string   `parser:"  ( @( '!' | '-' | '+' | 'try' )"`
	Unary   *Unary   `parser:"    @@ )"`
	Primary *Primary `parser:"| @@"`
	Cast    *Cast    `parser:"[ @@ ]"`
}

// Cast represents a type cast expression using the 'as' keyword
// Example: 0xB8000 as uint16*, or ptr as uint64
// Use 'as!' for trusted casts that bypass refinement checking
type Cast struct {
	baseToken
	Trusted bool     `parser:"'as' @'!'?"`
	Type    *TypeRef `parser:"@@"`
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
	Attributes []*Attribute           `parser:"{ @@ }"`
	Visibility string                 `parser:"[ @'private' | @'public' | @'protected' ]"`
	Name       string                 `parser:"'trait' @Ident"`
	TypeParams []*TypeParam           `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Fields     []*ImplementationField `parser:"'{' { @@ } '}'"`
}

type Implementation struct {
	baseToken
	Visibility  string                 `parser:"[ @'private' | @'public' | @'protected' ]"`
	Default     bool                   `parser:"[ @'default' ]"`
	Generic     *GenericImpl           `parser:"'impl' ( @@"`
	NonGeneric  *NonGenericImpl        `parser:"       | @@ )"`
}

// GenericImpl handles impl<T> Trait<Args> for Class<Args>
type GenericImpl struct {
	baseToken
	TypeParams  []*TypeParam           `parser:"'<' @@ { ',' @@ } '>'"`
	Name        string                 `parser:"@Ident"`
	TypeArgs    []*TypeRef             `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	For         string                 `parser:"[ 'for' @Ident"`
	ForTypeArgs []*TypeRef             `parser:"  [ '<' @@ { ',' @@ } '>' ] ]"`
	Fields      []*ImplementationField `parser:"[ '{' { @@ } '}' ]"`
}

// NonGenericImpl handles impl Trait<Args> for Class<Args> (no impl-level type params)
type NonGenericImpl struct {
	baseToken
	Name        string                 `parser:"@Ident"`
	TypeArgs    []*TypeRef             `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	For         string                 `parser:"[ 'for' @Ident"`
	ForTypeArgs []*TypeRef             `parser:"  [ '<' @@ { ',' @@ } '>' ] ]"`
	Fields      []*ImplementationField `parser:"[ '{' { @@ } '}' ]"`
}

// Accessor methods for Implementation to work with either Generic or NonGeneric

func (i *Implementation) GetTypeParams() []*TypeParam {
	if i.Generic != nil {
		return i.Generic.TypeParams
	}
	return nil
}

func (i *Implementation) GetName() string {
	if i.Generic != nil {
		return i.Generic.Name
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.Name
	}
	return ""
}

func (i *Implementation) GetTypeArgs() []*TypeRef {
	if i.Generic != nil {
		return i.Generic.TypeArgs
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.TypeArgs
	}
	return nil
}

func (i *Implementation) GetFor() string {
	if i.Generic != nil {
		return i.Generic.For
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.For
	}
	return ""
}

func (i *Implementation) GetForTypeArgs() []*TypeRef {
	if i.Generic != nil {
		return i.Generic.ForTypeArgs
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.ForTypeArgs
	}
	return nil
}

func (i *Implementation) GetFields() []*ImplementationField {
	if i.Generic != nil {
		return i.Generic.Fields
	}
	if i.NonGeneric != nil {
		return i.NonGeneric.Fields
	}
	return nil
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
	DocComment []string `parser:"{ @DocComment }"`
	Visibility string   `parser:"[ @'private' | @'public' | @'protected' ]"`
	Name       string   `parser:"'func' @Ident"`
	Arguments  []*Value `parser:"[ '(' [ @@ { ',' @@ } ] ')' ]"`
	Type       *TypeRef `parser:"[ ':' @@ ]"`
	Value      []*Entry `parser:"[ '{' @@* '}' ]"`
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
	Throws     *TypeRef     `parser:"[ 'throws' @@ ]"`
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
	Module   string     `parser:" | ( @Ident '.'"`
	Type     string     `parser:"     @Ident ) | @Ident)"`
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
	Throws     *TypeRef   `parser:"[ 'throws' @@ ]"`
}

type Literal struct {
	baseToken
	IsPointer    bool              `parser:"[ @'&' ]"`
	Intrinsic    *Intrinsic        `parser:"( @@"`
	FuncCall     *FuncCall         `parser:" | @@"`
	Bool         string            `parser:" | @( 'true' | 'false' )"`
	String       string            `parser:" | @String"`
	StructType     string            `parser:" | ( @Ident"`
	StructTypeArgs []*TypeRef       `parser:"     [ '<' @@ { ',' @@ } '>' ]"`
	StructFields   []*ObjectKeyValue `parser:"     '{' [ @@ { ',' @@ } ] '}' )"`
	SymbolModule string            // Populated during semantic analysis for module.symbol patterns
	Symbol       string            `parser:" | @Ident"`
	Number       string            `parser:" | @Number"`
	Object       []*ObjectKeyValue `parser:" | '{' [ @@ { ',' @@ } ] '}'"`
	Array        []*Literal        `parser:" | '[' [ @@ { ',' @@ } ] ']' )"`
	Chain        []*ChainAccess    `parser:"{ @@ }"`
	ArrayIndex   *Expression       `parser:"[ '[' @@ ']' ]"`
}

// ChainAccess represents a chained field or method access: .field or .method()
type ChainAccess struct {
	baseToken
	Name      string      `parser:"'.' @Ident"`
	TypeArgs  []*TypeRef  `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	HasParens bool        `parser:"[ @'('"`
	Args      []*Argument `parser:"  [ @@ { ',' @@ } ] ')' ]"`
}

// IsMethodCall returns true if this chain access is a method call (has parentheses)
func (c *ChainAccess) IsMethodCall() bool {
	return c.HasParens
}

// GetArgs returns the method call arguments
func (c *ChainAccess) GetArgs() []*Argument {
	return c.Args
}

// Intrinsic represents a compiler intrinsic call: @name(args) or @name<T>(args)
type Intrinsic struct {
	baseToken
	Name     string        `parser:"'@' @Ident"`
	TypeArgs []*TypeRef    `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Args     []*Expression `parser:"'(' [ @@ { ',' @@ } ] ')'"`
}

// MethodCall represents a chained method call as a statement: base.field.method()
// The last chain element must be a method call (with parentheses)
type MethodCall struct {
	baseToken
	Base  string         `parser:"@Ident"`
	Chain []*ChainAccess `parser:"@@ { @@ }"`
}

// IsValid returns true if the method call is valid (last chain element is a method call)
func (m *MethodCall) IsValid() bool {
	if len(m.Chain) == 0 {
		return false
	}
	return m.Chain[len(m.Chain)-1].IsMethodCall()
}

type FuncCall struct {
	baseToken
	// Static type call: module.Type<Args>::function() or Type::function()
	StaticModule   string     `parser:"[ ( @Ident '.')?"`
	StaticType     string     `parser:"    @Ident"`
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
