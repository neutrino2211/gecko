// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

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

// UnsafeBlockErrorNames maps an `@unsafe with ...` block to the synthesized C
// error-enum type name (variants = handler ids) produced by the analyzer. It is a
// package-level map (rather than a `parser:"-"` field on UnsafeBlock) because
// participle's LL(1) builder rejects an ignored field on a
// struct that also carries an `@@*` body.
var UnsafeBlockErrorNames = map[*UnsafeBlock]string{}

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
	Alias      string   // Import alias (e.g., "Vec" from `import std.collections as Vec`)
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
	Alias            string // Import alias (e.g., "Vec" from `import std.collections as Vec`)
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
	Header  string           `parser:"'cimport' @String"`
	Clauses []*CImportClause `parser:"{ @@ }"`
}

type CImportClause struct {
	baseToken
	WithObject  string `parser:"'withobject' @String"`
	WithLibrary string `parser:"| 'withlibrary' @String"`
}

type Foreign struct {
	baseToken
	Backend string           `parser:"'foreign' @String"`
	Module  string           `parser:"@Ident"`
	Clauses []*ForeignClause `parser:"{ @@ }"`
	Members []*ForeignMember `parser:"'{' { @@ } '}'"`
}

type ForeignClause struct {
	baseToken
	WithHeader  string `parser:"'withheader' @String"`
	WithLibrary string `parser:"| 'withlibrary' @String"`
	WithObject  string `parser:"| 'withobject' @String"`
}

type ForeignMember struct {
	baseToken
	Type   *ForeignType   `parser:"@@"`
	Method *ForeignMethod `parser:"| @@"`
}

type ForeignType struct {
	baseToken
	Name string `parser:"'type' @Ident 'opaque'"`
}

type ForeignMethod struct {
	baseToken
	Name      string   `parser:"'func' @Ident"`
	Arguments []*Value `parser:"'(' [ @@ { ',' @@ } ]"`
	Variadic  bool     `parser:"[ ',' @'...' | @'...' ] ')'"`
	Type      *TypeRef `parser:"[ ':' @@ ]"`
	Throws    *TypeRef `parser:"[ 'throws' @@ ]"`
	As        string   `parser:"[ 'as' @String ]"`
}

func (m *ForeignMethod) IsVariadic() bool {
	if m == nil {
		return false
	}
	return m.Variadic
}

// GetWithObjects returns all withobject values in declaration order.
func (c *CImport) GetWithObjects() []string {
	var objects []string
	for _, clause := range c.Clauses {
		if clause.WithObject != "" {
			objects = append(objects, clause.WithObject)
		}
	}
	return objects
}

// GetWithLibraries returns all withlibrary values in declaration order.
func (c *CImport) GetWithLibraries() []string {
	var libs []string
	for _, clause := range c.Clauses {
		if clause.WithLibrary != "" {
			libs = append(libs, clause.WithLibrary)
		}
	}
	return libs
}

func (f *Foreign) GetWithHeaders() []string {
	var headers []string
	for _, clause := range f.Clauses {
		if clause.WithHeader != "" {
			headers = append(headers, clause.WithHeader)
		}
	}
	return headers
}

func (f *Foreign) GetWithLibraries() []string {
	var libs []string
	for _, clause := range f.Clauses {
		if clause.WithLibrary != "" {
			libs = append(libs, clause.WithLibrary)
		}
	}
	return libs
}

func (f *Foreign) GetWithObjects() []string {
	var objects []string
	for _, clause := range f.Clauses {
		if clause.WithObject != "" {
			objects = append(objects, clause.WithObject)
		}
	}
	return objects
}

