package craft

import "testing"

func TestWebContractFacts(t *testing.T) {
	input := InputRecognition{Accepted: true, Understood: false, Reason: "unrecognized_extension"}
	if err := input.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (InputRecognition{Accepted: false, Understood: true}).Validate(); err == nil {
		t.Fatal("understood input cannot be unaccepted")
	}

	evidence := WebCheckEvidence{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckNotRun}
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
	if evidence.Ready() {
		t.Fatal("page load has not passed")
	}
	evidence.PageLoaded = WebCheckPassed
	if !evidence.Ready() {
		t.Fatal("all four checks passed")
	}
	evidence.PageLoaded = CheckOutcome("maybe")
	if err := evidence.Validate(); err == nil {
		t.Fatal("unknown check outcome accepted")
	}

	for _, state := range []StopOutcomeStatus{StopRequested, StopConfirmed, StopUnknown} {
		if err := (StopOutcome{Status: state, RunID: "run-1"}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if err := (StopOutcome{Status: StopOutcomeStatus("maybe"), RunID: "run-1"}).Validate(); err == nil {
		t.Fatal("unknown stop status accepted")
	}
	for _, state := range []WriterAcquireStatus{WriterAcquired, WriterConflict, WriterUnknown} {
		if err := (WriterAcquireOutcome{Status: state, WorkspaceID: "ws-1"}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if err := (WriterAcquireOutcome{Status: WriterAcquireStatus("maybe"), WorkspaceID: "ws-1"}).Validate(); err == nil {
		t.Fatal("unknown writer status accepted")
	}
}

func TestWebConsentAndBudgetContracts(t *testing.T) {
	if err := (BudgetPause{RunID: "run-1", Reason: "exhausted", Limit: 100, Used: 100}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (BudgetPause{RunID: "run-1", Reason: "exhausted", Limit: -1}).Validate(); err == nil {
		t.Fatal("negative budget accepted")
	}
	if err := (RestrictedContribution{VersionID: "v1", EvidenceDigest: "sha256:a", Restricted: true}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ShareDecision{VersionID: "v1", EvidenceDigest: "sha256:a", OwnerID: "u1", Decision: DecisionApproved}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ShareDecision{VersionID: "v1", EvidenceDigest: "sha256:a", OwnerID: "u1", Decision: DecisionStatus("maybe")}).Validate(); err == nil {
		t.Fatal("unknown share decision accepted")
	}
	manifest := ExportManifest{VersionID: "v1", ManifestDigest: "sha256:m", Files: []ExportFile{{Path: "index.html", SHA256: "sha256:f", Restricted: false}}}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ExportDecision{VersionID: "v1", ManifestDigest: "sha256:m", OwnerID: "u1", Decision: DecisionRejected}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ExportDecision{VersionID: "v1", ManifestDigest: "", OwnerID: "u1", Decision: DecisionApproved}).Validate(); err == nil {
		t.Fatal("unbound export decision accepted")
	}
}
