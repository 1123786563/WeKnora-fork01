package craft

import (
	"fmt"
	"strings"
)

// -----------------------------------------------------------------------------
// T15 (#130): the four-check web version promotion policy.
//
// A web version may take over the default preview seat only after FOUR
// independently observed checks — build success, valid entry, preview
// reachability and actual browser page load — each recorded as its own
// passed/failed/not_run fact. Reachability can never substitute for the load
// fact; any failed or not-run fact keeps the prior default in the seat and
// leaves the generated files a private Workspace draft.
// -----------------------------------------------------------------------------

// Verification check names of the two web-gate facts observed after
// collection. Together with CheckBuild and CheckEntry they are the four
// independent checks of one web promotion; consumers match on these exact
// strings.
const (
	// CheckPreviewReachable reports whether the controlled preview origin
	// answered for this version (T14 probe fact #1).
	CheckPreviewReachable = "preview_reachable"
	// CheckPageLoad reports whether an actual browser loaded the page
	// (T14 probe fact #2). It is separate evidence from reachability.
	CheckPageLoad = "page_load"
)

// WebPromotionRequest names the private candidate a promotion callback wants
// to promote, bound to the Workspace revision the candidate was captured at.
// All identities are server-derived; a callback that does not bind its own
// candidate and revision is refused.
type WebPromotionRequest struct {
	WorkspaceID string
	RunID       string
	CandidateID string
	Revision    int64
}

// Validate refuses incomplete or malformed callbacks before any store is
// touched.
func (r WebPromotionRequest) Validate() error {
	if r.WorkspaceID == "" || r.RunID == "" || r.Revision < 0 {
		return fmt.Errorf("%w: promotion request requires workspace, run and a non-negative revision", ErrInvalidInput)
	}
	if !strings.HasPrefix(r.CandidateID, CandidateIDPrefix) {
		return fmt.Errorf("%w: candidate id %q", ErrInvalidInput, r.CandidateID)
	}
	return nil
}

// WebPromotionRecord is the promotion decision's complete evidence: the four
// independently recorded check outcomes, bound to the producing Run, the
// Workspace revision the candidate was captured at, and the candidate's
// derived Version identity. It is what a promotion persists with the version
// and what a later audit reads back.
type WebPromotionRecord struct {
	RunID     string
	Revision  int64
	VersionID string
	WebCheckEvidence
}

// Validate enforces the three-way binding and the evidence's own validity.
func (r WebPromotionRecord) Validate() error {
	if r.RunID == "" || r.Revision < 0 || !IsVersionID(r.VersionID) {
		return fmt.Errorf("%w: promotion evidence requires run, revision and version binding", ErrInvalidInput)
	}
	return r.WebCheckEvidence.Validate()
}

// Promotable answers whether this record admits promotion: the binding is
// complete and every one of the four checks independently passed. A single
// failed or not_run fact — most importantly a page load that was never
// observed — is not promotable, whatever the other three say.
func (r WebPromotionRecord) Promotable() bool {
	return r.Validate() == nil && r.WebCheckEvidence.Ready()
}

// webCheckBinding renders the provenance string every recorded fact carries,
// so each check row is self-describing evidence: which Run produced it, at
// which Workspace revision, for which candidate Version.
func webCheckBinding(record WebPromotionRecord) string {
	return fmt.Sprintf("run %s, workspace revision %d, version %s", record.RunID, record.Revision, record.VersionID)
}

// WebChecks renders the four checks of one promotion record. Each check
// carries its own outcome (passed/failed/not_run — never inferred from
// another check) and the run/revision/version binding in its detail; the
// rendering is a pure function so an identical replay publishes
// byte-identical checks and adopts the stored version row.
func WebChecks(record WebPromotionRecord) []Check {
	binding := webCheckBinding(record)
	outcome := func(name string, o CheckOutcome, fact string) Check {
		return Check{Name: name, Status: string(o), Detail: fact + "; " + binding}
	}
	return []Check{
		outcome(CheckBuild, record.Build, "build step outcome"),
		outcome(CheckEntry, record.Entry, "entry deliverable outcome"),
		outcome(CheckPreviewReachable, record.PreviewReachable, "controlled preview reachability"),
		outcome(CheckPageLoad, record.PageLoaded, "actual browser page load"),
	}
}

