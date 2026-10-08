package craft

// T15 (#130): the four-check web promotion policy as pure rules. The four
// acceptance assertions these tests pin:
//  1. each check records passed/failed/not_run independently
//  2. preview reachability cannot substitute for the actual page load
//  3. any failed or not-run fact leaves the version out of the default seat
//  4. the promotion evidence binds Run, Workspace revision and Version
import (
	"strings"
	"testing"
)

func webAllPassedEvidence() WebCheckEvidence {
	return WebCheckEvidence{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckPassed}
}

// WebPromotionRecord.Validate binds the three identities before any store is
// involved: run, non-negative revision and a well-formed derived version id.
func TestWebPromotionRecordValidateBindsRunRevisionVersion(t *testing.T) {
	full := WebPromotionRecord{RunID: "run-1", Revision: 3, VersionID: VersionID("ws-1", "run-1", "d"), WebCheckEvidence: webAllPassedEvidence()}
	if err := full.Validate(); err != nil {
		t.Fatalf("complete record must validate: %v", err)
	}
	if !full.Promotable() {
		t.Fatal("four passed checks are promotable")
	}
	for name, mutate := range map[string]func(WebPromotionRecord) WebPromotionRecord{
		"missing run":       func(r WebPromotionRecord) WebPromotionRecord { r.RunID = ""; return r },
		"negative revision": func(r WebPromotionRecord) WebPromotionRecord { r.Revision = -1; return r },
		"malformed version": func(r WebPromotionRecord) WebPromotionRecord { r.VersionID = "ver_nothex"; return r },
	} {
		if err := mutate(full).Validate(); err == nil {
			t.Fatalf("%s must fail validation", name)
		}
		if mutate(full).Promotable() {
			t.Fatalf("%s must not be promotable", name)
		}
	}
}

// Each of the four facts refuses promotion on its own — failed AND not_run,
// on every axis. Reachability passing never substitutes for the page load.
func TestWebPromotionRecordRefusesAnyNonPassedFact(t *testing.T) {
	notMotable := []WebCheckEvidence{
		{Build: WebCheckNotRun, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckPassed},
		{Build: WebCheckFailed, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckPassed},
		{Build: WebCheckPassed, Entry: WebCheckNotRun, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckPassed},
		{Build: WebCheckPassed, Entry: WebCheckFailed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckPassed},
		{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckNotRun, PageLoaded: WebCheckPassed},
		{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckFailed, PageLoaded: WebCheckPassed},
		// The ticket's headline substitution: reachable but never loaded.
		{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckNotRun},
		{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckFailed},
	}
	for i, e := range notMotable {
		r := WebPromotionRecord{RunID: "run-1", Revision: 1, VersionID: VersionID("ws-1", "run-1", "d"), WebCheckEvidence: e}
		if r.Promotable() {
			t.Fatalf("case %d: %+v must not be promotable", i, e)
		}
	}
}