type Import struct {
	baseToken
	Path    []string `parser:"'import' @Ident { '.' @Ident }"`
	Alias   string   `parser:"[ 'as' @Ident ]"`
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

// ModuleName returns the alias if set, otherwise the last component of the import path
func (i *Import) ModuleName() string {
	if i.Alias != "" {
		return i.Alias
	}
	if len(i.Path) == 0 {
		return ""
	}
	return i.Path[len(i.Path)-1]
}

type Entry struct {
	baseToken
	Return     *Expression `parser:"'return' @@"`
	VoidReturn *bool       `parser:"| @'return'"`
	Break      *bool       `parser:"| @'break'"`
	Continue   *bool       `parser:"| @'continue'"`
	// Keyword-led statements that begin with a bare identifier must precede
	// ExprStmt. Expression parsing greedily consumes a leading identifier, so
	// `import ...`, `cimport ...`, `asm ...` and `impl ...` would otherwise be
	// misread as an expression statement and never reach their own alternatives.
	Import         *Import         `parser:"| @@"`
	CImport        *CImport        `parser:"| @@"`
	Asm            *Asm            `parser:"| @@"`
	// Implementation must come before Assignment to prevent 'impl' being parsed as identifier
	Implementation *Implementation `parser:"| @@"`
	Assignment     *Assignment     `parser:"| @@"`
	ElseIf         *ElseIf         `parser:"| @@"`
	Else           *Else           `parser:"| @@"`
	If             *If             `parser:"| @@"`
	Match          *Match          `parser:"| @@"`
	Defer          *Defer          `parser:"| @@"`
	// Declarations with optional attributes must come before Intrinsic
	// so @attr func/class/trait is parsed as declaration, not intrinsic
	Class       *Class       `parser:"| @@"`
	Trait       *Trait       `parser:"| @@"`
	Method      *Method      `parser:"| @@"`
	// Destructuring declarations must precede Field: `let Point { x, y } = p`
	// otherwise Field consumes `let Point` and leaves the braces unconsumed.
	Destructuring *DestructuringDeclaration `parser:"| @@"`
	Field         *Field                    `parser:"| @@"`
	Declaration   *Declaration              `parser:"| @@"`
	Foreign     *Foreign     `parser:"| @@"`
	Enum        *Enum        `parser:"| @@"`
	// @unsafe { ... } must precede Intrinsic so `@unsafe {` is not read as `@unsafe(`
	UnsafeBlock *UnsafeBlock `parser:"| @@"`
	// Intrinsic must come before declarations
	Intrinsic *Intrinsic `parser:"| @@"`
	// IncDec must come before ExprStmt to avoid i++ being parsed as expression i followed by unexpected ++
	IncDec *IncDec `parser:"| @@"`
	// Loop must come before ExprStmt to avoid `loop {}` being parsed as expression `loop` followed by unexpected `{`
	Loop *Loop `parser:"| @@"`
	// FuncCall and MethodCall are more specific than the generic ExprStmt and
	// must precede it; otherwise ExprStmt (an Expression, which itself contains
	// a FuncCall) swallows every call statement and these alternatives are dead.
	// FuncCall is tried before MethodCall: it captures a single optional
	// `module.`/`Type::` prefix, so `math.add()` stays a module/static call while
	// deeper chains like `self.field.method()` fail FuncCall and fall through to
	// MethodCall.
	FuncCall   *FuncCall   `parser:"| @@"`
	MethodCall *MethodCall `parser:"| @@"`
	ExprStmt   *Expression `parser:"| @@"`
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

// WhereClause is a trailing generic constraint list:
// `func f<T, U>(...) where T is Eq & Ord, U is Show { ... }`.
type WhereClause struct {
	baseToken
	Constraints []*WhereConstraint `parser:"'where' @@ { ',' @@ }"`
}

// WhereConstraint constrains one type parameter: `T is Eq & Ord`.
type WhereConstraint struct {
	baseToken
	Name   string   `parser:"@Ident 'is'"`
	Trait  string   `parser:"@Ident"`
	Traits []string `parser:"{ '&' @Ident }"`
}

// AllTraits returns every trait named in the constraint.
func (c *WhereConstraint) AllTraits() []string {
	if c == nil || c.Trait == "" {
		return nil
	}
	return append([]string{c.Trait}, c.Traits...)
}

// HasTrait reports whether name is already one of the parameter's constraints.
func (t *TypeParam) HasTrait(name string) bool {
	if t == nil || name == "" {
		return false
	}
	if t.Trait == name {
		return true
	}
	for _, tr := range t.Traits {
		if tr == name {
			return true
		}
	}
	return false
}

// ApplyWhereClause merges where-clause constraints into the matching type
// parameters, so consumers can keep reading constraints via TypeParam.AllTraits.
// The operation is idempotent, which makes it safe to run more than once.
func ApplyWhereClause(typeParams []*TypeParam, where *WhereClause) {
	if where == nil {
		return
	}
	byName := make(map[string]*TypeParam, len(typeParams))
	for _, tp := range typeParams {
		if tp != nil {
			byName[tp.Name] = tp
		}
	}
	for _, c := range where.Constraints {
		if c == nil {
			continue
		}
		tp, ok := byName[c.Name]
		if !ok {
			continue
		}
		for _, trait := range c.AllTraits() {
			if tp.HasTrait(trait) {
				continue
			}
			if tp.Trait == "" {
				tp.Trait = trait
			} else {
				tp.Traits = append(tp.Traits, trait)
			}
		}
	}
}

// NormalizeWhereClauses folds every `where` clause in a parsed file into the
// matching type parameters. Call once after parsing so all later phases see a
// single, uniform source of generic constraints.
func NormalizeWhereClauses(file *File) {
	if file == nil {
		return
	}
	for _, entry := range file.Entries {
		if entry == nil {
			continue
		}
		if entry.Method != nil {
			ApplyWhereClause(entry.Method.TypeParams, entry.Method.Where)
		}
		if entry.Class != nil {
			ApplyWhereClause(entry.Class.TypeParams, entry.Class.Where)
			for _, field := range entry.Class.Fields {
				if field != nil && field.Method != nil {
					ApplyWhereClause(field.Method.TypeParams, field.Method.Where)
				}
			}
		}
	}
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
	Where           *WhereClause       `parser:"[ @@ ]"`
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

type MatchCase struct {
	baseToken
	Pattern *MatchPattern   `parser:"@@"`
	Guard   *Expression     `parser:"[ 'if' @@ ]"`
	Body    *MatchCaseBody  `parser:" '=>' @@"`
}

// MatchCaseBody handles both block and expression bodies for match cases
type MatchCaseBody struct {
	baseToken
	Entries []*Entry   `parser:" '{' { @@ } '}'"`
	Expr    *Expression `parser:"| @@"`
}

type Match struct {
	baseToken
	Scrutinee *Expression  `parser:"'match' @@"`
	Cases     []*MatchCase `parser:"'{' { @@ } '}'"`
}

// MatchPattern represents a pattern with optional OR alternatives
type MatchPattern struct {
	baseToken
	Alts []*MatchPatternAlt `parser:"@@ { '|' @@ }"`
}

// MatchPatternAlt is a single pattern alternative
type MatchPatternAlt struct {
	baseToken
	Binding *string            `parser:"[ @Ident '@' ]"`
	Inner   *MatchPatternInner `parser:"@@"`
}

// MatchPatternInner is the core pattern (wildcard, literal, range, destructuring)
type MatchPatternInner struct {
	baseToken
	Wildcard    *bool               `parser:"@'_'"`
	Range       *RangePattern       `parser:"| @@"`
	Destructure *DestructurePattern `parser:"| @@"`
	Literal     *Primary            `parser:"| @@"`
}

// DestructurePattern handles struct destructuring: Type { field = pattern, ... }
type DestructurePattern struct {
	baseToken
	TypeName string              `parser:"@Ident"`
	TypeArgs []*TypeRef          `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Fields   []*DestructureField `parser:"'{' @@ { ',' @@ } '}'"`
}

// DestructureField is a field in a destructuring pattern
type DestructureField struct {
	baseToken
	Name    string        `parser:"@Ident '='"`
	Pattern *MatchPattern `parser:"@@"`
}

// RangePattern handles range matching: start..end
type RangePattern struct {
	baseToken
	Start *Primary `parser:"@@"`
	Dot   string   `parser:"@RangeDot"`
	End   *Primary `parser:"@@"`
}

type Defer struct {
	baseToken
	Expression *Expression `parser:"'defer' @@"`
}

// Expressions

type Expression struct {
	baseToken
	Cond *ConditionalExpr `parser:"@@"`
}

// ConditionalExpr handles ternary: cond ? a : b
type ConditionalExpr struct {
	baseToken
	LogicalOr *OrExpression    `parser:"@@"`
	TrueExpr  *Expression      `parser:"[ '?' @@"`
	FalseExpr *ConditionalExpr `parser:"':' @@ ]"`
}

// GetLambda returns the Lambda literal if this expression is a bare lambda (e.g., fun(x) { ... }),
// or nil otherwise.
func (e *Expression) GetLambda() *Lambda {
	if e == nil || e.Cond == nil || e.Cond.LogicalOr == nil || e.Cond.LogicalOr.LogicalOr == nil {
		return nil
	}
	lo := e.Cond.LogicalOr.LogicalOr
	if lo.Next != nil || lo.Op != "" {
		return nil
	}
	la := lo.LogicalAnd
	if la == nil || la.Next != nil || la.Op != "" {
		return nil
	}
	eq := la.Equality
	if eq == nil || eq.Next != nil || eq.Op != "" {
		return nil
	}
	c := eq.Comparison
	if c == nil || c.Next != nil || c.Op != "" {
		return nil
	}
	a := c.Addition
	if a == nil || a.Next != nil || a.Op != "" {
		return nil
	}
	m := a.Multiplication
	if m == nil || m.Next != nil || m.Op != "" {
		return nil
	}
	u := m.Unary
	if u == nil || u.Cast != nil || u.Unary != nil {
		return nil
	}
	p := u.Primary
	if p == nil || p.Literal == nil {
		return nil
	}
	return p.Literal.Lambda
}

// GetUnsafeBlock returns the UnsafeBlock if this expression is (or bottoms out
// in) an `@unsafe with ... { }` block, or nil otherwise. Used to detect the block
// in its expression positions (`let r = @unsafe ...` initializer and `r = @unsafe ...`
// reassignment) so it can be lowered as a statement that yields a Result.
func (e *Expression) GetUnsafeBlock() *UnsafeBlock {
	if e == nil || e.Cond == nil || e.Cond.LogicalOr == nil || e.Cond.LogicalOr.LogicalOr == nil {
		return nil
	}
	lo := e.Cond.LogicalOr.LogicalOr
	if lo.Next != nil || lo.Op != "" {
		return nil
	}
	la := lo.LogicalAnd
	if la == nil || la.Next != nil || la.Op != "" {
		return nil
	}
	eq := la.Equality
	if eq == nil || eq.Next != nil || eq.Op != "" {
		return nil
	}
	c := eq.Comparison
	if c == nil || c.Next != nil || c.Op != "" {
		return nil
	}
	a := c.Addition
	if a == nil || a.Next != nil || a.Op != "" {
		return nil
	}
	m := a.Multiplication
	if m == nil || m.Next != nil || m.Op != "" {
		return nil
	}
	u := m.Unary
	if u == nil || u.Cast != nil || u.Unary != nil {
		return nil
	}
	p := u.Primary
	if p == nil {
		return nil
	}
	return p.UnsafeBlock
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
	// UnsafeBlock lets `@unsafe with ... { }` be used as an expression that yields
	// a Result<T, E> (e.g. `let r = @unsafe with h { ... }` or `r = @unsafe ...`).
	UnsafeBlock *UnsafeBlock `parser:"| @@"`
	SubExpression *Expression `parser:" | '(' @@ ')'"`
}

// ToExpression wraps a Primary into a full Expression so it can be lowered
// through the normal expression pipeline (used for `@unsafe with` handlers,
// which are parsed as Primaries to keep `&` a handler separator).
func (p *Primary) ToExpression() *Expression {
	return &Expression{
		Cond: &ConditionalExpr{
			LogicalOr: &OrExpression{
				LogicalOr: &LogicalOr{
					LogicalAnd: &LogicalAnd{
						Equality: &Equality{
							Comparison: &Comparison{
								Addition: &Addition{
									Multiplication: &Multiplication{
										Unary: &Unary{Primary: p},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// GetBareIdentifier returns the bare identifier if the primary is just a simple symbol, or "" otherwise.
func (p *Primary) GetBareIdentifier() string {
	if p == nil || p.Literal == nil {
		return ""
	}
	if p.Literal.Symbol != "" && len(p.Literal.Chain) == 0 && p.Literal.ArrayIndex == nil {
		return p.Literal.Symbol
	}
	return ""
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
	Parent     string                 `parser:"[ ':' @Ident ]"`
	Fields     []*ImplementationField `parser:"'{' { @@ } '}'"`
}

// AllParents returns all inherited trait names.
func (t *Trait) AllParents() []string {
	if t == nil || t.Parent == "" {
		return nil
	}
	return []string{t.Parent}
}

type Implementation struct {
	baseToken
	DocComment []string     `parser:"{ @DocComment }"`
	Visibility string        `parser:"[ @'private' | @'public' | @'protected' ]"`
	Default    bool          `parser:"[ @'default' ]"`
	Attributes []*Attribute  `parser:"{ @@ }"`
	Generic    *GenericImpl  `parser:"'impl' ( @@"`
	NonGeneric *NonGenericImpl `parser:"       | @@ )"`
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

// GetAttributes returns the compile-time attributes attached to the impl
// (e.g. @attach_handler), used to register unsafe-handler coverage globally.
func (i *Implementation) GetAttributes() []*Attribute {
	return i.Attributes
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

// DestructuringDeclaration binds struct fields to local variables in one
// statement: `let Point { x, y } = p`. Shorthand `x` binds field x to local x;
// `x = a` binds field x to local a.
type DestructuringDeclaration struct {
	baseToken
	Mutability string                `parser:"(@'let' | @'const')"`
	TypeName   string                `parser:"@Ident"`
	TypeArgs   []*TypeRef            `parser:"[ '<' @@ { ',' @@ } '>' ]"`
	Bindings   []*DestructureBinding `parser:"'{' @@ { ',' @@ } '}'"`
	Value      *Expression           `parser:"'=' @@"`
}

// DestructureBinding is one field binding in a DestructuringDeclaration.
type DestructureBinding struct {
	baseToken
	Field string `parser:"@Ident"`
	Alias string `parser:"[ '=' @Ident ]"`
}

// Target returns the local variable name a binding introduces.
func (b *DestructureBinding) Target() string {
	if b == nil {
		return ""
	}
	if b.Alias != "" {
		return b.Alias
	}
	return b.Field
}

type Assignment struct {
	baseToken
	Global bool        `parser:"[ @'global' ]"`
	Name   string      `parser:"@Ident"`
	Field  string      `parser:"[ '.' @Ident ]"`
	Index  *Expression `parser:"[ '[' @@ ']' ]"`
	Op     string      `parser:"@( '=' | '+=' | '-=' | '*=' | '/=' | '&=' | '|=' | '^=' | '%=' )"`
	Value  *Expression `parser:"@@"`
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
	Arguments  []*Value     `parser:"'(' [ @@ { ',' @@ } ]"`
	Variadic   bool         `parser:"[ ',' @'...' | @'...' ] ')'"`
	Type       *TypeRef     `parser:"[ ':' @@ ]"`
	Throws     *TypeRef     `parser:"[ 'throws' @@ ]"`
	Where      *WhereClause `parser:"[ @@ ]"`
	LinkName   string
	Value      []*Entry `parser:"[ '{' @@* '}' ]"`
}

func (m *Method) IsVariadic() bool {
	if m == nil {
		return false
	}
	return m.Variadic || m.Variardic
}

type Value struct {
	baseToken
	Variadic bool        `parser:"[ @'...' ]"`
	Name     string      `parser:"@Ident"`
	Out      bool        `parser:"[ ':' [ @'out' ]"`
	Type     *TypeRef    `parser:"@@ ]"`
	Default  *Expression `parser:"[ '=' @@ ]"`
}

type Argument struct {
	baseToken
	Name    string      `parser:"[ @Ident ':' ]"`
	Out     bool        `parser:"[ @'out' ]"`
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
	// Const is the readonly type qualifier: `T readonly` / `T readonly*`.
	// Distinct from binding `const` (Mutability); strips only via as! in @unsafe.
	Const    bool       `parser:"[ @'readonly' ]"`
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

type Lambda struct {
	baseToken
	Params     []*Value    `parser:"'fn' '(' [ @@ { ',' @@ } ] ')'"`
	ReturnType *TypeRef    `parser:"[ ':' @@ ]"`
	Body       []*Entry    `parser:"'{' @@* '}'"`
}

type Literal struct {
	baseToken
	IsPointer      bool              `parser:"[ @'&' ]"`
	Intrinsic      *Intrinsic        `parser:"( @@"`
	Lambda         *Lambda           `parser:" | @@"`
	FuncCall       *FuncCall         `parser:" | @@"`
	IncDec         *IncDec           `parser:" | @@"`
	Match          *Match            `parser:" | @@"`
	Bool           string            `parser:" | @( 'true' | 'false' )"`
	String         string            `parser:" | @String"`
	BacktickString string            `parser:" | @BacktickString"`
	StructType     string            `parser:" | ( @Ident"`
	StructTypeArgs []*TypeRef        `parser:"     [ '<' @@ { ',' @@ } '>' ]"`
	StructFields   []*ObjectKeyValue `parser:"     '{' [ @@ { ',' @@ } ] '}' )"`
	SymbolModule   string            // Populated during semantic analysis for module.symbol patterns
	Symbol         string            `parser:" | @Ident"`
	Number         string            `parser:" | @Number"`
	Object         []*ObjectKeyValue `parser:" | '{' [ @@ { ',' @@ } ] '}'"`
	Array          []*Literal        `parser:" | '[' [ @@ { ',' @@ } ] ']' )"`
	Chain          []*ChainAccess    `parser:"{ @@ }"`
	ArrayIndex     *Expression       `parser:"[ '[' @@ ']' ]"`
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

// UnsafeBlock is an explicit unsafe region: `@unsafe { ... }` or
// `@unsafe with H1 & H2 { ... }`. The `with` form activates unsafe handler
// instances that guard the intrinsics used inside the block.
type UnsafeBlock struct {
	baseToken
	// Handlers are parsed as Primary expressions (not full Expressions) so the
	// composing `&` is treated as a handler separator, not a bitwise-AND.
	Handlers []*Primary `parser:"'@' 'unsafe' [ 'with' @@ { '&' @@ } ]"`
	Body []*Entry   `parser:"'{' @@* '}'"`
}

// UnsafeBlockBindNames maps an `@unsafe with ...` block to the variable name it
// is yielded into (`let r = @unsafe ...` or `r = @unsafe ...`). It is a
// package-level map rather than a parsed field because participle's LL(1) builder
// rejects an ignored field here; the binder is set programmatically by the
// analyzer/codegen, never parsed from source (there is no `as name` syntax).
var UnsafeBlockBindNames = map[*UnsafeBlock]string{}

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

type IncDec struct {
	baseToken
	Target string `parser:"@Ident"`
	Op     string `parser:"@( Inc | Dec )"`
}

type Loop struct {
	baseToken
	For           string      `parser:"( 'for'"`
	ForOf         *ForOfLoop  `parser:"  ( @@"`
	ForIn         *ForInLoop  `parser:"   | @@"`
	ForExpression *Expression `parser:"   | @@ )"`
	While         string      `parser:"| 'while'"`
	WhileExpr     *Expression `parser:"  @@ "`
	LoopKw        string      `parser:"| 'loop'"`
	Value         []*Entry    `parser:") '{' @@* '}' "`
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
