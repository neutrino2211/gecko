package parser

import (
	"testing"
)

func TestRangePattern(t *testing.T) {
	input := `external func main(): int32 {
    let x: int32 = 5
    let y: int32 = match x {
        1..10 => 100
        default => 0
    }
    return y
}
`
	_, err := Parser.ParseString("test.gecko", input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
}
