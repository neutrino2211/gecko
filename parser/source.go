package parser

import (
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/neutrino2211/gecko/tokens"
)

type Source struct {
	File   *tokens.File
	Tokens []SourceToken
}

type recordingLexer struct {
	lexer.Lexer
	tokens []SourceToken
}

var tokenNames = lexer.SymbolsByRune(geckoLexer)

func (l *recordingLexer) Next() (lexer.Token, error) {
	token, err := l.Lexer.Next()
	if err == nil && token.Type != lexer.EOF {
		l.tokens = append(l.tokens, SourceToken{Kind: tokenNames[token.Type], Value: token.Value, Offset: token.Pos.Offset})
	}
	return token, err
}

func ParseSource(path, content string) (*Source, error) {
	source := &Source{}
	stream, err := geckoLexer.LexString(path, content)
	if err != nil {
		return source, err
	}
	recorded := &recordingLexer{Lexer: stream}
	symbols := geckoLexer.Symbols()
	peeker, err := lexer.Upgrade(recorded, symbols["Comment"], symbols["Whitespace"])
	source.Tokens = recorded.tokens
	if err != nil {
		return source, err
	}
	file, err := Parser.ParseFromLexer(peeker)
	source.File = file
	if file != nil {
		file.Path = path
		file.Content = content
		tokens.NormalizeWhereClauses(file)
	}
	return source, err
}
