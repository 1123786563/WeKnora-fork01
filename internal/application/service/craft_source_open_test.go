package service

// T10 (#127): every source open is reauthorized for the CURRENT viewer.
//
// The journey pins the whole per-open contract at the highest service seam:
//
//   - a Task Viewer sees the citation row (placeholder + integrity) but an
//     open without the viewer's OWN knowledge grant is denied — Task/artifact
//     access never implies original-source access;
//   - granting the viewer knowledge access makes the NEXT open succeed, and
//     the result is only the recorded durable ref, never a provider/storage
//     URL and never the excerpt itself;
//   - revoking that grant (or the Task grant) fails the very next open —
//     nothing about a previous open decision is cached;
//   - a missing citation and a tampered (URL-shaped) recorded ref both deny
//     stably and non-leaking (ErrNotFound), whatever the viewer can access.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// t10RoleChecker is the T08 task-ACL stand-in: it answers per user with a
// craft.TaskRole and defers to the REAL role/action policy, so the journey
// exercises production TaskOpenSource semantics instead of a boolean.
type t10RoleChecker struct{ roles map[string]craft.TaskRole }

func (c *t10RoleChecker) CheckTaskAccess(_ context.Context, scope craft.Scope, action craft.TaskAction) error {
	role, ok := c.roles[scope.UserID]
	if !ok {
		return craft.ErrForbidden
	}
	if !role.AllowsTaskAction(action) {
		return craft.ErrForbidden
	}
	return nil
}

