// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md
package parser
import (
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)
// geckoLexer tokenizes Gecko source. lexer.MustSimple evaluates rules in order
// and emits the first rule that matches at the current offset, so rule ordering
// is significant. The rules are grouped from most specific to most general:
//
//  1. Comments first: `///` must win over `//`, and both must win over the `/`
//     and `/=` operators, otherwise a comment's leading slashes tokenize as Punct.
//  2. Multi-character operators before single-character Punct so `...`, `::`,
//     `&&`, `++`, `+=`, ... are not split into their component characters.
//     Within the group, longer spellings precede shorter ones (`...` before `..`).
//  3. String literals before identifiers/operators so their delimiters and
//     contents are consumed atomically.
//  4. Numeric literals before identifiers so numbers are never read as names.
//  5. Identifiers before Punct. Keywords need no lexer rule: the parser matches
//     them as Ident token *values* (e.g. 'import', 'as'), which keeps them
//     usable as ordinary identifiers elsewhere.
//  6. Whitespace last; it is elided, and trailing it makes the precedence of
//     every other rule explicit.
var geckoLexer = lexer.MustSimple([]lexer.SimpleRule{
	// 1. Comments (doc comments must precede plain comments).
	{Name: "DocComment", Pattern: `///[^\n]*`},
	{Name: "Comment", Pattern: `//[^\n]*`},
	// 2. Multi-character operators, longest spellings first.
	{Name: "Ellipsis", Pattern: `\.\.\.`},
	{Name: "RangeDot", Pattern: `\.\.`},
	{Name: "DoubleColon", Pattern: `::`},
	{Name: "LogicalAnd", Pattern: `&&`},
	{Name: "LogicalOr", Pattern: `\|\|`},
	{Name: "PlusAssign", Pattern: `\+=`},
	{Name: "MinusAssign", Pattern: `-=`},
	{Name: "StarAssign", Pattern: `\*=`},
	{Name: "SlashAssign", Pattern: `/=`},
	{Name: "AndAssign", Pattern: `&=`},
	{Name: "OrAssign", Pattern: `\|=`},
	{Name: "XorAssign", Pattern: `\^=`},
	{Name: "ModuloAssign", Pattern: `%=`},
	{Name: "Inc", Pattern: `\+\+`},
	{Name: "Dec", Pattern: `--`},
	{Name: "Arrow", Pattern: `=>`},
	// 3. String literals.
	{Name: "BacktickString", Pattern: "`[^`]*`"},
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},
	// 4. Numeric literals (hex before decimal).
	{Name: "Number", Pattern: `0[xX][0-9a-fA-F]+|[0-9]+(?:\.[0-9]+)?`},
	// 5. Identifiers.
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	// 6. Single-character punctuation.
	{Name: "Punct", Pattern: `[<>=!+\-*/%&|^~.,;:?(){}[\]@#]`},
	// Elided whitespace.
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
})
var Parser = participle.MustBuild[tokens.File](
	participle.Lexer(geckoLexer),
	participle.UseLookahead(50),
	participle.Elide("Comment", "Whitespace"),
)
