// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

// Package tokens contains the definitions for all gecko symbols/tokens
package tokens

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
	Pattern *MatchPattern  `parser:"@@"`
	Guard   *Expression    `parser:"[ 'if' @@ ]"`
	Body    *MatchCaseBody `parser:" '=>' @@"`
}

// MatchCaseBody handles both block and expression bodies for match cases
type MatchCaseBody struct {
	baseToken
	Entries []*Entry    `parser:" '{' { @@ } '}'"`
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

// UnsafeBlock is an explicit unsafe region: `@unsafe { ... }` or
// `@unsafe with H1 & H2 { ... }`. The `with` form activates unsafe handler
// instances that guard the intrinsics used inside the block.
type UnsafeBlock struct {
	baseToken
	Setup *UnsafeSetup `parser:"'@' 'unsafe' [ @@ ]"`
	// Handlers are parsed as Primary expressions (not full Expressions) so the
	// composing `&` is treated as a handler separator, not a bitwise-AND.
	Handlers []*Primary `parser:"[ 'with' @@ { '&' @@ } ]"`
	Body     []*Entry   `parser:"'{' @@* '}'"`
}

type UnsafeSetup struct {
	baseToken
	Bindings []*UnsafeBinding `parser:"'[' @@ { ',' @@ } ']'"`
}

type UnsafeBinding struct {
	baseToken
	Name  string      `parser:"@Ident"`
	Type  *TypeRef    `parser:"[ ':' @@ ]"`
	Value *Expression `parser:"'=' @@"`
}

func (b *UnsafeBinding) Field() *Field {
	return &Field{baseToken: b.baseToken, Mutability: "let", Name: b.Name, Type: b.Type, Value: b.Value}
}

func (e *Entry) UnsafeValue() *Expression {
	if e.ExprStmt != nil {
		return e.ExprStmt
	}
	lit := &Literal{}
	switch {
	case e.Intrinsic != nil && e.Intrinsic.Store == nil:
		lit.Intrinsic = e.Intrinsic
	case e.FuncCall != nil:
		lit.FuncCall = e.FuncCall
	case e.MethodCall != nil:
		lit.Symbol = e.MethodCall.Base
		lit.Chain = e.MethodCall.Chain
	default:
		return nil
	}
	return (&Primary{Literal: lit}).ToExpression()
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
	Store    *Expression   `parser:"[ '=' @@ ]"`
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
