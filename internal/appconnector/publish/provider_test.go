package publish

// AC1 evidence: the publish service layer is provider-neutral. The SAME
// table of assertions drives BOTH the notion and the feishu profile
// through plan formation, approval, execution and receipt settlement;
// the only per-provider input is the ProviderProfile (whose blocks/snapshot
// functions live in the adapter domain). If a future edit re-introduces
// a provider branch into plan.go, this table stops being symmetric and
// the test fails.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

// newPublishActionsForTest builds the dedicated publish ActionService the
// production composition uses: the bridge is BOTH dispatcher and unknown
// resolver (plan_test.go newPlanEnv wires the same shape for notion).
func newPublishActionsForTest(t *testing.T, store *repoappconn.ActionStore, bridge interface {
	appconnectorsvc.ActionDispatcher
	appconnectorsvc.UnknownResolver
}) *appconnectorsvc.ActionService {
	t.Helper()
	return appconnectorsvc.NewActionService(store, passGuard{}, nil, bridge, bridge)
}

type feishuPlanEnv struct {
	svc   *NotionPublishService
	pubs  *repoappconn.PublicationStore
	store *repoappconn.ActionStore
}

func newFeishuPlanEnv(t *testing.T, remote *fakeRemote, scope NotionConnectionScope) *feishuPlanEnv {
	t.Helper()
	db := openPlanDB(t)
	store := repoappconn.NewActionStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	bridge := NewFeishuBridge(
		&fakeScopes{scope: scope},
		&fakePolicies{pol: appconn.HTTPPolicy{}}, // empty policy: the adapter fails closed BEFORE dialing
		&fakeTokens{tok: "secret_test_token"},
		pubs,
	)
	actions := newPublishActionsForTest(t, store, bridge)
	artifacts := &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
		Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ObjectKey: "artifact-versions/7/run-1/d", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 12,
	}}
	svc := NewProviderPublishService(actions, store, pubs, artifacts, &fakeContent{data: []byte("飞书第一段。\n\n飞书第二段。")},
		&fakeScopes{scope: scope}, FeishuProfile(remote))
	return &feishuPlanEnv{svc: svc, pubs: pubs, store: store}
}

// TestPublishServiceProfilesAreSymmetric drives BOTH profiles through
// the identical service-layer assertion set.
func TestPublishServiceProfilesAreSymmetric(t *testing.T) {
	type profileCase struct {
		name             string
		appID            string
		createDest       string // PublishPlanInput.ParentPageID
		expectedMode     string
		expectedDest     string
		expectedBaseline string // notion: the parent page version; feishu: empty (folder has no revision)
		expectSnapshot   func(t *testing.T, argsJSON string)
	}
	notionCase := profileCase{
		name: "notion", appID: "notion", createDest: "parent-1",
		expectedMode: "create", expectedDest: "parent-1", expectedBaseline: "v-1",
		expectSnapshot: func(t *testing.T, argsJSON string) {
			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(argsJSON), &raw))
			require.Len(t, raw, 3, "notion create snapshot: parent, title, blocks")
			require.Contains(t, raw, "parent")
			var parent string
			require.NoError(t, json.Unmarshal(raw["parent"], &parent))
			require.Equal(t, "parent-1", parent)
		},
	}
	feishuCase := profileCase{
		name: "feishu", appID: "feishu", createDest: "fld-1",
		expectedMode: "create", expectedDest: "fld-1", expectedBaseline: "",
		expectSnapshot: func(t *testing.T, argsJSON string) {
			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(argsJSON), &raw))
			require.Len(t, raw, 3, "feishu create snapshot: parent_folder, title, blocks")
			require.Contains(t, raw, "parent_folder")
			var folder string
			require.NoError(t, json.Unmarshal(raw["parent_folder"], &folder))
			require.Equal(t, "fld-1", folder)
		},
	}
	for _, tc := range []profileCase{notionCase, feishuCase} {
		t.Run(tc.name, func(t *testing.T) {
			scope := NotionConnectionScope{
				AppID: tc.appID, ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
				ApprovedParents: []string{tc.createDest},
				Scopes:          []string{appconn.FeishuCapabilityWriteDocx},
			}
			if tc.appID == "notion" {
				scope.InsertCapability = true
			}
			// The feishu case leaves the remote map EMPTY: the folder
			// destination has no revision, so the pre-read yields an
			// empty baseline (the shared layer accepts it for create).
			remoteByApp := map[string]map[string]string{
				"notion": {tc.createDest: "v-1"},
				"feishu": {},
			}
			remote := &fakeRemote{versions: remoteByApp[tc.appID]}
			var env *feishuPlanEnv
			if tc.appID == "notion" {
				// The notion profile rides the SAME symmetric assertions
				// via the shared constructor path.
				db := openPlanDB(t)
				store := repoappconn.NewActionStore(db)
				pubs := repoappconn.NewPublicationStore(db)
				actions := newPublishActionsForTest(t, store, NewNotionBridge(&fakeScopes{scope: scope}, &fakePolicies{pol: appconn.HTTPPolicy{}}, &fakeTokens{tok: "secret_test_token"}, pubs))
				svc := NewNotionPublishService(actions, store, pubs, &fakeArtifacts{version: repository.ArtifactVersion{
					TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
					Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
					ObjectKey: "k", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 12,
				}}, &fakeContent{data: []byte("一。\n\n二。")}, remote, &fakeScopes{scope: scope})
				env = &feishuPlanEnv{svc: svc, pubs: pubs, store: store}
			} else {
				env = newFeishuPlanEnv(t, remote, scope)
			}

			input := PublishPlanInput{TenantID: 7, ActorID: "u1", ConnectionID: "conn-x",
				SessionID: "sess-1", ArtifactVersionID: "ver-1", Title: "T", ParentPageID: tc.createDest}
			view, err := env.svc.FormPlan(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedMode, view.Mode)
			require.Equal(t, tc.expectedDest, view.Destination)
			if tc.expectedBaseline == "" {
				require.Empty(t, view.ExpectedExternalVersion,
					"AC1: a feishu folder destination records an EMPTY baseline (typed not-found → create proceeds)")
			} else {
				require.Equal(t, tc.expectedBaseline, view.ExpectedExternalVersion, "AC1 baseline pre-read rides the shared path")
			}

			// The recorded publication row is provider-branded by the profile.
			row, err := env.pubs.FindByAction(context.Background(), 7, view.ActionID)
			require.NoError(t, err)
			require.Equal(t, tc.appID, row.Provider)

			// The prepared action's snapshot shape comes from the profile.
			snap, err := env.store.FindAction(context.Background(), view.ActionID)
			require.NoError(t, err)
			tc.expectSnapshot(t, snap.ArgsSnapshot)

			// Execute requires approval first; after approval the shared
			// execute path settles the receipt from the action row. The
			// empty test policy makes the adapter refuse to dial — the
			// SAME terminal transition for BOTH providers (the symmetric
			// settle assertion). A definitive failed dispatch is an
			// OUTCOME, not a Go error (action.go settleOutcome), so
			// Execute itself reports no error — the assertion strength
			// lives in the action state + settled receipt below.
			require.NoError(t, env.svc.actions.Approve(context.Background(), view.ActionID, "u1", view.Digest))
			outcome, execErr := env.svc.Execute(context.Background(), 7, view.ActionID)
			require.NoError(t, execErr) // empty test policy: adapter refuses to dial — a failed OUTCOME for BOTH providers
			require.Equal(t, appconn.ActionFailed, outcome.ActionState)
			require.Equal(t, repoappconn.PublicationFailed, outcome.Receipt.State)
		})
	}
}

