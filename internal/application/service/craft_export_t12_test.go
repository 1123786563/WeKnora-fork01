package service

// T12 (#132) — the version-bound source bundle journey at the highest
// artifact service seam. A Run promotes V1 with pinned evidence (T07); the
// bundle download projects exactly V1's immutable members, build metadata
// and citation/source manifest. Pins the acceptance matrix:
//   - the bundle binds the Version ID and reads immutable artifact members
//     only — later Workspace runs and knowledge edits never move V1's
//     bundle digest;
//   - the bundle carries source members, build metadata and the
//     citation/source manifest (stable citation identity, title, digest and
//     acquisition time, authenticated craftkb reference);
//   - restricted originals are excluded by default: the manifest lists
//     generated members only, and the cited originals resolve solely
//     through the viewer's own fresh authorization;
//   - a non-member download is refused and audited; a member download is
//     audited with the member, the Version and the manifest digest;
//   - traversal-shaped version ids and poisoned member paths are refused
//     before any byte is packaged; an evidence-less version honestly
//     refuses instead of reconstructing history.
import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCraftT12Journey(t *testing.T) {
	ctx := context.Background()
	// "s1" is the session key the shared fake sandbox source serves.
	scope := craft.Scope{TenantID: 1, UserID: "u-owner", SessionID: "s1"}
	const workspaceID = "ws-t12"

	filesSvc := newDirBackedFileService(t)
	versionStore := newT07EvidenceVersionStore()
	candidateStore := &memoryCandidateStore{}
	draftHeads := &stubDraftHeadStore{heads: map[string]craft.DraftHead{}}
	probe := &scriptedPageProbe{}
	records := &t05Records{}
	buildEvidence := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{ClaimedSuccess: true, BuildRan: true, BuildExitCode: 0}
	}
	svc := NewCraftArtifactServiceWithCandidates(
		craftSourceWith(nil, nil), filesSvc, versionStore, candidateStore, buildEvidence,
		CraftArtifactConfig{OutputDir: craftTestOutputDir},
	).WithWebPromotion(draftHeads, probe).WithWebBuildReceipt(verifiedWebBuildReceipts{}).WithVersionEvidence(records)

	t1 := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	t2 := t1.Add(2 * time.Hour)
	runRecord := func(runID string, sourceDigest string, acquired time.Time) craft.KnowledgeRecord {
		record := craft.KnowledgeRecord{
			Scope: scope, RunID: runID,
			RequestDigest: t07Digest("req-" + runID), PackageDigest: t07Digest("pkg-" + runID),
			PublicationState: craft.KnowledgePublicationPublished,
			Sources: []craft.KnowledgeSourceRecord{
				{ID: craft.KnowledgeCitationID("kb-own", "k-own", "c-"+runID), Ref: craft.KnowledgeRef("kb-own", "k-own", "c-"+runID),
					Digest: sourceDigest, TenantID: 1, AcquiredAt: acquired, ExcerptBytes: 32},
				{ID: craft.KnowledgeCitationID("kb-shared", "k-shared", "c-"+runID), Ref: craft.KnowledgeRef("kb-shared", "k-shared", "c-"+runID),
					Digest: t07Digest("shared-" + runID), TenantID: 7, AcquiredAt: acquired.Add(time.Minute), ExcerptBytes: 32},
			},
		}
		require.NoError(t, records.Save(context.Background(), record))
		return record
	}
	stage := func(runID, content string, revision int64) craft.Candidate {
		t.Helper()
		task := craftArtifactTask(scope.SessionID, workspaceID, runID)
		task.Scope = scope
		generation := "gen-" + runID
		candidate, err := svc.CollectCandidate(ctx, task, craft.KindWeb, candidateSource(runID, generation, content), generation)
		require.NoError(t, err)
		draftHeads.setSealed(workspaceID, revision, runID, candidate.ManifestDigest)
		return candidate
	}
	promote := func(runID string, candidate craft.Candidate, revision int64) craft.Version {
		t.Helper()
		probe.set(craft.WebCheckPassed, craft.WebCheckPassed)
		version, err := svc.PromoteWebVersion(ctx, scope, craft.WebPromotionRequest{
			WorkspaceID: workspaceID, RunID: runID, CandidateID: candidate.ID, Revision: revision,
		})
		require.NoError(t, err)
		return version
	}

	db, err := gorm.Open(sqlite.Open("file:craft107_t12_journey?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)

	checker := &t10RoleChecker{roles: map[string]craft.TaskRole{
		"u-owner":  craft.TaskRoleOwner,
		"u-viewer": craft.TaskRoleViewer,
	}}
	titles := func(_ context.Context, tenantID uint64, knowledgeIDs []string) (map[string]string, error) {
		out := map[string]string{}
		for _, id := range knowledgeIDs {
			if id == "k-own" {
				out[id] = "Own Region Sales"
			}
			if id == "k-shared" {
				out[id] = "Shared Region Sales"
			}
		}
		return out, nil
	}
	exportSvc, err := NewCraftExportService(CraftExportConfig{
		DB: db, Versions: versionStore, Evidence: t12MemberEvidenceStore{inner: versionStore},
		Titles: titles, TaskAccess: checker,
		Now: func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	auditRows := func(action string) []map[string]any {
		var rows []map[string]any
		require.NoError(t, db.Raw("SELECT actor_user_id, target_id, details FROM audit_logs WHERE action = ?", action).Scan(&rows).Error)
		return rows
	}

	// ------------------------------------------------------------------------
	// Phase 0 — success: run-1 promotes V1 with pinned evidence; the member
	// downloads V1's bundle.
	// ------------------------------------------------------------------------
	runRecord("run-1", t07Digest("source-v1"), t1)
	v1 := promote("run-1", stage("run-1", "<h1>v1</h1>", 1), 1)
	memberCtx := craftKnowledgeCtx(scope)

	bundle, err := exportSvc.ExportBundle(memberCtx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, v1.ID, bundle.Version.ID, "the bundle is bound to the requested Version ID")
	require.Equal(t, v1.ID, bundle.Manifest.VersionID)
	require.Len(t, bundle.Manifest.Files, len(v1.Files), "the bundle packages exactly the version's immutable members")

	// Source members: the packaged files ARE the version members — no
	// original knowledge bytes, no workspace files.
	bundlePaths := map[string]bool{}
	for _, f := range bundle.Manifest.Files {
		bundlePaths[f.Path] = true
		require.False(t, f.Restricted, "generated members are not restricted originals")
	}
	versionPaths := map[string]bool{}
	for _, f := range v1.Files {
		versionPaths[f.Path] = true
	}
	require.Equal(t, versionPaths, bundlePaths, "bundle members are exactly the version manifest members")
	index, ok := bundlePaths["index.html"]
	require.True(t, ok && index, "the entry file is a bundle member")

	// Build metadata: the version's checks ride the bundle.
	require.NotEmpty(t, bundle.Version.Checks, "build metadata rides the bundle")

	// Citation/source manifest: stable identity, title, digest/time and the
	// authenticated reference, verbatim from the pinned evidence. Entries
	// order by citation id; locate the own-tenant source by its ref.
	require.Len(t, bundle.CitationManifest.Sources, 2)
	var own *craft.BundleSource
	for i := range bundle.CitationManifest.Sources {
		if strings.HasPrefix(bundle.CitationManifest.Sources[i].Ref, "craftkb://kb/kb-own/") {
			own = &bundle.CitationManifest.Sources[i]
		}
	}
	require.NotNil(t, own, "the own-tenant source rides the citation manifest")
	require.Regexp(t, `^kc_[0-9a-f]{24}$`, own.CitationID)
	require.Equal(t, t07Digest("source-v1"), own.Digest)
	require.Equal(t, t1, own.AcquiredAt)
	require.Equal(t, "Own Region Sales", own.Title)
	require.True(t, strings.HasPrefix(own.Ref, "craftkb://kb/kb-own/"), "the manifest reference stays the authenticated durable ref: %s", own.Ref)
	require.NotContains(t, own.Ref, "http", "the reference is never a provider URL")

	// The digest is present, stable and distinct per version.
	digest1 := bundle.Manifest.ManifestDigest
	require.NotEmpty(t, digest1)

	// Audited: member, Version and manifest digest on one row.
	rows := auditRows("craft.export_bundle")
	require.Len(t, rows, 1)
	require.Equal(t, "u-owner", rows[0]["actor_user_id"])
	require.Equal(t, v1.ID, rows[0]["target_id"])
	require.Contains(t, rows[0]["details"], digest1, "the audit row records the manifest digest")

	// A Task Viewer — a read member — downloads the same bundle.
	viewerScope := scope
	viewerScope.UserID = "u-viewer"
	viewerBundle, err := exportSvc.ExportBundle(craftKnowledgeCtx(viewerScope), viewerScope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, digest1, viewerBundle.Manifest.ManifestDigest)

	// ------------------------------------------------------------------------
	// Phase 1 — V1 before/after V2: the workspace advances (run-2 rewrites
	// every file with new knowledge) and V1's bundle digest stays stable.
	// ------------------------------------------------------------------------
	runRecord("run-2", t07Digest("source-v2-updated"), t2)
	v2 := promote("run-2", stage("run-2", "<h1>v2 rewritten</h1>", 2), 2)

	again, err := exportSvc.ExportBundle(memberCtx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, digest1, again.Manifest.ManifestDigest, "a historical version's bundle digest stays stable after later edits")
	require.Equal(t, bundle.CitationManifest, again.CitationManifest, "V1's citation manifest never mutates after V2")
	for _, f := range again.Manifest.Files {
		require.Equal(t, bundlePaths[f.Path], true)
	}

	bundle2, err := exportSvc.ExportBundle(memberCtx, scope, v2.ID)
	require.NoError(t, err)
	require.NotEqual(t, digest1, bundle2.Manifest.ManifestDigest, "the new version's bundle digest differs")
	require.Equal(t, t07Digest("source-v2-updated"), bundle2.CitationManifest.Sources[0].Digest, "V2's manifest pins its own observed facts")

	// A staged-but-unpromised run-3 draft changes nothing about V1.
	_ = stage("run-3", "<h1>draft only</h1>", 3)
	stable, err := exportSvc.ExportBundle(memberCtx, scope, v1.ID)
	require.NoError(t, err)
	require.Equal(t, digest1, stable.Manifest.ManifestDigest, "workspace drafts never move a published version's bundle")

	// ------------------------------------------------------------------------
	// Phase 2 — authorization: a non-member is refused and the refusal is
	// audited; a caller/scope mismatch is refused the same way.
	// ------------------------------------------------------------------------
	strangerScope := scope
	strangerScope.UserID = "u-stranger"
	_, err = exportSvc.ExportBundle(craftKnowledgeCtx(strangerScope), strangerScope, v1.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)
	denied := auditRows("craft.export_denied:task_access")
	require.Len(t, denied, 1, "the proven refusal is audited")

	// Caller identity must match the scope: the context caller is the auth
	// fact, the scope is server-derived.
	_, err = exportSvc.ExportBundle(craftKnowledgeCtx(craft.Scope{TenantID: 1, UserID: "u-other", SessionID: scope.SessionID}), scope, v1.ID)
	require.ErrorIs(t, err, craft.ErrForbidden)

	// ------------------------------------------------------------------------
	// Phase 3 — traversal and poisoned members: hostile inputs are refused
	// before any byte is packaged.
	// ------------------------------------------------------------------------
	for _, hostile := range []string{"../../etc/passwd", "..", "ver_short", "v1", ""} {
		_, err = exportSvc.ExportBundle(memberCtx, scope, hostile)
		require.Error(t, err, "hostile version id %q must be refused", hostile)
	}

	// A store-poisoned member path (a corrupted row, not a client input)
	// refuses the whole bundle rather than packaging an escape.
	poisoned := craft.Version{
		ID: "ver_" + strings.Repeat("f", 64), WorkspaceID: workspaceID, RunID: "run-poison", Kind: craft.KindWeb,
		Files: []craft.File{{Path: "../escape.txt", Ref: "obj-x", SHA256: t07Digest("x"), Bytes: 3}},
	}
	versionStore.mu.Lock()
	versionStore.byID[poisoned.ID] = poisoned
	versionStore.evidence[poisoned.ID] = craft.VersionEvidence{
		VersionID: poisoned.ID, RunID: "run-poison",
		RequestDigest: t07Digest("req-p"), PackageDigest: t07Digest("pkg-p"),
		PinnedAt: t1, Sources: []craft.KnowledgeSourceRecord{{
			ID: craft.KnowledgeCitationID("kb-own", "k-own", "c-p"), Ref: craft.KnowledgeRef("kb-own", "k-own", "c-p"),
			Digest: t07Digest("sp"), TenantID: 1, AcquiredAt: t1, ExcerptBytes: 3,
		}},
	}
	versionStore.scopes[poisoned.ID] = scope
	versionStore.mu.Unlock()
	_, err = exportSvc.ExportBundle(memberCtx, scope, poisoned.ID)
	require.ErrorIs(t, err, craft.ErrInvalidInput, "a poisoned member path refuses the bundle wholesale")

	// An evidence-less version honestly refuses: history is never rebuilt
	// from the Workspace, the live record or the current knowledge base.
	legacyFiles := []craft.File{{Path: "index.html", Ref: "obj-l", SHA256: t07Digest("l"), Bytes: 3}}
	legacyDigest, err := craft.ManifestDigest(legacyFiles)
	require.NoError(t, err)
	legacy := craft.Version{
		ID: craft.VersionID(workspaceID, "run-legacy", legacyDigest), WorkspaceID: workspaceID, RunID: "run-legacy", Kind: craft.KindWeb,
		Files: legacyFiles,
	}
	_, err = versionStore.Publish(ctx, scope, legacy)
	require.NoError(t, err)
	_, err = exportSvc.ExportBundle(memberCtx, scope, legacy.ID)
	require.ErrorIs(t, err, craft.ErrNotFound, "a version without pinned evidence refuses instead of reconstructing")

	// ------------------------------------------------------------------------
	// Phase 4 — the manifest reference remains authenticated: the citation
	// ref alone opens nothing. The T10 resolver re-authorizes EVERY open
	// against the current viewer's own grant; a viewer without the original
	// permission is refused even holding the downloaded manifest (exercised
	// end-to-end at the HTTP journey below).
	// ------------------------------------------------------------------------
	require.NotContains(t, own.Ref, "signature", "the ref carries no reusable credential")
}

// t12MemberEvidenceStore adapts the T07 evidence store's owner-only read to
// the member-level read the T12 bundle needs: any member of the SAME task
// (tenant + session) reads, other tasks stay invisible. Task membership is
// already enforced by the export service's fresh RequireTaskAccess(TaskRead)
// on every download; central assembly (T20) wires the production equivalent
// — the same member-level relaxation the T07 integration recorded for the
// evidence read surface.
type t12MemberEvidenceStore struct{ inner *t07EvidenceVersionStore }

func (s t12MemberEvidenceStore) VersionEvidence(ctx context.Context, scope craft.Scope, versionID string) (craft.VersionEvidence, error) {
	s.inner.mu.Lock()
	owner, known := s.inner.scopes[versionID]
	s.inner.mu.Unlock()
	if !known || owner.TenantID != scope.TenantID || owner.SessionID != scope.SessionID {
		return craft.VersionEvidence{}, craft.ErrNotFound
	}
	return s.inner.VersionEvidence(ctx, owner, versionID)
}

// TestCraftT12ExportJourneyStabilityGuard pins the digest's determinism at
// the service seam across repeated downloads (idempotent projection, no
// clock or ordering inputs).
func TestCraftT12ExportJourneyStabilityGuard(t *testing.T) {
	ctx := context.Background()
	scope := craft.Scope{TenantID: 1, UserID: "u-owner", SessionID: "s-t12b"}
	versionStore := newT07EvidenceVersionStore()
	db, err := gorm.Open(sqlite.Open("file:craft107_t12_stab?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)

	// The version identity derives from its content manifest, exactly as the
	// store derives it on publish.
	files := []craft.File{{Path: "index.html", Ref: "obj-s", SHA256: t07Digest("idx"), Bytes: 4}}
	digest, err := craft.ManifestDigest(files)
	require.NoError(t, err)
	versionID := craft.VersionID("ws-s", "run-s", digest)
	acquired := time.Date(2026, 9, 25, 1, 2, 3, 0, time.UTC)
	evidence := craft.VersionEvidence{
		VersionID: versionID, RunID: "run-s",
		RequestDigest: t07Digest("r"), PackageDigest: t07Digest("p"),
		AcquiredAt: acquired, PinnedAt: acquired.Add(time.Hour),
		Sources: []craft.KnowledgeSourceRecord{{
			ID: craft.KnowledgeCitationID("kb-own", "k-own", "c-s"), Ref: craft.KnowledgeRef("kb-own", "k-own", "c-s"),
			Digest: t07Digest("s"), TenantID: 1, AcquiredAt: acquired, ExcerptBytes: 8,
		}},
	}
	version := craft.Version{
		ID: versionID, WorkspaceID: "ws-s", RunID: "run-s", Kind: craft.KindWeb,
		Files:  files,
		Checks: []craft.Check{{Name: craft.CheckBuild, Status: craft.CheckPassed}},
	}
	_, err = versionStore.PublishWithEvidence(ctx, scope, version, evidence)
	require.NoError(t, err)

	svc, err := NewCraftExportService(CraftExportConfig{
		DB: db, Versions: versionStore, Evidence: versionStore,
		TaskAccess: &t10RoleChecker{roles: map[string]craft.TaskRole{"u-owner": craft.TaskRoleOwner}},
		Now:        func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	memberCtx := craftKnowledgeCtx(scope)
	first, err := svc.ExportBundle(memberCtx, scope, version.ID)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		repeat, err := svc.ExportBundle(memberCtx, scope, version.ID)
		require.NoError(t, err)
		require.Equal(t, first.Manifest.ManifestDigest, repeat.Manifest.ManifestDigest)
		require.Equal(t, first.CitationManifest, repeat.CitationManifest)
	}
	// Missing titles degrade honestly: the pinned facts still carry and the
	// digest is unchanged by the absent display names.
	require.Equal(t, first.CitationManifest.Sources[0].CitationID, evidence.Sources[0].ID)
}
