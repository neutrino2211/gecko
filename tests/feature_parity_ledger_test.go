// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/modules.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package tests

import (
	"testing"

	"github.com/neutrino2211/gecko/backends"
)

func TestFeatureParityLedgerCoverage(t *testing.T) {
	for _, feature := range backends.AllFeatures() {
		record, ok := backends.FeatureParityLedger[feature]
		if !ok {
			t.Fatalf("missing parity ledger entry for feature %q", feature)
		}
		if record.Feature != feature {
			t.Fatalf("feature ledger key/value mismatch for %q", feature)
		}
		if len(record.TestLinks) == 0 {
			t.Fatalf("feature %q must have at least one test link in parity ledger", feature)
		}
	}
}

func TestLLVMParitySliceOneLedger(t *testing.T) {
	if len(backends.LLVMParitySliceOne) != 3 {
		t.Fatalf("unexpected LLVM parity slice-one size: got %d want 3", len(backends.LLVMParitySliceOne))
	}

	for _, feature := range backends.LLVMParitySliceOne {
		record, ok := backends.FeatureParityLedger[feature]
		if !ok {
			t.Fatalf("missing parity ledger entry for slice-one feature %q", feature)
		}
		if record.LLVM != backends.ParitySupported {
			t.Fatalf("slice-one feature %q must be marked LLVM supported; got %q", feature, record.LLVM)
		}
	}
}
