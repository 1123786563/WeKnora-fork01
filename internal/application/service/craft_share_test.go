package service

// T11 (#128): sharing a restricted-source derived result requires an
// explicit, currently-valid owner consent. The journey pins the whole
// contract at the highest service seam:
//
//   - a version whose citation facts bind cross-tenant (organization-shared
//     library) recorded originals is restricted and private by default —
//     no sharing authority exists;
//   - only the CURRENT Task Owner can consent: a Viewer, a Collaborator and
//     a revoked owner are all refused with a fresh TaskShare check, and the
//     refusal is audited;
//   - the owner's approval binds the immutable Version ID and the evidence
//     digest computed server-side from recorded evidence; the response
//     carries exactly that binding and never any original-source material;
//   - changed evidence (a new version with different evidence) unbinds the
//     prior consent, and replaying the old decision is a conflict that
//     creates no authority; the owner can only re-consent against the
//     current evidence;
//   - expiry returns the version to private; an explicit reject declines;
//     revocation kills a live consent inside its TTL;
//   - sharing authority grants no original-source access: every projection
//     is the typed consent summary only, and consent never widens a
//     Viewer's task actions.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// t11VersionStore serves the immutable delivered versions under test.
type t11VersionStore struct{ byID map[string]craft.Version }

func (s t11VersionStore) Get(_ context.Context, _ craft.Scope, id string) (craft.Version, error) {
	v, ok := s.byID[id]
	if !ok {
		return craft.Version{}, fmt.Errorf("%w: version %q", craft.ErrNotFound, id)
	}
	return v, nil
}
func (s t11VersionStore) List(context.Context, craft.Scope) ([]craft.Version, error) {
	return nil, nil
}
func (s t11VersionStore) Publish(_ context.Context, _ craft.Scope, v craft.Version) (craft.Version, error) {
	return v, nil
}

// t11Files serves one stored object per durable ref.
type t11Files struct{ objects map[string][]byte }