// WebChecks renders exactly four independent checks whose statuses mirror the
// evidence outcomes and whose details carry the run/revision/version binding.
func TestWebChecksRenderFourIndependentBoundChecks(t *testing.T) {
	evidence := WebCheckEvidence{Build: WebCheckPassed, Entry: WebCheckFailed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckNotRun}
	record := WebPromotionRecord{RunID: "run-9", Revision: 4, VersionID: VersionID("ws-1", "run-9", "d"), WebCheckEvidence: evidence}
	checks := WebChecks(record)
	if len(checks) != 4 {
		t.Fatalf("expected four checks, got %d", len(checks))
	}
	want := map[string]string{
		CheckBuild:            CheckPassed,
		CheckEntry:            CheckFailed,
		CheckPreviewReachable: CheckPassed,
		CheckPageLoad:         CheckNotRun,
	}
	binding := "run run-9"
	revisionBinding := "revision 4"
	for _, c := range checks {
		status, ok := want[c.Name]
		if !ok {
			t.Fatalf("unexpected check %q", c.Name)
		}
		delete(want, c.Name)
		if c.Status != status {
			t.Fatalf("check %s: status %s, want %s", c.Name, c.Status, status)
		}
		if !strings.Contains(c.Detail, binding) || !strings.Contains(c.Detail, revisionBinding) || !strings.Contains(c.Detail, record.VersionID) {
			t.Fatalf("check %s detail %q does not bind run/revision/version", c.Name, c.Detail)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing checks: %v", want)
	}
}

// WebEvidenceFromChecks derives the four outcomes from a version's recorded
// checks; absent or malformed entries stay not_run, never guessed.
func TestWebEvidenceFromChecksDerivesOutcomes(t *testing.T) {
	derived := WebEvidenceFromChecks([]Check{
		{Name: CheckBuild, Status: CheckPassed},
		{Name: CheckEntry, Status: CheckFailed},
		{Name: CheckPreviewReachable, Status: CheckPassed},
		{Name: "preview", Status: CheckPassed}, // legacy W02 verdict must not feed page load
		{Name: CheckPageLoad, Status: "maybe"}, // malformed status stays not_run
	})
	if derived.Build != WebCheckPassed || derived.Entry != WebCheckFailed || derived.PreviewReachable != WebCheckPassed || derived.PageLoaded != WebCheckNotRun {
		t.Fatalf("derived evidence mismatch: %+v", derived)
	}
	empty := WebEvidenceFromChecks(nil)
	if empty != (WebCheckEvidence{Build: WebCheckNotRun, Entry: WebCheckNotRun, PreviewReachable: WebCheckNotRun, PageLoaded: WebCheckNotRun}) {
		t.Fatalf("absent checks must derive all not_run: %+v", empty)
	}
	// Round trip: WebChecks output derives back to the same evidence.
	record := WebPromotionRecord{RunID: "r", Revision: 1, VersionID: VersionID("w", "r", "d"), WebCheckEvidence: webAllPassedEvidence()}
	if got := WebEvidenceFromChecks(WebChecks(record)); got != record.WebCheckEvidence {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

// SelectDefaultVersion keeps the prior default whenever the newer versions
// are not four-check ready — newest-first input, first ready wins.
func TestSelectDefaultVersionKeepsPriorDefaultUntilReady(t *testing.T) {
	ready := func(id string) Version {
		return Version{ID: id, Kind: KindWeb, Checks: WebChecks(WebPromotionRecord{RunID: "r", Revision: 1, VersionID: id, WebCheckEvidence: webAllPassedEvidence()})}
	}
	notReady := func(id string, evidence WebCheckEvidence) Version {
		return Version{ID: id, Kind: KindWeb, Checks: WebChecks(WebPromotionRecord{RunID: "r", Revision: 1, VersionID: id, WebCheckEvidence: evidence})}
	}
	unverified := Version{ID: "ver_legacy", Kind: KindWeb, Checks: BuildChecks(KindWeb, []File{{Path: "index.html"}}, ArtifactEvidence{})}

	// Newest first: an unverified newer version never displaces the old default.
	got, ok := SelectDefaultVersion([]Version{notReady("ver_new", WebCheckEvidence{Build: WebCheckPassed, Entry: WebCheckPassed, PreviewReachable: WebCheckPassed, PageLoaded: WebCheckNotRun}), ready("ver_old")})
	if !ok || got.ID != "ver_old" {
		t.Fatalf("not-run page load must keep the prior default, got %+v ok=%v", got, ok)
	}
	// A newer ready version becomes the default.
	got, ok = SelectDefaultVersion([]Version{ready("ver_new"), ready("ver_old")})
	if !ok || got.ID != "ver_new" {
		t.Fatalf("newest ready version must win, got %+v ok=%v", got, ok)
	}
	// Only unverified/legacy versions: no default at all — a generated file
	// alone is not a usable page.
	if _, ok := SelectDefaultVersion([]Version{unverified, notReady("ver_failed", WebCheckEvidence{Build: WebCheckFailed, Entry: WebCheckFailed, PreviewReachable: WebCheckNotRun, PageLoaded: WebCheckNotRun})}); ok {
		t.Fatal("no four-check ready version must yield no default")
	}
	if _, ok := SelectDefaultVersion(nil); ok {
		t.Fatal("empty history must yield no default")
	}
}

// The T04 build evidence and the entry manifest project onto the web gate's
// independent outcomes without fabricating facts.
func TestWebOutcomeProjections(t *testing.T) {
	if got := WebBuildOutcome(ArtifactEvidence{}); got != WebCheckNotRun {
		t.Fatalf("unobserved build must stay not_run, got %s", got)
	}
	if got := WebBuildOutcome(ArtifactEvidence{BuildRan: true, BuildExitCode: 0}); got != WebCheckPassed {
		t.Fatalf("build exit 0 must pass, got %s", got)
	}
	if got := WebBuildOutcome(ArtifactEvidence{BuildRan: true, BuildExitCode: 2}); got != WebCheckFailed {
		t.Fatalf("build exit 2 must fail, got %s", got)
	}
	files := []File{{Path: "index.html"}}
	if got := WebEntryOutcome(KindWeb, files); got != WebCheckPassed {
		t.Fatalf("entry present must pass, got %s", got)
	}
	if got := WebEntryOutcome(KindWeb, nil); got != WebCheckFailed {
		t.Fatalf("missing entry must fail, got %s", got)
	}
}

// WebPromotionRequest.Validate refuses incomplete or malformed callbacks.
func TestWebPromotionRequestValidate(t *testing.T) {
	full := WebPromotionRequest{WorkspaceID: "ws-1", RunID: "run-1", CandidateID: "cand_" + strings.Repeat("a", 16), Revision: 2}
	if err := full.Validate(); err != nil {
		t.Fatalf("complete request must validate: %v", err)
	}
	for name, mutate := range map[string]func(WebPromotionRequest) WebPromotionRequest{
		"no workspace":  func(r WebPromotionRequest) WebPromotionRequest { r.WorkspaceID = ""; return r },
		"no run":        func(r WebPromotionRequest) WebPromotionRequest { r.RunID = ""; return r },
		"no candidate":  func(r WebPromotionRequest) WebPromotionRequest { r.CandidateID = ""; return r },
		"bad candidate": func(r WebPromotionRequest) WebPromotionRequest { r.CandidateID = "ver_x"; return r },
		"negative rev":  func(r WebPromotionRequest) WebPromotionRequest { r.Revision = -1; return r },
	} {
		if err := mutate(full).Validate(); err == nil {
			t.Fatalf("%s must fail validation", name)
		}
	}
}