func TestCraftT10Journey(t *testing.T) {
	// --- the world: one craft session, an owner, a Task Viewer, and one
	// selected knowledge document the Run actually used.
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "Region Sales")
	f.seedChunk("kb-a", "k-a", "c-a", "selected source excerpt")
	require.NoError(t, f.db.AutoMigrate(&repository.CraftKnowledgeRecordRow{}))
	records := repository.NewCraftKnowledgeRecordRepository(f.db)

	// The per-viewer knowledge permission gate. This closure is the existing
	// shared-aware document ACL entrance the service is bound to in
	// production; the map models which viewer CURRENTLY holds a grant, and
	// every mutation below models a grant or revocation landing between opens.
	resourceAllowed := map[string]bool{"u-owner": true, "u-viewer": false}
	accessCalls := 0
	access := func(ctx context.Context, _ uint64, ids []string) ([]*types.Knowledge, error) {
		accessCalls++
		caller := types.CallerFromContext(ctx)
		if !resourceAllowed[caller.UserID] {
			return nil, nil
		}
		var rows []*types.Knowledge
		for _, id := range ids {
			if id == "k-a" {
				rows = append(rows, &types.Knowledge{ID: "k-a", KnowledgeBaseID: "kb-a", TenantID: 1, Title: "Region Sales"})
			}
		}
		return rows, nil
	}

	checker := &t10RoleChecker{roles: map[string]craft.TaskRole{
		"u-owner": craft.TaskRoleOwner, "u-viewer": craft.TaskRoleViewer,
	}}
	require.True(t, craft.TaskRoleViewer.AllowsTaskAction(craft.TaskOpenSource), "a Task Viewer holds the open_source task action")
	owner := craft.Scope{TenantID: 1, UserID: "u-owner", SessionID: "s-craft"}
	viewer := craft.Scope{TenantID: 1, UserID: "u-viewer", SessionID: "s-craft"}
	ownerCtx := craftKnowledgeCtx(owner)
	viewerCtx := craftKnowledgeCtx(viewer)
	base := f.service(t, nil)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store:  &knowledgeWorkspaceStore{ws: craft.Workspace{ID: "ws-t10", Scope: owner}},
		Access: access, Search: base.search, Writer: f.writer.write,
		TaskAccess: checker, Records: records, Publisher: newT05Publisher(f.writer),
		Now: func() time.Time { return time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)

	bundle, err := svc.BuildForRun(ownerCtx, owner, "run-t10", "region sales", []string{"k-a"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)
	citationID := bundle.Sources[0].ID
	recordedRef := bundle.Sources[0].Ref
	require.NotEmpty(t, citationID)

	// --- 1. the Viewer sees the citation and its integrity, but the open is
	// denied: artifact/task access never implies original-source access.
	visible, err := svc.Sources(viewerCtx, viewer, "run-t10")
	require.NoError(t, err, "a Task Viewer may read the historical source facts")
	require.Len(t, visible.Sources, 1)
	require.Equal(t, citationID, visible.Sources[0].ID)
	require.Equal(t, recordedRef, visible.Sources[0].Ref)
	require.NotEmpty(t, visible.Sources[0].Digest, "the citation placeholder keeps its integrity digest while the open is denied")

	callsBeforeOpen := accessCalls
	_, err = svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-t10", citationID)
	require.ErrorIs(t, err, craft.ErrForbidden, "a Viewer without the source's own grant cannot open the original")
	require.Equal(t, callsBeforeOpen+1, accessCalls, "the denied open still performed a fresh authorization lookup")
	// The denial is stable and repeats identically.
	_, err = svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-t10", citationID)
	require.ErrorIs(t, err, craft.ErrForbidden)

	// --- 2. granting the Viewer their own knowledge permission makes the
	// NEXT open succeed; the result is exactly the recorded durable ref.
	resourceAllowed["u-viewer"] = true
	openedRef, err := svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-t10", citationID)
	require.NoError(t, err, "a Viewer whose own current permission covers the source may open it")
	require.Equal(t, recordedRef, openedRef)
	require.NotContains(t, openedRef, "http", "no provider/storage URL is ever handed out")
	require.True(t, len(openedRef) > 0 && openedRef[0] != '/', "the open result is a durable ref, not a path")

	// --- 3. revoking the knowledge grant fails the very next open.
	resourceAllowed["u-viewer"] = false
	_, err = svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-t10", citationID)
	require.ErrorIs(t, err, craft.ErrForbidden, "revocation applies to the next open — no cached authority")

	// --- 4. revoking the Task grant fails the next open before any resource
	// lookup, while the citation row itself stays server-side intact.
	delete(checker.roles, "u-viewer")
	_, err = svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-t10", citationID)
	require.ErrorIs(t, err, craft.ErrForbidden, "a revoked Task member cannot open sources")
	_, err = svc.Sources(viewerCtx, viewer, "run-t10")
	require.ErrorIs(t, err, craft.ErrForbidden)
	stillVisible, err := svc.Sources(ownerCtx, owner, "run-t10")
	require.NoError(t, err)
	require.Len(t, stillVisible.Sources, 1, "the historical citation row is untouched by viewer denials")
	checker.roles["u-viewer"] = craft.TaskRoleViewer

	// --- 5. a citation the record does not carry denies as a stable
	// non-leaking NotFound, never a 5xx and never a reason.
	_, err = svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-t10", "kc_0000000000000000000000ff")
	require.ErrorIs(t, err, craft.ErrNotFound)
	_, err = svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-missing", citationID)
	require.ErrorIs(t, err, craft.ErrNotFound)

	// --- 6. a tampered record whose recorded ref is URL-shaped (as if a
	// model or an attacker had smuggled a provider URL into the evidence)
	// still cannot be opened by a fully granted viewer.
	_, err = svc.BuildForRun(ownerCtx, owner, "run-tamper", "region sales", []string{"k-a"})
	require.NoError(t, err)
	tamperedRecord, err := records.Load(ownerCtx, owner, "run-tamper")
	require.NoError(t, err)
	tamperedRecord.Sources[0].Ref = "https://storage.internal.example/bucket/k-a?signature=reusable"
	raw, err := json.Marshal(tamperedRecord)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	require.NoError(t, f.db.Model(&repository.CraftKnowledgeRecordRow{}).
		Where("tenant_id = ? AND session_id = ? AND run_id = ?", owner.TenantID, owner.SessionID, "run-tamper").
		Updates(map[string]interface{}{"record_json": string(raw), "digest": hex.EncodeToString(sum[:])}).Error)

	_, err = svc.AuthorizeSourceOpen(ownerCtx, owner, "run-tamper", tamperedRecord.Sources[0].ID)
	require.ErrorIs(t, err, craft.ErrNotFound, "a URL-shaped recorded ref is not resolvable evidence and denies stably")
	_, err = svc.AuthorizeSourceOpen(viewerCtx, viewer, "run-tamper", tamperedRecord.Sources[0].ID)
	require.ErrorIs(t, err, craft.ErrNotFound, "the tamper denial is identical for every viewer — non-leaking")

	// --- 7. the owner's own grant revocation denies too: authorization is
	// per-open and per-viewer, never per-record or per-role-alone.
	resourceAllowed["u-owner"] = false
	_, err = svc.AuthorizeSourceOpen(ownerCtx, owner, "run-t10", citationID)
	require.ErrorIs(t, err, craft.ErrForbidden, "even the Task Owner is denied once their own source grant is gone")
}