func (f t11Files) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	data, ok := f.objects[ref]
	if !ok {
		return nil, fmt.Errorf("%w: object %q", craft.ErrNotFound, ref)
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func TestCraftT11Journey(t *testing.T) {
	// --- the world: one craft task (tenant 1), an owner, a collaborator and
	// a viewer; one own-tenant knowledge original and one organization-
	// shared library original (tenant 7) the Run actually recorded.
	f := newKnowledgeFixture(t, map[string]bool{"kb-shared": true})
	f.seedKB(t, "kb-own", 1)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Region Sales")
	f.seedChunk("kb-own", "k-own", "c-own", "own tenant source excerpt")
	f.seedKB(t, "kb-shared", 7)
	f.seedKnowledge(t, "k-shared", "kb-shared", 7, "Shared Region Sales")
	f.seedChunk("kb-shared", "k-shared", "c-shared", "shared library source excerpt")
	require.NoError(t, f.db.AutoMigrate(&repository.CraftKnowledgeRecordRow{}, &craftShareDecisionRow{}))
	require.NoError(t, f.db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id text PRIMARY KEY, tenant_id integer, user_id text)`).Error)
	require.NoError(t, f.db.Exec(`INSERT OR REPLACE INTO sessions (id, tenant_id, user_id) VALUES ('s-craft', 1, 'u-owner')`).Error)
	require.NoError(t, f.db.Exec(`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)
	records := repository.NewCraftKnowledgeRecordRepository(f.db)

	checker := &t10RoleChecker{roles: map[string]craft.TaskRole{
		"u-owner":        craft.TaskRoleOwner,
		"u-collaborator": craft.TaskRoleCollaborator,
		"u-viewer":       craft.TaskRoleViewer,
	}}
	require.False(t, craft.TaskRoleCollaborator.AllowsTaskAction(craft.TaskShare), "only the Owner holds the share task action")
	owner := craft.Scope{TenantID: 1, UserID: "u-owner", SessionID: "s-craft"}
	viewer := craft.Scope{TenantID: 1, UserID: "u-viewer", SessionID: "s-craft"}
	collaborator := craft.Scope{TenantID: 1, UserID: "u-collaborator", SessionID: "s-craft"}

	base := f.service(t, nil)
	knowledge, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store:  &knowledgeWorkspaceStore{ws: craft.Workspace{ID: "ws-t11", Scope: owner}},
		Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: checker, Records: records, Publisher: newT05Publisher(f.writer),
		Now: func() time.Time { return time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	ownerCtx := craftKnowledgeCtx(owner)
	bundle, err := knowledge.BuildForRun(ownerCtx, owner, "run-t11", "region sales", []string{"k-own", "k-shared"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 2)
	var sharedID, ownID string
	for _, source := range bundle.Sources {
		if source.TenantID == 7 {
			sharedID = source.ID
		} else {
			ownID = source.ID
		}
	}
	require.NotEmpty(t, sharedID, "the Run recorded the organization-shared original")
	require.NotEmpty(t, ownID)

	// A later Run of the same task consumed only own-tenant originals: its
	// versions carry an unrestricted contribution.
	ownOnly, err := knowledge.BuildForRun(ownerCtx, owner, "run-t11-own", "region sales", []string{"k-own"})
	require.NoError(t, err)
	require.Len(t, ownOnly.Sources, 1)

	// The delivered immutable version v-1 cites BOTH originals in its
	// admitted citation manifest; v-2 later changes the evidence.
	manifestBytes := func(ids ...string) []byte {
		entries := make([]craft.WebCitationEntry, 0, len(ids))
		for _, id := range ids {
			entries = append(entries, craft.WebCitationEntry{Kind: craft.WebCitationFact, CitationID: id, Claim: "claim of " + id})
		}
		raw, err := json.Marshal(craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: entries})
		require.NoError(t, err)
		return raw
	}
	files := t11Files{objects: map[string][]byte{}}
	version := func(id, runID string, citationIDs ...string) craft.Version {
		ref := "obj-" + id
		files.objects[ref] = manifestBytes(citationIDs...)
		return craft.Version{ID: id, WorkspaceID: "ws-t11", RunID: runID, Kind: craft.KindWeb, Files: []craft.File{{Path: craft.WebCitationsPath, Ref: ref, SHA256: "sha", MIME: "application/json", Bytes: 4096}}}
	}
	versions := t11VersionStore{byID: map[string]craft.Version{}}
	versions.byID["v-1"] = version("v-1", "run-t11", sharedID, ownID)

	now := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	clock := now
	share, err := NewCraftShareService(CraftShareConfig{
		DB: f.db, Versions: versions, Files: files, Records: records, TaskAccess: checker,
		Now: func() time.Time { return clock },
	})
	require.NoError(t, err)

	// --- 1. restricted result is private by default: the summary the owner
	// must decide on carries the exact version and evidence digest, and no
	// authority exists.
	view, err := share.ShareView(ownerCtx, owner, "v-1")
	require.NoError(t, err)
	require.True(t, view.Contribution.Restricted, "the cross-tenant citation fact marks the version restricted, computed server-side from recorded evidence")
	require.Equal(t, "v-1", view.Contribution.VersionID)
	digest1 := view.Contribution.EvidenceDigest
	require.NotEmpty(t, digest1)
	require.Equal(t, craft.ShareStatePrivate, view.State, "a restricted result is private until the owner decides")
	require.Nil(t, view.Decision)
	contribution, allowed, err := share.ShareAuthority(ownerCtx, owner, "v-1")
	require.NoError(t, err)
	require.False(t, allowed, "no sharing authority without an owner consent")
	require.Equal(t, digest1, contribution.EvidenceDigest)

	// --- 2. only the CURRENT Task Owner can consent; every refusal is a
	// fresh TaskShare check and is audited.
	_, err = share.DecideShare(craftKnowledgeCtx(viewer), viewer, "v-1", craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrForbidden, "a Viewer cannot consent")
	_, err = share.DecideShare(craftKnowledgeCtx(collaborator), collaborator, "v-1", craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrForbidden, "a Collaborator cannot consent")
	delete(checker.roles, "u-owner")
	_, err = share.DecideShare(ownerCtx, owner, "v-1", craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrForbidden, "a revoked owner cannot consent")
	var deniedAudits int64
	require.NoError(t, f.db.Table("audit_logs").Where("action LIKE ? AND outcome = ?", "craft.share_denied:%", "denied").Count(&deniedAudits).Error)
	require.Equal(t, int64(3), deniedAudits, "each non-owner consent attempt is audited")
	checker.roles["u-owner"] = craft.TaskRoleOwner

	// --- 3. the owner consents against the CURRENT evidence; the decision
	// binds the immutable Version ID and evidence digest.
	view, err = share.DecideShare(ownerCtx, owner, "v-1", craft.DecisionApproved, digest1)
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateConsented, view.State)
	require.NotNil(t, view.Decision)
	require.Equal(t, "v-1", view.Decision.VersionID)
	require.Equal(t, digest1, view.Decision.EvidenceDigest)
	require.Equal(t, "u-owner", view.Decision.OwnerID)
	require.Equal(t, craft.DecisionApproved, view.Decision.Decision)
	require.NotNil(t, view.ExpiresAt, "the consent carries its expiry")

	_, allowed, err = share.ShareAuthority(ownerCtx, owner, "v-1")
	require.NoError(t, err)
	require.True(t, allowed, "a live bound owner approval is sharing authority")

	// A Viewer reads the same summary (read is a TaskRead fact) and it never
	// carries original-source material.
	viewerView, err := share.ShareView(craftKnowledgeCtx(viewer), viewer, "v-1")
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateConsented, viewerView.State)
	raw, err := json.Marshal(viewerView)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "craftkb://", "the share projection carries no source ref")
	require.NotContains(t, string(raw), "excerpt", "the share projection carries no excerpt")
	require.NotContains(t, string(raw), "Region Sales", "the share projection carries no original title")
	// Sharing the derived result never widens the Viewer's task actions:
	// open_source stays a per-viewer permission the share does not touch.
	require.True(t, craft.TaskRoleViewer.AllowsTaskAction(craft.TaskOpenSource))
	require.False(t, craft.TaskRoleViewer.AllowsTaskAction(craft.TaskShare))

	var decisionAudits []struct{ Action, Outcome string }
	require.NoError(t, f.db.Table("audit_logs").Where("action = ?", "craft.share_decision_recorded").Order("id").Find(&decisionAudits).Error)
	require.Len(t, decisionAudits, 1)
	require.Equal(t, "success", decisionAudits[0].Outcome)

	// --- 4. changed evidence invalidates the prior consent: version v-2
	// cites different evidence, the old approval does not travel, and
	// replaying the old decision is a conflict.
	versions.byID["v-2"] = version("v-2", "run-t11", sharedID)
	view2, err := share.ShareView(ownerCtx, owner, "v-2")
	require.NoError(t, err)
	digest2 := view2.Contribution.EvidenceDigest
	require.NotEqual(t, digest1, digest2, "different evidence yields a different digest")
	require.Equal(t, craft.ShareStatePrivate, view2.State, "the prior consent does not travel to the new evidence")

	_, allowed, err = share.ShareAuthority(ownerCtx, owner, "v-2")
	require.NoError(t, err)
	require.False(t, allowed, "changed evidence leaves no sharing authority")

	_, err = share.DecideShare(ownerCtx, owner, "v-2", craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrConflict, "replaying a consent recorded for other evidence is a conflict")
	_, allowed, err = share.ShareAuthority(ownerCtx, owner, "v-2")
	require.NoError(t, err)
	require.False(t, allowed, "the replay created no authority")

	// The owner re-consents against the CURRENT evidence of v-2.
	view2, err = share.DecideShare(ownerCtx, owner, "v-2", craft.DecisionApproved, digest2)
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateConsented, view2.State)

	// --- 5. expiry: after the TTL the consent grants nothing.
	clock = now.Add(craft.ShareDecisionTTL + time.Minute)
	view2, err = share.ShareView(ownerCtx, owner, "v-2")
	require.NoError(t, err)
	require.Equal(t, craft.ShareStatePrivate, view2.State, "an expired consent returns the version to private")
	_, allowed, err = share.ShareAuthority(ownerCtx, owner, "v-2")
	require.NoError(t, err)
	require.False(t, allowed, "expiry creates no sharing authority")

	// --- 6. an explicit reject declines sharing.
	view2, err = share.DecideShare(ownerCtx, owner, "v-2", craft.DecisionRejected, digest2)
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateDeclined, view2.State)
	require.NotNil(t, view2.Decision)
	require.Equal(t, craft.DecisionRejected, view2.Decision.Decision)
	_, allowed, err = share.ShareAuthority(ownerCtx, owner, "v-2")
	require.NoError(t, err)
	require.False(t, allowed, "a rejected decision creates no sharing authority")

	// --- 7. revocation kills the live v-1 consent inside its TTL.
	clock = now.Add(time.Hour) // v-1's consent (decided at now) is live again
	view1, err := share.ShareView(ownerCtx, owner, "v-1")
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateConsented, view1.State, "v-1 keeps its live consent inside the TTL")
	view1, err = share.RevokeShare(ownerCtx, owner, "v-1")
	require.NoError(t, err)
	require.Equal(t, craft.ShareStatePrivate, view1.State)
	_, allowed, err = share.ShareAuthority(ownerCtx, owner, "v-1")
	require.NoError(t, err)
	require.False(t, allowed, "a revoked consent is dead even inside its TTL")
	// Revocation is durable: re-reading keeps no authority, and the revoked
	// row can never grant again without a fresh owner decision.
	view1, err = share.ShareView(ownerCtx, owner, "v-1")
	require.NoError(t, err)
	require.Equal(t, craft.ShareStatePrivate, view1.State)
	var revokedAudits int64
	require.NoError(t, f.db.Table("audit_logs").Where("action = ? AND outcome = ?", "craft.share_revoked", "success").Count(&revokedAudits).Error)
	require.Equal(t, int64(1), revokedAudits, "the revocation is audited")
	// Non-owner revocation is refused like consent.
	_, err = share.RevokeShare(craftKnowledgeCtx(viewer), viewer, "v-1")
	require.ErrorIs(t, err, craft.ErrForbidden)

	// --- 8. an unrestricted version needs no consent and holds authority by
	// default while still projecting zero source material.
	versions.byID["v-own"] = version("v-own", "run-t11-own", ownID)
	own, err := share.ShareView(ownerCtx, owner, "v-own")
	require.NoError(t, err)
	require.False(t, own.Contribution.Restricted)
	require.Equal(t, craft.ShareStateConsented, own.State, "an unrestricted result carries no consent requirement")
	_, allowed, err = share.ShareAuthority(ownerCtx, owner, "v-own")
	require.NoError(t, err)
	require.True(t, allowed)

	// --- 9. malformed consent inputs never become authority.
	_, err = share.DecideShare(ownerCtx, owner, "v-1", craft.DecisionUnknown, digest1)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = share.DecideShare(ownerCtx, owner, "v-1", craft.DecisionApproved, "")
	require.ErrorIs(t, err, craft.ErrInvalidInput, "a consent must state the evidence it binds")
	_, err = share.DecideShare(ownerCtx, owner, "v-none", craft.DecisionApproved, digest1)
	require.ErrorIs(t, err, craft.ErrNotFound)
}

