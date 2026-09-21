package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// C06 controlled supplemental retrieval. The CraftKnowledgeTool wraps the
// SAME two real ACL entrances Build uses (shared-aware knowledge resolution
// + per-library authorizeKBAccess) behind a short-term execution credential
// that is bound to tenant/user/run/task and the task's STORED knowledge
// authorization snapshot. Every Query re-authorizes; nothing a model
// produces can widen the scope.

// -----------------------------------------------------------------------------
// fixtures
// -----------------------------------------------------------------------------

// knowledgeTaskStore fakes the R02 store for one delegated task: scope
// guarded, with an optional stored terminal result.
type knowledgeTaskStore struct {
	craft.Store
	ws     craft.Workspace
	task   craft.Task
	result *craft.Result
}

func (s *knowledgeTaskStore) GetWorkspace(context.Context, craft.Scope) (craft.Workspace, error) {
	return s.ws, nil
}

func (s *knowledgeTaskStore) GetTask(_ context.Context, scope craft.Scope, id string) (craft.Task, error) {
	if id != s.task.ID {
		return craft.Task{}, craft.ErrNotFound
	}
	if !craft.SameScope(scope, s.task.Scope) {
		return craft.Task{}, craft.ErrForbidden
	}
	return s.task, nil
}

func (s *knowledgeTaskStore) GetResult(_ context.Context, scope craft.Scope, id string) (craft.Result, error) {
	if s.result == nil || id != s.task.ID || !craft.SameScope(scope, s.task.Scope) {
		return craft.Result{}, craft.ErrNotFound
	}
	return *s.result, nil
}

type fakeKnowledgeClock struct{ now time.Time }

func (c *fakeKnowledgeClock) Now() time.Time { return c.now }

type knowledgeToolFixture struct {
	*knowledgeFixture
	store *knowledgeTaskStore
	clock *fakeKnowledgeClock
	tool  *CraftKnowledgeTool
}

// newKnowledgeToolFixture assembles the tool over the REAL sqlite ACL
// entrances (same seam as Build) plus a scope-guarded task store.
func newKnowledgeToolFixture(t *testing.T, allowedKBs map[string]bool, enabled bool) *knowledgeToolFixture {
	t.Helper()
	f := newKnowledgeFixture(t, allowedKBs)
	f.seedKB(t, "kb-own", 1)
	f.seedKB(t, "kb-shared", 2)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Doc")
	f.seedKnowledge(t, "k-shared", "kb-shared", 2, "Shared Doc")
	f.seedChunk("kb-own", "k-own", "c-own-1", "own excerpt one")
	f.seedChunk("kb-own", "k-own", "c-own-2", "own excerpt two")
	f.seedChunk("kb-shared", "k-shared", "c-shared-1", "shared excerpt")

	scope := craftKnowledgeScope()
	deadline := time.Now().UTC().Add(time.Hour)
	store := &knowledgeTaskStore{
		ws: craft.Workspace{ID: "ws-knowledge", Scope: scope},
		task: craft.Task{
			ID: "dlg-tool", Scope: scope, WorkspaceID: "ws-knowledge",
			Fence:    agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: 1, RunID: "run-1"}, Owner: "worker-1", Epoch: 1},
			Deadline: deadline,
			// The task's STORED knowledge authorization snapshot: the
			// craftkb:// material inputs staged when the task was prepared.
			Inputs: []craft.Input{
				{Ref: craft.KnowledgeRef("kb-own", "k-own", "c-own-1"), Name: "c-own-1.txt", SHA256: strings.Repeat("a", 64), Bytes: 16},
				{Ref: craft.KnowledgeRef("kb-shared", "k-shared", "c-shared-1"), Name: "c-shared-1.txt", SHA256: strings.Repeat("b", 64), Bytes: 16},
			},
		},
	}
	accessSvc := &knowledgeService{
		repo:           repository.NewKnowledgeRepository(f.db),
		kbShareService: f.shares,
	}
	kbSvc := &knowledgeBaseService{
		repo:           repository.NewKnowledgeBaseRepository(f.db),
		kgRepo:         repository.NewKnowledgeRepository(f.db),
		kbShareService: f.shares,
	}
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		kbs, err := kbSvc.repo.GetKnowledgeBaseByIDs(ctx, []string{kbID})
		if err != nil {
			return nil, err
		}
		if err := kbSvc.authorizeKBAccess(ctx, kbs); err != nil {
			return nil, err
		}
		return f.results[kbID], nil
	}
	clock := &fakeKnowledgeClock{now: time.Now().UTC()}
	tool, err := NewCraftKnowledgeTool(CraftKnowledgeToolConfig{
		Store: store, Access: BindCraftKnowledgeAccess(accessSvc), Search: search,
		Enabled: enabled, Now: clock.Now,
	})
	require.NoError(t, err)
	return &knowledgeToolFixture{knowledgeFixture: f, store: store, clock: clock, tool: tool}
}

