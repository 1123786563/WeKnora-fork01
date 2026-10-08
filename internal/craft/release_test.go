package craft

import "testing"

// O05 Step 1 (brief-verbatim): a paid release without commercial billing
// evidence must never pass the gate, while the same facts stay releasable
// in the controlled (unpaid) mode.
func TestPaidReleaseRequiresCommercialEvidence(t *testing.T) {
	f := ReleaseFacts{Web: true, Permissions: true, Cancel: true, Reconnect: true, Recovery: true, Versioning: true, Isolation: true, Quota: true, Usage: true}
	if CanRelease(f, true) {
		t.Fatal("paid without billing")
	}
	if !CanRelease(f, false) {
		t.Fatal("controlled release rejected")
	}
}

// Every axis is load-bearing: dropping any one of the ten facts must reject
// even the controlled release (Billing only gates the paid mode).
func TestCanReleaseRequiresEveryAxis(t *testing.T) {
	all := ReleaseFacts{Web: true, Permissions: true, Cancel: true, Reconnect: true, Recovery: true, Versioning: true, Isolation: true, Quota: true, Usage: true, Billing: true}
	if !CanRelease(all, true) || !CanRelease(all, false) {
		t.Fatal("full facts rejected")
	}
	axes := []struct {
		name  string
		clear func(f *ReleaseFacts)
	}{{
		name: "Web", clear: func(f *ReleaseFacts) { f.Web = false },
	}, {
		name: "Permissions", clear: func(f *ReleaseFacts) { f.Permissions = false },
	}, {
		name: "Cancel", clear: func(f *ReleaseFacts) { f.Cancel = false },
	}, {
		name: "Reconnect", clear: func(f *ReleaseFacts) { f.Reconnect = false },
	}, {
		name: "Recovery", clear: func(f *ReleaseFacts) { f.Recovery = false },
	}, {
		name: "Versioning", clear: func(f *ReleaseFacts) { f.Versioning = false },
	}, {
		name: "Isolation", clear: func(f *ReleaseFacts) { f.Isolation = false },
	}, {
		name: "Quota", clear: func(f *ReleaseFacts) { f.Quota = false },
	}, {
		name: "Usage", clear: func(f *ReleaseFacts) { f.Usage = false },
	}, {
		name: "Billing", clear: func(f *ReleaseFacts) { f.Billing = false },
	}}
	for _, axis := range axes {
		f := all
		axis.clear(&f)
		if axis.name == "Billing" {
			if CanRelease(f, true) {
				t.Fatalf("paid release passed without %s", axis.name)
			}
			continue
		}
		if CanRelease(f, false) || CanRelease(f, true) {
			t.Fatalf("release passed without %s", axis.name)
		}
	}
}