// TestCraftShareAssemblyFailsClosed pins the assembly contract: the share
// service refuses to construct without every authority port, and a service
// without a task-ACL answers nothing. Consent authority can never degrade
// into a permissive default.
func TestCraftShareAssemblyFailsClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	complete := CraftShareConfig{
		DB: db, Versions: t11VersionStore{}, Files: t11Files{},
		Records: t11RecordsStub{}, TaskAccess: &t10RoleChecker{},
	}
	_, err = NewCraftShareService(complete)
	require.NoError(t, err, "the complete assembly constructs")

	missing := []CraftShareConfig{
		{DB: nil, Versions: t11VersionStore{}, Files: t11Files{}, Records: t11RecordsStub{}, TaskAccess: &t10RoleChecker{}},
		{DB: db, Versions: nil, Files: t11Files{}, Records: t11RecordsStub{}, TaskAccess: &t10RoleChecker{}},
		{DB: db, Versions: t11VersionStore{}, Files: nil, Records: t11RecordsStub{}, TaskAccess: &t10RoleChecker{}},
		{DB: db, Versions: t11VersionStore{}, Files: t11Files{}, Records: nil, TaskAccess: &t10RoleChecker{}},
		{DB: db, Versions: t11VersionStore{}, Files: t11Files{}, Records: t11RecordsStub{}, TaskAccess: nil},
	}
	for _, cfg := range missing {
		_, err = NewCraftShareService(cfg)
		require.Error(t, err, "an assembly missing one authority port must refuse to construct")
	}
}