// TestFeishuProfileBlocksOfMapsAdapterSentinels pins B5-F78/F57: the feishu
// profile must surface the NEUTRAL publish sentinels — the family the
// handler's failPublish matches — not the adapter-package instances whose
// errors.Is chains never reach the handler. Empty content → the 400 branch;
// an oversized block set → the 413 branch.
func TestFeishuProfileBlocksOfMapsAdapterSentinels(t *testing.T) {
	profile := FeishuProfile(&fakeRemote{})

	_, err := profile.BlocksOf("   \n\n  \n\t\n")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPublishEmptyContent, "空内容必须落到 publish 包哨兵（handler 400 分支可达）")

	paragraphs := make([]string, MaxPublishBlocks+1)
	for i := range paragraphs {
		paragraphs[i] = "段"
	}
	_, err = profile.BlocksOf(strings.Join(paragraphs, "\n\n"))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPublishContentTooLarge, "超限块数必须落到 publish 包哨兵（handler 413 分支可达）")

	blocks, err := profile.BlocksOf("第一段。\n\n第二段。")
	require.NoError(t, err)
	require.Len(t, blocks, 2, "正常内容不受影响")
	require.False(t, errors.Is(err, appconn.ErrFeishuPublishEmptyContent))
}

// TestConfluencePublishServiceIsProviderProfileThin pins R5-F12: the
// confluence publish service must be the SHARED provider-neutral body over
// the Confluence profile — the #49/#50 ProviderProfile collapse feishu
// already took. A parallel FormPlan/Execute/Reconcile/Receipt copy in
// confluence.go is double maintenance (this batch's reconciliation-semantics
// fix already had to land in two projections); the source assertion keeps
// the copy from creeping back.
func TestConfluencePublishServiceIsProviderProfileThin(t *testing.T) {
	raw, err := os.ReadFile("confluence.go")
	require.NoError(t, err)
	src := string(raw)
	for _, method := range []string{"FormPlan", "Execute", "Reconcile", "Receipt", "project"} {
		require.NotContainsf(t, src, "func (s *ConfluencePublishService) "+method,
			"the %s body must live only in the shared plan.go service", method)
	}
	require.Contains(t, src, "NewProviderPublishService(",
		"the confluence constructor must build the shared service over the profile")
}
