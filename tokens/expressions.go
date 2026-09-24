// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

// Package tokens contains the definitions for all gecko symbols/tokens
package tokens

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
	UnsafeBlock   *UnsafeBlock `parser:"| @@"`
	SubExpression *Expression  `parser:" | '(' @@ ')'"`
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

type FuncType struct {
	baseToken
	ParamTypes []*TypeRef `parser:"'func' '(' [ @@ { ',' @@ } ] ')'"`
	ReturnType *TypeRef   `parser:"[ ':' @@ ]"`
	Throws     *TypeRef   `parser:"[ 'throws' @@ ]"`
}

type Lambda struct {
	baseToken
	Params     []*Value `parser:"'fn' '(' [ @@ { ',' @@ } ] ')'"`
	ReturnType *TypeRef `parser:"[ ':' @@ ]"`
	Body       []*Entry `parser:"'{' @@* '}'"`
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

type Object struct {
	baseToken
}

type ObjectKeyValue struct {
	baseToken
	Key   string      `parser:"@Ident ':'"`
	Value *Expression `parser:"@@"`
}

// IsStructLiteral returns true if this Literal is a struct literal (TypeName { fields })
func (l *Literal) IsStructLiteral() bool {
	return l.StructType != "" && l.StructFields != nil
}
