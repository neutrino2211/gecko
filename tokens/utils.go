// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package tokens

import (
	"math/rand"
	"strconv"
	"time"
)

func (t *baseToken) GetID() string {
	s := rand.NewSource(time.Now().UnixNano() + rand.Int63())
	r := rand.New(s)
	if t.RefID == "" {
		i := 0
		for i < 32 {
			t.RefID += strconv.Itoa(r.Intn(9))
			i++
		}
	}

	return t.RefID
}
