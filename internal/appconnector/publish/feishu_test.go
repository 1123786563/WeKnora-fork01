package publish

// FeishuBridge tests: snapshot-shape routing, the non-feishu connection
// refusal, the AC2 write-capability gate (schema_json scopes), the plan
// pre-read, and the pinned production policy.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

func feishuSnapshot(id string) appconnectorsvc.ActionSnapshot {
	args, _ := json.Marshal(map[string]any{"parent_folder": "fld-1", "title": "T", "blocks": []any{}})
	return appconnectorsvc.ActionSnapshot{ID: id, TenantID: 7, ActorID: "u1", ConnectionID: "conn-f",
		Version: "feishu/v1", Target: "fld-1", Risk: "write", AuthVersion: 1, Args: args}
}

func newFeishuBridge(scope NotionConnectionScope, pubs *repoappconn.PublicationStore) *FeishuBridge {
	return NewFeishuBridge(&fakeScopes{scope: scope}, &fakePolicies{pol: appconn.HTTPPolicy{}}, &fakeTokens{tok: "secret_test_token"}, pubs)
}

func TestFeishuBridgeRefusesNonFeishuConnection(t *testing.T) {
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "notion", AuthVersion: 1,
		Scopes: []string{appconn.FeishuCapabilityWriteDocx}}, repoappconn.NewPublicationStore(db))
	_, err := bridge.Dispatch(context.Background(), feishuSnapshot("a1"), "k")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted, "a provable pre-send wiring gap")
}

func TestFeishuBridgeRefusesReadOnlyScope(t *testing.T) {
	// AC2: a connection whose reviewed scopes carry only read/sync
	// capabilities never satisfies the write gate.
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "feishu", AuthVersion: 1,
		Scopes: []string{"read_docx", "sync_content"}}, repoappconn.NewPublicationStore(db))
	out, err := bridge.Dispatch(context.Background(), feishuSnapshot("a2"), "k")
	require.NoError(t, err, "capability refusal is a definitive failed OUTCOME, not a wiring gap")
	require.Equal(t, appconn.ActionFailed, out.Status)
	require.Contains(t, out.ProviderResult, appconn.FeishuCapabilityWriteDocx)
}

func TestFeishuBridgeRoutesUpdateSnapshotToAdapter(t *testing.T) {
	// Routing itself is adapter-domain (IsFeishuDocUpdateArgs); the bridge
	// only proves both shapes pass the scope/caps gate without erroring
	// at the wiring layer. With an empty test policy the adapter fails
	// closed BEFORE dialing — a deterministic, networkless outcome.
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "feishu", AuthVersion: 1,
		Scopes: []string{appconn.FeishuCapabilityWriteDocx}}, repoappconn.NewPublicationStore(db))
	updateArgs, _ := json.Marshal(map[string]any{"document_id": "doc-1", "expected_revision": "1", "title": "T", "blocks": []any{}})
	snap := feishuSnapshot("a3")
	snap.Args = updateArgs
	out, err := bridge.Dispatch(context.Background(), snap, "k")
	require.NoError(t, err)
	require.Equal(t, appconn.ActionFailed, out.Status, "empty test policy → configError, identical to the notion bridge's loopback-hook pattern")
}

func TestFeishuBridgeReadPageVersionDelegatesToAdapter(t *testing.T) {
	// The pre-read rides ReadFeishuDocumentVersion; with no reviewed
	// policy (empty) it fails closed — pinned here so the bridge cannot
	// silently dial with an unpinned contract.
	db := openPlanDB(t)
	bridge := newFeishuBridge(NotionConnectionScope{AppID: "feishu", AuthVersion: 1}, repoappconn.NewPublicationStore(db))
	_, err := bridge.ReadPageVersion(context.Background(), "conn-f", "doc-1")
	require.Error(t, err)
}

func TestConstantFeishuPolicyProviderPinsContract(t *testing.T) {
	pol, err := NewConstantFeishuPolicyProvider().PolicyFor(context.Background(), "conn-f")
	require.NoError(t, err)
	require.Equal(t, "https", pol.Scheme)
	require.Equal(t, appconn.FeishuAPIHost, pol.Host, "open.feishu.cn — pinned, never model-derived")
	require.Equal(t, []string{http.MethodGet, http.MethodPost}, pol.Methods)
	require.Equal(t, "/open-apis/docx/", pol.PathPrefix)
	require.Equal(t, 30*time.Second, pol.Timeout)
	require.Empty(t, pol.AuthorizedNetworks, "production policy never authorizes private ranges")
}

func TestFeishuProfileSnapshotShapes(t *testing.T) {
	profile := FeishuProfile(&fakeRemote{versions: map[string]string{}})
	blocks, err := appconn.FeishuTextBlocks("a。\n\nb。")
	require.NoError(t, err)

	createArgs, err := profile.CreateArgs("fld-1", "T", blocks)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(createArgs, &raw))
	require.Len(t, raw, 3)
	require.Contains(t, raw, "parent_folder")

	updateArgs, err := profile.UpdateArgs("doc-1", "4", "T", blocks)
	require.NoError(t, err)
	var updateRaw map[string]json.RawMessage // a FRESH map: Unmarshal merges into a non-nil map, so reusing `raw` would keep the create keys
	require.NoError(t, json.Unmarshal(updateArgs, &updateRaw))
	require.Len(t, updateRaw, 4)
	require.Contains(t, updateRaw, "document_id")
	require.Contains(t, updateRaw, "expected_revision")

	require.Equal(t, "feishu", profile.AppID)
	require.Equal(t, "feishu", profile.Provider)
	require.Equal(t, "feishu/v1", profile.ActionVersion)
	require.Equal(t, FeishuVersionConflictResult, profile.ConflictResultPrefix)

	// ParseReceipt projects the adapter-domain receipt onto the shared view.
	id, ver, err := profile.ParseReceipt(`{"document":{"document_id":"doc-9","revision_id":12}}`)
	require.NoError(t, err)
	require.Equal(t, "doc-9", id)
	require.Equal(t, "12", ver)
}

func TestNotionProfileUnchanged(t *testing.T) {
	// #48's profile is a pure extraction: same fields, same snapshot bytes.
	profile := NotionProfile(&fakeRemote{versions: map[string]string{}})
	require.Equal(t, "notion", profile.AppID)
	require.Equal(t, "notion", profile.Provider)
	require.Equal(t, "notion/v1", profile.ActionVersion)
	require.Equal(t, PublishVersionConflictResult, profile.ConflictResultPrefix)
	id, ver, err := profile.ParseReceipt(`{"object":"page","id":"p1","last_edited_time":"t1"}`)
	require.NoError(t, err)
	require.Equal(t, "p1", id)
	require.Equal(t, "t1", ver)
}
