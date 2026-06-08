// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package tokens

import (
	"crypto/rand"
	"encoding/hex"
	mrand "math/rand"
	"time"
)

func (t *baseToken) GetID() string {
	if t.RefID != "" {
		return t.RefID
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
	}
	t.RefID = hex.EncodeToString(b)
	return t.RefID
}
