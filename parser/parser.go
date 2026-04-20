package parser

import (
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

var geckoLexer = lexer.MustSimple([]lexer.SimpleRule{
	// Comments (order matters: DocComment before Comment)
	{Name: "DocComment", Pattern: `///[^\n]*`},
	{Name: "Comment", Pattern: `//[^\n]*`},

	// Multi-character operators (must come before single-char Punct)
	{Name: "DoubleColon", Pattern: `::`},
	{Name: "LogicalAnd", Pattern: `&&`},
	{Name: "LogicalOr", Pattern: `\|\|`},

	// Strings (before Punct to capture quotes)
	{Name: "String", Pattern: `"(?:[^"\\]|\\.)*"`},

	// Numbers (hex, then decimal with optional float)
	{Name: "Number", Pattern: `0[xX][0-9a-fA-F]+|[0-9]+(?:\.[0-9]+)?`},

	// Identifiers
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},

	// Punctuation (single characters - order after multi-char operators)
	{Name: "Punct", Pattern: `[<>=!+\-*/%&|^~.,;:?(){}[\]@#]`},

	// Whitespace (elided)
	{Name: "Whitespace", Pattern: `[ \t\n\r]+`},
})

var Parser = participle.MustBuild[tokens.File](
	participle.Lexer(geckoLexer),
	participle.UseLookahead(50),
	participle.Elide("Comment", "Whitespace"),
)
