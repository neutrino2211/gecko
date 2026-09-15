// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

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
