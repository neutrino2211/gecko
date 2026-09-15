// spec: spec/types.md, spec/traits.md, spec/modules.md, spec/scoping.md

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