// fenced returns the task's own run fence on the context.
func (f *knowledgeToolFixture) fenced() context.Context {
	return agentruntime.WithRunFence(craftKnowledgeCtx(f.store.task.Scope), f.store.task.Fence)
}

func (f *knowledgeToolFixture) otherRun() context.Context {
	other := f.store.task.Fence
	other.RunID = "run-other"
	return agentruntime.WithRunFence(craftKnowledgeCtx(f.store.task.Scope), other)
}

// mint mints the execution credential for the fixture task.
func (f *knowledgeToolFixture) mint(t *testing.T) *CraftKnowledgeGrant {
	t.Helper()
	grant, err := f.tool.Mint(f.fenced(), f.store.task.Scope, f.store.task.ID, f.store.task.Deadline.Add(time.Minute))
	require.NoError(t, err)
	return grant
}

// -----------------------------------------------------------------------------
// scenarios
// -----------------------------------------------------------------------------

// The default flag craft.execution_knowledge=false: the tool refuses both
// minting and querying until the deployment switch is explicitly enabled.
func TestExecutionKnowledgeDisabledByDefault(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, false)
	_, err := f.tool.Mint(f.fenced(), f.store.task.Scope, f.store.task.ID, time.Now().Add(time.Hour))
	require.ErrorIs(t, err, craft.ErrUnsupported)
	_, err = f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrUnsupported)
}

// The query scope is the task's STORED snapshot intersected with the CURRENT
// ACL: a share revoked after the task was prepared narrows the answer to the
// still-authorized documents instead of failing or widening.
func TestQueryNarrowsToSnapshotIntersectedWithCurrentACL(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	f.mint(t)

	bundle, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 3)

	// Revoke the shared library: the current ACL drops it, the answer keeps
	// only own-library material.
	f.shares.allowedKBs = map[string]bool{}
	bundle, err = f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 2)
	for _, s := range bundle.Sources {
		require.NotContains(t, s.Ref, "kb-shared", "revoked library leaked into the answer")
	}
}

// A knowledge the model discovers later (authorized now, in the same
// library) never widens the stored snapshot: only the two staged documents
// are ever returned.
func TestQueryNeverWidensBeyondStoredSnapshot(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	f.mint(t)
	// A brand-new authorized document with a matching chunk — invisible to
	// the tool because the task snapshot never recorded it.
	f.seedKnowledge(t, "k-new", "kb-own", 1, "New Doc")
	f.seedChunk("kb-own", "k-new", "c-new-1", "new excerpt")

	bundle, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.NoError(t, err)
	for _, s := range bundle.Sources {
		require.NotContains(t, s.Ref, "k-new", "model-invented knowledge id widened the scope")
	}
}