// WebEvidenceFromChecks derives the four web-gate outcomes from a version's
// recorded checks. Absent entries — a legacy three-check row, or a check the
// probe never supplied — derive not_run; a malformed status never feeds the
// evidence. The legacy W02 "preview" verdict deliberately does NOT map onto
// page_loaded: reachability-era evidence cannot promote under T15.
func WebEvidenceFromChecks(checks []Check) WebCheckEvidence {
	evidence := WebCheckEvidence{Build: WebCheckNotRun, Entry: WebCheckNotRun, PreviewReachable: WebCheckNotRun, PageLoaded: WebCheckNotRun}
	for _, c := range checks {
		outcome := CheckOutcome(c.Status)
		if !outcome.valid() {
			continue
		}
		switch c.Name {
		case CheckBuild:
			evidence.Build = outcome
		case CheckEntry:
			evidence.Entry = outcome
		case CheckPreviewReachable:
			evidence.PreviewReachable = outcome
		case CheckPageLoad:
			evidence.PageLoaded = outcome
		}
	}
	return evidence
}

// SelectDefaultVersion picks the workspace's default preview version from
// newest-first versions: the newest version whose four recorded web checks
// each independently passed. Versions without the four-check evidence —
// legacy rows, failed rounds, candidates never actually loaded by a browser
// — are never selected, so a refused or unverified promotion keeps the
// prior default exactly where it was.
func SelectDefaultVersion(versions []Version) (Version, bool) {
	for _, v := range versions {
		if WebEvidenceFromChecks(v.Checks).Ready() {
			return v, true
		}
	}
	return Version{}, false
}

// WebBuildOutcome projects the externally observed build evidence (T04's
// build-log facts) onto the web gate's independent outcome. An unobserved
// build stays not_run — it is never guessed from a claimed success.
func WebBuildOutcome(evidence ArtifactEvidence) CheckOutcome {
	switch {
	case !evidence.BuildRan:
		return WebCheckNotRun
	case evidence.BuildExitCode == 0:
		return WebCheckPassed
	default:
		return WebCheckFailed
	}
}

// WebEntryOutcome projects the entry-deliverable fact of the collected
// manifest onto the web gate's independent outcome, mirroring BuildChecks'
// entry semantics: the kind's entry file present passes; anything else —
// including a claimed success without the entry — fails.
func WebEntryOutcome(kind string, files []File) CheckOutcome {
	entry, defined := EntryPath(kind)
	switch {
	case !defined:
		return WebCheckNotRun
	case hasArtifactFile(files, entry):
		return WebCheckPassed
	default:
		return WebCheckFailed
	}
}

// ReleaseFacts is the release gate's fact sheet: one boolean per capability
// axis the 27-task Craft program committed to, each backed by machine
// evidence collected in docs/testing/craft/release-evidence.json (never a
// hand-filled assertion — scripts/check-craft-release.py verifies the
// evidence file and the artifacts it points at).
//
// The axes map onto the program's pillar tasks:
//   - Web: W01–W06 workbench surfaces (create/run/versions/preview/reconnect)
//   - Permissions: W03/R06 read/write ACLs and interaction approvals
//   - Cancel: W04/O03 run cancellation and lifecycle teardown guards
//   - Reconnect: W06 browser reconnect (refresh/killed-stream, single admission)
//   - Recovery: C04/G3 crash recovery + C05 snapshot restore after real kills
//   - Versioning: W01/W02 immutable versions and per-kind manifests
//   - Isolation: R02/R03 tenant/scope isolation and sandbox binding
//   - Quota: O01/O03 budget admission and call authorization
//   - Usage: O04 usage ledger/views (unknown stays visible, never folded to 0)
//   - Billing: O02 commercial evidence — ONLY meaningful for paid releases
//     and, per the G4 external gap, currently NOT satisfiable (missing
//     commercial_reservations owner column + four AutoMigrate-only tables),
//     so a paid release fails this gate until that side lands.
type ReleaseFacts struct {
	Web, Permissions, Cancel, Reconnect, Recovery, Versioning, Isolation, Quota, Usage, Billing bool
}

// CanRelease answers whether the Craft feature set may ship in the requested
// mode. Every capability axis must hold; a paid (commercial) release
// additionally requires the Billing fact, which only real commercial
// evidence can set. A controlled (unpaid) release is the default posture and
// is never blocked by a missing Billing fact.
func CanRelease(f ReleaseFacts, paid bool) bool {
	return f.Web && f.Permissions && f.Cancel && f.Reconnect && f.Recovery && f.Versioning && f.Isolation && f.Quota && f.Usage && (!paid || f.Billing)
}