// t11RecordsStub answers any load with nothing.
type t11RecordsStub struct{}

func (t11RecordsStub) Load(context.Context, craft.Scope, string) (craft.KnowledgeRecord, error) {
	return craft.KnowledgeRecord{}, craft.ErrNotFound
}

// t11FixedRecords serves the recorded knowledge record per Run.
type t11FixedRecords struct {
	byRun map[string]craft.KnowledgeRecord
}

func (r t11FixedRecords) Load(_ context.Context, _ craft.Scope, runID string) (craft.KnowledgeRecord, error) {
	record, ok := r.byRun[runID]
	if !ok {
		return craft.KnowledgeRecord{}, craft.ErrNotFound
	}
	return record, nil
}

// t11CountingVersions counts Get calls so a code path can prove how many
// times it derived a contribution from the version store.
type t11CountingVersions struct {
	t11VersionStore
	gets *int
}

func (s t11CountingVersions) Get(ctx context.Context, scope craft.Scope, id string) (craft.Version, error) {
	*s.gets++
	return s.t11VersionStore.Get(ctx, scope, id)
}

// TestCraftT11OCRShareRegressions pins the T11 OCR fixes at the service
// seam: a non-restricted version never projects a bound historical
// rejection beside its consented state (F1); digest-replay conflicts and
// caller identity mismatches are audited refusals (F2); repeated identical
// denials are deduplicated inside the shared window while a different
// actor's refusal still records (F4); and the decide hot path derives the
// contribution exactly once, composing its response without a re-read (F6).
func TestCraftT11OCRShareRegressions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:craft107_t11_ocr?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS sessions (id text PRIMARY KEY, tenant_id integer, user_id text)`).Error)
	require.NoError(t, db.Exec(`INSERT OR REPLACE INTO sessions (id, tenant_id, user_id) VALUES ('s-craft', 1, 'u-owner')`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE audit_logs (id integer primary key autoincrement, tenant_id integer, actor_user_id text, action text, scope_type text, scope_id text, target_type text, target_id text, target_user_id text, outcome text, details text, created_at datetime)`).Error)
	require.NoError(t, db.AutoMigrate(&craftShareDecisionRow{}))

	shared := craft.KnowledgeSourceRecord{
		ID: "kc_" + strings.Repeat("a", 24), Ref: "craftkb://kb/k-shared/knowledge/k-s/chunk/c-s",
		Digest: "d-shared", TenantID: 7, ExcerptBytes: 32,
	}
	own := craft.KnowledgeSourceRecord{
		ID: "kc_" + strings.Repeat("b", 24), Ref: "craftkb://kb/k-own/knowledge/k-o/chunk/c-o",
		Digest: "d-own", TenantID: 1, ExcerptBytes: 32,
	}
	scope := craft.Scope{TenantID: 1, UserID: "u-owner", SessionID: "s-craft"}
	records := t11FixedRecords{byRun: map[string]craft.KnowledgeRecord{
		"run-r":   {Scope: scope, RunID: "run-r", PublicationState: craft.KnowledgePublicationPublished, Sources: []craft.KnowledgeSourceRecord{shared, own}},
		"run-own": {Scope: scope, RunID: "run-own", PublicationState: craft.KnowledgePublicationPublished, Sources: []craft.KnowledgeSourceRecord{own}},
	}}
	manifestBytes := func(ids ...string) []byte {
		entries := make([]craft.WebCitationEntry, 0, len(ids))
		for _, id := range ids {
			entries = append(entries, craft.WebCitationEntry{Kind: craft.WebCitationFact, CitationID: id, Claim: "claim of " + id})
		}
		raw, err := json.Marshal(craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: entries})
		require.NoError(t, err)
		return raw
	}
	files := t11Files{objects: map[string][]byte{}}
	version := func(id, runID string, citationIDs ...string) craft.Version {
		ref := "obj-" + id
		files.objects[ref] = manifestBytes(citationIDs...)
		return craft.Version{ID: id, WorkspaceID: "ws-t11", RunID: runID, Kind: craft.KindWeb, Files: []craft.File{{Path: craft.WebCitationsPath, Ref: ref, SHA256: "sha", MIME: "application/json", Bytes: 4096}}}
	}
	versions := t11VersionStore{byID: map[string]craft.Version{}}
	versions.byID["v-r"] = version("v-r", "run-r", shared.ID, own.ID)
	versions.byID["v-x"] = version("v-x", "run-r", shared.ID)
	versions.byID["v-own"] = version("v-own", "run-own", own.ID)

	gets := 0
	checker := &t10RoleChecker{roles: map[string]craft.TaskRole{
		"u-owner": craft.TaskRoleOwner, "u-viewer": craft.TaskRoleViewer, "u-viewer2": craft.TaskRoleViewer,
	}}
	share, err := NewCraftShareService(CraftShareConfig{
		DB: db, Versions: t11CountingVersions{versions, &gets}, Files: files, Records: records, TaskAccess: checker,
		Now: func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	ownerCtx := craftKnowledgeCtx(scope)
	viewer := craft.Scope{TenantID: 1, UserID: "u-viewer", SessionID: "s-craft"}
	viewer2 := craft.Scope{TenantID: 1, UserID: "u-viewer2", SessionID: "s-craft"}
	// The scope names the owner (it passes the TaskShare check) while the
	// context carries someone else's identity: exactly the caller mismatch
	// the service must refuse and audit.
	mismatchCtx := context.WithValue(ownerCtx, types.UserIDContextKey, "u-other")

	// --- F1: an unrestricted version never projects a bound historical
	// rejection beside its consented state.
	ownView, err := share.ShareView(ownerCtx, scope, "v-own")
	require.NoError(t, err)
	require.False(t, ownView.Contribution.Restricted)
	digestOwn := ownView.Contribution.EvidenceDigest
	decided, err := share.DecideShare(ownerCtx, scope, "v-own", craft.DecisionRejected, digestOwn)
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateConsented, decided.State, "an unrestricted version stays consented")
	require.Nil(t, decided.Decision, "F1: a bound historical rejection is never projected beside an unrestricted consent")
	require.Nil(t, decided.ExpiresAt)
	reread, err := share.ShareView(ownerCtx, scope, "v-own")
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateConsented, reread.State)
	require.Nil(t, reread.Decision)

	// --- F2a: replaying a decision of other evidence is an audited refusal.
	restricted, err := share.ShareView(ownerCtx, scope, "v-r")
	require.NoError(t, err)
	require.True(t, restricted.Contribution.Restricted)
	digestR := restricted.Contribution.EvidenceDigest
	_, err = share.DecideShare(ownerCtx, scope, "v-r", craft.DecisionApproved, strings.Repeat("0", 64))
	require.ErrorIs(t, err, craft.ErrConflict)
	denialDetails := func() []string {
		var details []string
		require.NoError(t, db.Table("audit_logs").Where("action LIKE ? AND outcome = ?", "craft.share_denied:%", "denied").Order("id").Pluck("details", &details).Error)
		return details
	}
	require.Contains(t, strings.Join(denialDetails(), "\n"), "evidence_digest_mismatch", "F2: the replay conflict is audited with its reason")

	// --- F2b: a caller identity mismatch (decide and revoke) is audited too.
	// (Both refusals share one reason: distinct targets keep their rows
	// independent — a same-reason denial of the SAME tuple dedups by design,
	// and the R3 section below pins that different reasons never swallow
	// each other even on one tuple.)
	_, err = share.DecideShare(mismatchCtx, scope, "v-own", craft.DecisionApproved, digestR)
	require.ErrorIs(t, err, craft.ErrForbidden)
	_, err = share.RevokeShare(mismatchCtx, scope, "v-x")
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Equal(t, 2, strings.Count(strings.Join(denialDetails(), "\n"), "caller_identity_mismatch"), "F2: both identity-mismatch refusals are audited")

	// --- F6: the decide hot path derives the contribution exactly once and
	// composes its response without re-reading the version store.
	before := gets
	approved, err := share.DecideShare(ownerCtx, scope, "v-r", craft.DecisionApproved, digestR)
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateConsented, approved.State)
	require.NotNil(t, approved.Decision)
	require.Equal(t, digestR, approved.Decision.EvidenceDigest)
	require.NotNil(t, approved.ExpiresAt)
	require.Equal(t, before+1, gets, "F6: DecideShare composes the response from the already-derived contribution")
	fresh, err := share.ShareView(ownerCtx, scope, "v-r")
	require.NoError(t, err)
	require.Equal(t, fresh.State, approved.State, "the composed response equals a fresh read")
	require.Equal(t, fresh.Decision, approved.Decision)
	require.Equal(t, fresh.ExpiresAt, approved.ExpiresAt, "R3: the TTL expiry projection is one shared projection on both paths")
	require.Equal(t, before+2, gets, "only the fresh read adds a second derivation")

	// --- the F1 guard did not over-suppress: a restricted version still
	// projects its bound rejected decision as declined.
	declined, err := share.DecideShare(ownerCtx, scope, "v-r", craft.DecisionRejected, digestR)
	require.NoError(t, err)
	require.Equal(t, craft.ShareStateDeclined, declined.State)
	require.NotNil(t, declined.Decision)
	require.Equal(t, craft.DecisionRejected, declined.Decision.Decision)

	// --- F4: a probing client replaying the same denial writes exactly one
	// audit row inside the window, and a different actor is not swallowed.
	var before4 int64
	require.NoError(t, db.Table("audit_logs").Where("action LIKE ? AND outcome = ?", "craft.share_denied:%", "denied").Count(&before4).Error)
	for i := 0; i < 3; i++ {
		_, err = share.DecideShare(craftKnowledgeCtx(viewer), viewer, "v-r", craft.DecisionApproved, digestR)
		require.ErrorIs(t, err, craft.ErrForbidden)
	}
	var afterViewer int64
	require.NoError(t, db.Table("audit_logs").Where("action LIKE ? AND outcome = ?", "craft.share_denied:%", "denied").Count(&afterViewer).Error)
	require.Equal(t, before4+1, afterViewer, "F4: the same denial replayed three times writes exactly one row")
	_, err = share.DecideShare(craftKnowledgeCtx(viewer2), viewer2, "v-r", craft.DecisionApproved, digestR)
	require.ErrorIs(t, err, craft.ErrForbidden)
	var afterViewer2 int64
	require.NoError(t, db.Table("audit_logs").Where("action LIKE ? AND outcome = ?", "craft.share_denied:%", "denied").Count(&afterViewer2).Error)
	require.Equal(t, afterViewer+1, afterViewer2, "a different actor's refusal is still recorded")

	// --- R3: different denial reasons never swallow each other inside the
	// window on the SAME actor and version: the reason rides the typed
	// action column, so the digest replay signal and a caller identity
	// mismatch each land their own row (before the fix, whichever reason
	// wrote first suppressed the other for 60s).
	versions.byID["v-y"] = version("v-y", "run-r", shared.ID)
	_, err = share.DecideShare(ownerCtx, scope, "v-y", craft.DecisionApproved, strings.Repeat("0", 64))
	require.ErrorIs(t, err, craft.ErrConflict)
	_, err = share.DecideShare(mismatchCtx, scope, "v-y", craft.DecisionApproved, digestR)
	require.ErrorIs(t, err, craft.ErrForbidden)
	denialsOf := func(action string) int64 {
		var n int64
		require.NoError(t, db.Table("audit_logs").Where("action = ? AND outcome = ? AND target_id = ?", action, "denied", "v-y").Count(&n).Error)
		return n
	}
	require.Equal(t, int64(1), denialsOf("craft.share_denied:evidence_digest"), "R3: the replay signal lands even when another reason denied the same tuple first")
	require.Equal(t, int64(1), denialsOf("craft.share_denied:caller_identity"), "R3: the identity refusal lands beside the replay signal on the same tuple")
	// Replaying the SAME reason on the same tuple is still deduplicated.
	_, err = share.DecideShare(ownerCtx, scope, "v-y", craft.DecisionApproved, strings.Repeat("0", 64))
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Equal(t, int64(1), denialsOf("craft.share_denied:evidence_digest"), "replaying the same denial of the same reason still writes exactly one row")
}