// A credential minted for one run is refused inside another run: the token
// never crosses runs, and a query without the run fence is refused outright.
func TestQueryRejectsCrossRunCredential(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	f.mint(t)
	_, err := f.tool.Query(f.otherRun(), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
	_, err = f.tool.Query(craftKnowledgeCtx(f.store.task.Scope), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// At most eight queries per credential; the ninth is refused.
func TestQueryCallBudgetIsEight(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	grant := f.mint(t)
	require.Equal(t, MaxExecutionKnowledgeCalls, grant.CallsLeft)
	for i := 0; i < MaxExecutionKnowledgeCalls; i++ {
		_, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
		require.NoError(t, err, "call %d", i+1)
	}
	_, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "budget")
}

// Expiry never outlives the run deadline: a request beyond the task deadline
// is clamped to it, and once it passes the credential is dead.
func TestQueryExpiryNeverExceedsRunDeadline(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	grant := f.mint(t) // mint requested a deadline PAST the task deadline
	require.True(t, !grant.Deadline.After(f.store.task.Deadline), "grant deadline must not exceed the run deadline")

	_, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.NoError(t, err)

	f.clock.now = f.store.task.Deadline.Add(time.Second)
	_, err = f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// A terminal task is dead immediately: once a result is stored the
// credential answers nothing, even with budget and time left. An explicit
// Revoke cancels just as fast.
func TestTerminalAndRevokedTasksAnswerNothing(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	f.mint(t)

	// Terminal result stored server-side.
	f.store.result = &craft.Result{TaskID: f.store.task.ID, Status: "succeeded"}
	_, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "terminal")

	// Fresh grant, explicit cancel.
	f.store.result = nil
	f.mint(t)
	f.tool.Revoke(f.store.task.Scope, f.store.task.ID)
	_, err = f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// Without a minted credential there is nothing to query with.
func TestQueryRequiresMintedCredential(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	_, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// The answer is bounded C01 material structure only: excerpts, citation IDs,
// durable refs and digests — never database or vector-store credentials —
// with the bundle capped at 64 KiB per query.
func TestQueryReturnsBoundedMaterialStructureOnly(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{}, true)
	// Ten over-size chunks: each excerpt is capped at 8 KiB and the bundle
	// at 64 KiB, so the answer must come back bounded AND truncated.
	for i := 0; i < 10; i++ {
		f.seedChunk("kb-own", "k-own", fmt.Sprintf("c-big-%02d", i), strings.Repeat("x", 20<<10))
	}
	f.mint(t)
	bundle, err := f.tool.Query(f.fenced(), f.store.task.Scope, f.store.task.ID, "totals")
	require.NoError(t, err)
	total := 0
	capped := 0
	for _, s := range bundle.Sources {
		// ExcerptOf bounds the content portion to the cap and appends one
		// UTF-8 replacement character (3 bytes) when it truncated.
		require.LessOrEqual(t, len(s.Excerpt), craft.MaxKnowledgeExcerptBytes+3, "per-source excerpt cap")
		if len(s.Excerpt) >= craft.MaxKnowledgeExcerptBytes {
			capped++
		}
		require.NotEmpty(t, s.ID)
		require.True(t, strings.HasPrefix(s.Ref, "craftkb://kb/"))
		require.Len(t, s.Digest, 64)
		total += len(s.Excerpt)
	}
	require.Positive(t, capped, "at least one excerpt must hit the per-source cap")
	require.LessOrEqual(t, total, craft.MaxKnowledgeBundleBytes)
	require.True(t, bundle.Truncated, "over-cap retrieval must be marked truncated")
}

// A task whose inputs carry no knowledge material has no knowledge snapshot:
// no credential may be minted for it (empty scope is not zero scope).
func TestMintRequiresStoredKnowledgeSnapshot(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	f.store.task.Inputs = []craft.Input{{Ref: "resource://upload/1", Name: "a.csv", SHA256: strings.Repeat("c", 64), Bytes: 3}}
	_, err := f.tool.Mint(f.fenced(), f.store.task.Scope, f.store.task.ID, time.Now().Add(time.Minute))
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

// A query against a task outside the caller's scope is invisible.
func TestQueryScopeGuarded(t *testing.T) {
	f := newKnowledgeToolFixture(t, map[string]bool{"kb-shared": true}, true)
	f.mint(t)
	foreign := craftKnowledgeScope()
	foreign.UserID = "someone-else"
	_, err := f.tool.Query(agentruntime.WithRunFence(craftKnowledgeCtx(foreign), f.store.task.Fence), foreign, f.store.task.ID, "totals")
	require.ErrorIs(t, err, craft.ErrForbidden)
}

var _ = errors.New
