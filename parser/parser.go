package parser

import (
	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/participle/lexer/ebnf"
	"github.com/neutrino2211/gecko/tokens"
)

var graphQLLexer = lexer.Must(ebnf.New(`
DocComment = "/" "/" "/" { "\u0000"…"\uffff"-"\n" } .
Comment = "/" "/" { "\u0000"…"\uffff"-"\n" } .
Ident = (alpha | "_") { "_" | alpha | digit } .
Name = (alpha | "_" ) { "_" | alpha | digit } .
SingleQuoteString = "'" [ { "\u0000"…"\uffff"-"\""-"\\" | "\\" Any } ] "'" .
String = "\"" [ { "\u0000"…"\uffff"-"\""-"\\" | "\\" Any } ] "\"" .
Number = "0" ( "x" hexdigit { hexdigit } | { digit | "." | "_" } ) | nonzerodigit { digit | "." | "_" } .
Whitespace = " " | "\t" | "\n" | "\r" .
DoubleColon = "::" .
LogicalAnd = "&" "&" .
LogicalOr = "|" "|" .
Punct = "!"…"/" | ":"…"@" | "["…` + "\"`\"" + ` | "{"…"~" .
alpha = "a"…"z" | "A"…"Z" .
digit = "0"…"9" .
nonzerodigit = "1"…"9" .
hexdigit = "0"…"9" | "a"…"f" | "A"…"F" .
EOL = ( "\n" | "\r" ) { "\n" | "\r" } .
Any = "\u0000"…"\uffff" .
`))

var Parser = participle.MustBuild(
	&tokens.File{},
	participle.UseLookahead(50),
	participle.Lexer(graphQLLexer),
	participle.Elide("Comment", "Whitespace"),
)
