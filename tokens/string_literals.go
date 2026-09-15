// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md, spec/stdlib.md

package tokens

import "strconv"

// StringLiteralToken returns the raw source token for a string literal.
func (l *Literal) StringLiteralToken() string {
	if l == nil {
		return ""
	}
	if l.String != "" {
		return l.String
	}
	return l.BacktickString
}

// HasStringLiteral reports whether this literal is a string literal.
func (l *Literal) HasStringLiteral() bool {
	return l.StringLiteralToken() != ""
}

// StringLiteralValue returns the decoded runtime value for the string literal.
func (l *Literal) StringLiteralValue() (string, error) {
	token := l.StringLiteralToken()
	if token == "" {
		return "", nil
	}
	return strconv.Unquote(token)
}

// CStringLiteral returns the literal encoded as a C string token.
func (l *Literal) CStringLiteral() (string, error) {
	value, err := l.StringLiteralValue()
	if err != nil {
		return "", err
	}
	return strconv.Quote(value), nil
}
