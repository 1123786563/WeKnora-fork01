package service

// T13 (#133) — the restricted derived-export consent journey at the highest
// artifact service seam. A Run promotes versions with pinned evidence; the
// export consent service classifies each version's derived members from
// their RECORDED origins, shows the owner the exact manifest, records owner
// decisions bound to the Version ID + Export Manifest digest, and the
// consent-gated bundle projection withholds every restricted derived byte
// unless a live approved decision of the CURRENT owner binds exactly that
// manifest. Pins the acceptance matrix:
//
//   - derived files are classified server-side from recorded origins;
//   - consent binds Version ID and manifest digest, owner-only;
//   - without valid consent no restricted derived byte rides the bundle;
//   - an explicit rejection yields the optional safe bundle (restricted
//     derived files withheld, citation manifest intact);
//   - a changed manifest (another version, another digest) invalidates the
//     old consent and replaying it is a proven conflict;
//   - restricted originals stay excluded and every decision, proven refusal
//     and withholding is audited.
import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCraftT13Journey(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u-owner", SessionID: "s-t13"}
	const workspaceID = "ws-t13"

	versionStore := newT07EvidenceVersionStore()
	memberEvidence := t12MemberEvidenceStore{inner: versionStore}

	db, err := gorm.Open(sqlite.Open("file:craft107_t13_journey?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&craftExportDecisionRow{}))
	require.NoError(t, db.Exec(`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)
	// The sessions row is the durable owner fact the consent reads: the
	// CURRENT owner alone can hold live export authority.
	require.NoError(t, db.Exec(`CREATE TABLE sessions (id text primary key, tenant_id integer, user_id text)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('s-t13', 1, 'u-owner')`).Error)

	checker := &t10RoleChecker{roles: map[string]craft.TaskRole{
		"u-owner":  craft.TaskRoleOwner,
		"u-collab": craft.TaskRoleCollaborator,
		"u-viewer": craft.TaskRoleViewer,
	}}
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	consentSvc, err := NewCraftExportConsentService(CraftExportConsentConfig{
		DB: db, Versions: versionStore, Evidence: memberEvidence,
		TaskAccess: checker, Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	exportSvc, err := NewCraftExportService(CraftExportConfig{
		DB: db, Versions: versionStore, Evidence: memberEvidence,
		TaskAccess: checker, Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	gated, err := NewConsentGatedExportService(exportSvc, consentSvc)
	require.NoError(t, err)

	publish := func(runID string, files []craft.File, evidenceFor func(versionID string) craft.VersionEvidence) craft.Version {
		t.Helper()
		digest, derr := craft.ManifestDigest(files)
		require.NoError(t, derr)
		versionID := craft.VersionID(workspaceID, runID, digest)
		version := craft.Version{
			ID: versionID, WorkspaceID: workspaceID,
			RunID: runID, Kind: craft.KindWeb, Files: files,
			Checks: []craft.Check{{Name: craft.CheckBuild, Status: craft.CheckPassed}},
		}
		_, err := versionStore.PublishWithEvidence(ctx, scope, version, evidenceFor(versionID))
		require.NoError(t, err)
		return version
	}
	acquired := time.Date(2026, 9, 26, 1, 2, 3, 0, time.UTC)
	mixedEvidence := func(runID string) func(string) craft.VersionEvidence {
		return func(versionID string) craft.VersionEvidence {
			return craft.VersionEvidence{
				VersionID: versionID, RunID: runID,
				RequestDigest: t07Digest("req-" + runID), PackageDigest: t07Digest("pkg-" + runID),
				AcquiredAt: acquired, PinnedAt: acquired.Add(time.Hour),
				Sources: []craft.KnowledgeSourceRecord{
					{ID: craft.KnowledgeCitationID("kb-own", "k-own", "c-"+runID), Ref: craft.KnowledgeRef("kb-own", "k-own", "c-"+runID),
						Digest: t07Digest("own-" + runID), TenantID: 1, AcquiredAt: acquired, ExcerptBytes: 32},
					{ID: craft.KnowledgeCitationID("kb-shared", "k-shared", "c-"+runID), Ref: craft.KnowledgeRef("kb-shared", "k-shared", "c-"+runID),
						Digest: t07Digest("shared-" + runID), TenantID: 7, AcquiredAt: acquired.Add(time.Minute), ExcerptBytes: 32},
				},
			}
		}
	}
	ownOnlyEvidence := func(runID string) func(string) craft.VersionEvidence {
		inner := mixedEvidence(runID)
		return func(versionID string) craft.VersionEvidence {
			evidence := inner(versionID)
			evidence.Sources = evidence.Sources[:1]
			return evidence
		}
	}
	// v1 derives from restricted (cross-tenant) sources; v2 rewrites every
	// member with a different manifest; v-clean derives from own-tenant only.
	v1 := publish("run-1", []craft.File{
		{Path: "index.html", Ref: "obj-v1-index", SHA256: t07Digest("v1-index"), Bytes: 12},
		{Path: "citations.json", Ref: "obj-v1-cit", SHA256: t07Digest("v1-cit"), Bytes: 13},
	}, mixedEvidence("run-1"))
	v2 := publish("run-2", []craft.File{
		{Path: "index.html", Ref: "obj-v2-index", SHA256: t07Digest("v2-index"), Bytes: 14},
		{Path: "citations.json", Ref: "obj-v2-cit", SHA256: t07Digest("v2-cit"), Bytes: 15},
	}, mixedEvidence("run-2"))
	vClean := publish("run-clean", []craft.File{
		{Path: "index.html", Ref: "obj-clean-index", SHA256: t07Digest("clean-index"), Bytes: 12},
	}, ownOnlyEvidence("run-clean"))

	// The full (ungated) manifest digest of v1 — what consent binds to.
	full1, err := exportSvc.ExportBundle(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	digest1 := full1.Manifest.ManifestDigest

	auditRows := func(action string) []map[string]any {
		var rows []map[string]any
		require.NoError(t, db.Raw("SELECT tenant_id, actor_user_id, scope_id, target_id, details FROM audit_logs WHERE action = ?", action).Scan(&rows).Error)
		return rows
	}

	// ------------------------------------------------------------------------
	// Phase 0 — the consent view: the owner sees the exact files, origins
	// and the restricted classification before any download.
	// ------------------------------------------------------------------------
	view, err := consentSvc.ExportConsentView(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, v1.ID, view.Manifest.VersionID)
	require.Equal(t, digest1, view.Manifest.ManifestDigest, "the view binds the exact export manifest digest")
	require.Equal(t, craft.ExportConsentAwaiting, view.State)
	require.Nil(t, view.Decision, "no decision exists yet")

	// TaskRead authorizes the requested scope, while the service must still
	// bind that scope to the authenticated caller before reading consent.
	mismatchedCallerCtx := types.WithCaller(craftKnowledgeCtx(scope), types.Caller{TenantID: 7, UserID: "u-other"})
	_, err = consentSvc.ExportConsentView(mismatchedCallerCtx, scope, v1.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)
	callerDenials := auditRows("craft.export_denied:caller_identity")
	require.Len(t, callerDenials, 1, "a caller identity mismatch is audited")
	require.Equal(t, int64(7), callerDenials[0]["tenant_id"], "denial audit attributes the authenticated caller tenant")
	require.Equal(t, "u-other", callerDenials[0]["actor_user_id"], "denial audit attributes the authenticated caller")
	require.Equal(t, "s-t13", callerDenials[0]["scope_id"], "denial audit retains the requested Task as its target scope")
	require.Equal(t, v1.ID, callerDenials[0]["target_id"])
	require.ElementsMatch(t, []string{"index.html", "citations.json"}, view.RestrictedDerived,
		"both members derive from the restricted source and are classified restricted derived")
	for _, f := range view.Manifest.Files {
		restrictedOrigin := false
		for _, origin := range f.Origins {
			if origin.Restricted {
				restrictedOrigin = true
			}
		}
		require.Equal(t, craft.DerivedFileRestricted(f), restrictedOrigin, "the classification reads the recorded origins")
	}
	// Restricted originals never ride the manifest: members are generated
	// files only, and every original appears as an opaque authenticated ref.
	for _, f := range view.Manifest.Files {
		require.False(t, f.Restricted, "no original knowledge material enters the bundle")
		for _, origin := range f.Origins {
			require.NotContains(t, origin.Ref, "http", "origins stay opaque authenticated refs")
		}
	}

	// A read member (viewer) sees the same typed view.
	viewerScope := scope
	viewerScope.UserID = "u-viewer"
	viewerView, err := consentSvc.ExportConsentView(craftKnowledgeCtx(viewerScope), viewerScope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentAwaiting, viewerView.State)

	// An unrestricted version needs no consent at all.
	cleanView, err := consentSvc.ExportConsentView(craftKnowledgeCtx(scope), scope, vClean.ID)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentNone, cleanView.State)
	require.Empty(t, cleanView.RestrictedDerived)
	cleanBundle, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, vClean.ID)
	require.NoError(t, err)
	require.Len(t, cleanBundle.Version.Files, 1, "an unrestricted version's bundle flows whole without any consent")

	// ------------------------------------------------------------------------
	// Phase 1 — without consent no restricted derived byte is returned: the
	// gated bundle withholds both members and carries the safe manifest.
	// ------------------------------------------------------------------------
	safeBundle, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Empty(t, safeBundle.Version.Files, "every restricted derived member is withheld")
	require.Empty(t, safeBundle.Manifest.Files)
	require.NotEqual(t, digest1, safeBundle.Manifest.ManifestDigest, "the safe bundle carries its own digest")
	require.NoError(t, safeBundle.Manifest.Validate())
	require.Len(t, safeBundle.CitationManifest.Sources, 2, "the citation manifest still cites honestly")
	require.Len(t, auditRows("craft.export_derived_withheld"), 1, "the withholding is audited")

	// A viewer downloads the same safe projection.
	viewerSafe, err := gated.ExportBundle(craftKnowledgeCtx(viewerScope), viewerScope, v1.ID)
	require.NoError(t, err)
	require.Empty(t, viewerSafe.Version.Files)

	// A non-member is refused by the underlying task gate.
	strangerScope := scope
	strangerScope.UserID = "u-stranger"
	_, err = gated.ExportBundle(craftKnowledgeCtx(strangerScope), strangerScope, v1.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)

	// ------------------------------------------------------------------------
	// Phase 2 — only the current owner can decide; a collaborator's consent
	// attempt is a proven audited refusal.
	// ------------------------------------------------------------------------
	collabScope := scope
	collabScope.UserID = "u-collab"
	_, err = consentSvc.DecideExport(craftKnowledgeCtx(collabScope), collabScope, v1.ID, craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrForbidden, "only the Task Owner holds the export decision")
	// The proven refusal is audited. The task_access denial vocabulary is
	// shared with the inner T12 export gate (the stranger's download
	// refusal above), so pin the collaborator's row by its actor and the
	// attempted decision it carries.
	refusals := auditRows("craft.export_denied:task_access")
	collabRefusal := false
	for _, row := range refusals {
		if row["actor_user_id"] == "u-collab" && strings.Contains(fmt.Sprint(row["details"]), "attempted_decision") {
			collabRefusal = true
		}
	}
	require.True(t, collabRefusal, "the collaborator's proven refusal is audited: %v", refusals)
	var count int64
	require.NoError(t, db.Model(&craftExportDecisionRow{}).Where("tenant_id = ? AND session_id = ?", scope.TenantID, scope.SessionID).Count(&count).Error)
	require.Zero(t, count, "no decision row was persisted for the refused attempt")

	// The caller identity must match the scope.
	decideMismatchCtx := types.WithCaller(craftKnowledgeCtx(scope), types.Caller{TenantID: 7, UserID: "u-other"})
	_, err = consentSvc.DecideExport(decideMismatchCtx, scope, v1.ID, craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Len(t, auditRows("craft.export_denied:caller_identity"), 1)

	// ------------------------------------------------------------------------
	// Phase 3 — rejection produces the optional safe bundle.
	// ------------------------------------------------------------------------
	declined, err := consentSvc.DecideExport(craftKnowledgeCtx(scope), scope, v1.ID, craft.DecisionRejected, digest1)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentDeclined, declined.State)
	require.NotNil(t, declined.Decision)
	require.Equal(t, craft.DecisionRejected, declined.Decision.Decision)
	require.Equal(t, digest1, declined.Decision.ManifestDigest, "the decision binds the exact manifest digest")
	require.Equal(t, "u-owner", declined.Decision.OwnerID)
	require.Len(t, auditRows("craft.export_decision_recorded"), 1, "the decision is audited")

	rejectedBundle, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Empty(t, rejectedBundle.Version.Files, "a rejection yields the safe bundle without restricted derived files")

	// The decision survives a restart: a fresh service over the same store
	// sees the same declined state.
	fresh, err := NewCraftExportConsentService(CraftExportConsentConfig{
		DB: db, Versions: versionStore, Evidence: memberEvidence,
		TaskAccess: checker, Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	freshView, err := fresh.ExportConsentView(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentDeclined, freshView.State)

	// ------------------------------------------------------------------------
	// Phase 4 — the owner's approval unlocks the full bundle; a fresh
	// decision upserts over the rejection.
	// ------------------------------------------------------------------------
	consented, err := consentSvc.DecideExport(craftKnowledgeCtx(scope), scope, v1.ID, craft.DecisionApproved, digest1)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentConsented, consented.State)

	grantedBundle, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Len(t, grantedBundle.Version.Files, 2, "the live consent unlocks the restricted derived members")
	require.Equal(t, digest1, grantedBundle.Manifest.ManifestDigest, "the consented bundle carries the exact bound manifest")
	require.ElementsMatch(t, []string{"index.html", "citations.json"}, pathsOf(grantedBundle))

	// The consented state is visible to every member.
	viewerGranted, err := gated.ExportBundle(craftKnowledgeCtx(viewerScope), viewerScope, v1.ID)
	require.NoError(t, err)
	require.Len(t, viewerGranted.Version.Files, 2)

	// A former owner's consent stops granting the moment the task's owner
	// row moves: authority follows the CURRENT owner only.
	require.NoError(t, db.Exec(`UPDATE sessions SET user_id = 'u-new-owner' WHERE id = 's-t13'`).Error)
	afterTransfer, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Empty(t, afterTransfer.Version.Files, "the former owner's decision no longer grants restricted derived bytes")
	transferView, err := consentSvc.ExportConsentView(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentAwaiting, transferView.State)
	require.Nil(t, transferView.Decision, "a non-binding decision never projects as authority")
	require.NoError(t, db.Exec(`UPDATE sessions SET user_id = 'u-owner' WHERE id = 's-t13'`).Error)

	// ------------------------------------------------------------------------
	// Phase 5 — a changed manifest invalidates the old consent: v2's digest
	// differs, v1's approval grants nothing there, and replaying v1's
	// digest against v2 is a proven conflict.
	// ------------------------------------------------------------------------
	full2, err := exportSvc.ExportBundle(craftKnowledgeCtx(scope), scope, v2.ID)
	require.NoError(t, err)
	require.NotEqual(t, digest1, full2.Manifest.ManifestDigest)
	v2Bundle, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, v2.ID)
	require.NoError(t, err)
	require.Empty(t, v2Bundle.Version.Files, "v1's consent cannot broaden to v2's manifest")

	v2View, err := consentSvc.ExportConsentView(craftKnowledgeCtx(scope), scope, v2.ID)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentAwaiting, v2View.State, "v2 awaits its own decision")

	// Replay: deciding v2 with v1's digest is the proven conflict.
	_, err = consentSvc.DecideExport(craftKnowledgeCtx(scope), scope, v2.ID, craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Len(t, auditRows("craft.export_denied:manifest_digest"), 1, "the replay signal is audited")
	// Nothing was recorded for v2.
	var v2Rows int64
	require.NoError(t, db.Model(&craftExportDecisionRow{}).Where("version_id = ?", v2.ID).Count(&v2Rows).Error)
	require.Zero(t, v2Rows)

	// A decision without the digest is refused before anything is read.
	_, err = consentSvc.DecideExport(craftKnowledgeCtx(scope), scope, v2.ID, craft.DecisionApproved, "")
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = consentSvc.DecideExport(craftKnowledgeCtx(scope), scope, v2.ID, craft.DecisionStatus("bogus"), full2.Manifest.ManifestDigest)
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// The owner consents to v2's exact manifest and the full bundle flows.
	v2Consented, err := consentSvc.DecideExport(craftKnowledgeCtx(scope), scope, v2.ID, craft.DecisionApproved, full2.Manifest.ManifestDigest)
	require.NoError(t, err)
	require.Equal(t, craft.ExportConsentConsented, v2Consented.State)
	v2Granted, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, v2.ID)
	require.NoError(t, err)
	require.Len(t, v2Granted.Version.Files, 2)
	// v1 keeps its own live consent — decisions are per-version facts.
	v1Still, err := gated.ExportBundle(craftKnowledgeCtx(scope), scope, v1.ID)
	require.NoError(t, err)
	require.Len(t, v1Still.Version.Files, 2)

	// ------------------------------------------------------------------------
	// Phase 6 — hostile inputs stay stable and non-leaking.
	// ------------------------------------------------------------------------
	for _, hostile := range []string{"../../etc/passwd", "v1", ""} {
		_, err = consentSvc.ExportConsentView(craftKnowledgeCtx(scope), scope, hostile)
		require.Error(t, err, "hostile version id %q must be refused", hostile)
	}
	// An unknown version answers missing without leaking.
	_, err = consentSvc.ExportConsentView(craftKnowledgeCtx(scope), scope, "ver_"+strings.Repeat("a", 64))
	require.ErrorIs(t, err, craft.ErrNotFound)
}

// pathsOf lists one bundle's member paths.
func pathsOf(bundle CraftExportBundle) []string {
	paths := make([]string, 0, len(bundle.Version.Files))
	for _, f := range bundle.Version.Files {
		paths = append(paths, f.Path)
	}
	return paths
}

// TestCraftT13ConsentGateFailsClosed pins the assembly discipline: a nil
// inner projector or nil consent service refuses every request instead of
// degrading into the ungated T12 projection.
func TestCraftT13ConsentGateFailsClosed(t *testing.T) {
	_, err := NewConsentGatedExportService(nil, nil)
	require.Error(t, err)
}
